package watcher

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nxadm/tail"
	pb "logstream/agent/pb"
	"logstream/agent/config"
	"logstream/agent/offset"
	"logstream/agent/parser"
)

// pollInterval defines how often the watcher scans the directory for new files.
// This is a fallback for when fsnotify does not fire events (e.g. on some Docker volumes).
const pollInterval = 5 * time.Second

// Watcher monitors a directory for .log files and emits parsed LogEntry values.
type Watcher struct {
	dir     string
	parser  *parser.Parser
	entryCh chan *pb.LogEntry
	offsets *offset.Store
	cfg     *config.Config

	mu      sync.Mutex
	tailing map[string]struct{} // set of files currently being tailed
}

// New creates a Watcher for the given directory.
func New(dir string, p *parser.Parser, offsets *offset.Store, cfg *config.Config) *Watcher {
	return &Watcher{
		dir:     dir,
		parser:  p,
		entryCh: make(chan *pb.LogEntry, 4096),
		tailing: make(map[string]struct{}),
		offsets: offsets,
		cfg:     cfg,
	}
}

// EntryCh returns the channel on which parsed log entries are delivered.
func (w *Watcher) EntryCh() <-chan *pb.LogEntry {
	return w.entryCh
}

// Start begins watching the directory. It blocks until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	// Tail all existing .log files — offset store determines seek position.
	w.scanDir(ctx)

	// Set up fsnotify to detect new files.
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[watcher] fsnotify unavailable: %v — falling back to poll-only mode", err)
		return w.pollLoop(ctx)
	}
	defer fsWatcher.Close()

	if err := fsWatcher.Add(w.dir); err != nil {
		log.Printf("[watcher] fsnotify watch failed on %s: %v — poll loop will cover this", w.dir, err)
	}

	// Polling ticker as fallback for filesystems where fsnotify misses CREATE events.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-fsWatcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				if strings.HasSuffix(event.Name, ".log") {
					w.startTail(ctx, event.Name)
				}
			}

		case watchErr, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] fsnotify error: %v", watchErr)

		case <-ticker.C:
			// Periodic scan catches files that fsnotify missed (Docker volumes, NFS, etc.)
			w.scanDir(ctx)
		}
	}
}

// pollLoop is used when fsnotify is completely unavailable.
func (w *Watcher) pollLoop(ctx context.Context) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.scanDir(ctx)
		}
	}
}

// scanDir globs for .log files and starts tailing any that are not already being watched.
func (w *Watcher) scanDir(ctx context.Context) {
	files, err := filepath.Glob(filepath.Join(w.dir, "*.log"))
	if err != nil {
		log.Printf("[watcher] scanDir glob error: %v", err)
		return
	}
	log.Printf("[watcher] scanDir found %d .log files", len(files))
	for _, f := range files {
		w.startTail(ctx, f)
	}
}

// resolveSeek determines where to start reading a file based on saved offset.
// Returns seekInfo, whether we're in replay mode, and the saved offset value.
func (w *Watcher) resolveSeek(ctx context.Context, filePath string) (tail.SeekInfo, bool, int64) {
	// If no offset store, start from end (safe default — no duplicates)
	if w.offsets == nil || !w.offsets.Available() {
		log.Printf("[watcher] %s: no offset store — seeking to end", filepath.Base(filePath))
		if fi, err := os.Stat(filePath); err == nil {
			return tail.SeekInfo{Whence: io.SeekStart, Offset: fi.Size()}, false, -1
		}
		return tail.SeekInfo{Whence: io.SeekEnd}, false, -1
	}

	savedOffset, meta, err := w.offsets.Load(ctx, filePath)
	if err != nil {
		log.Printf("[watcher] %s: offset load error: %v — seeking to end", filepath.Base(filePath), err)
		if fi, err := os.Stat(filePath); err == nil {
			return tail.SeekInfo{Whence: io.SeekStart, Offset: fi.Size()}, false, -1
		}
		return tail.SeekInfo{Whence: io.SeekEnd}, false, -1
	}

	// First execution — no saved offset
	if savedOffset < 0 {
		if w.cfg.ReadExisting {
			log.Printf("[watcher] %s: first execution + READ_EXISTING — reading from start", filepath.Base(filePath))
			return tail.SeekInfo{Whence: io.SeekStart, Offset: 0}, false, -1
		}
		log.Printf("[watcher] %s: first execution — seeking to end (set LOG_READ_EXISTING=true to read existing)", filepath.Base(filePath))
		if fi, err := os.Stat(filePath); err == nil {
			return tail.SeekInfo{Whence: io.SeekStart, Offset: fi.Size()}, false, -1
		}
		return tail.SeekInfo{Whence: io.SeekEnd}, false, -1
	}

	// Check for file rotation (inode changed)
	currentInode, _ := offset.FileInode(filePath)
	if meta != nil && offset.IsRotated(meta.Inode, currentInode) {
		log.Printf("[watcher] %s: file rotated (inode %d → %d) — reading from start", filepath.Base(filePath), meta.Inode, currentInode)
		return tail.SeekInfo{Whence: io.SeekStart, Offset: 0}, false, savedOffset
	}

	// Normal restart — apply rewind window
	rewindPos := offset.RewindPosition(savedOffset, w.cfg.RestartWindowMinutes)
	log.Printf("[watcher] %s: resuming from offset %d (saved=%d, rewind=%dmin)", filepath.Base(filePath), rewindPos, savedOffset, w.cfg.RestartWindowMinutes)

	return tail.SeekInfo{Whence: io.SeekStart, Offset: rewindPos}, rewindPos < savedOffset, savedOffset
}

// startTail starts a tail goroutine for a file, if not already tailing it.
func (w *Watcher) startTail(ctx context.Context, filePath string) {
	w.mu.Lock()
	if _, already := w.tailing[filePath]; already {
		w.mu.Unlock()
		return
	}
	w.tailing[filePath] = struct{}{}
	w.mu.Unlock()

	log.Printf("[watcher] starting tail: %s", filepath.Base(filePath))

	go func() {
		defer func() {
			w.mu.Lock()
			delete(w.tailing, filePath)
			w.mu.Unlock()
			log.Printf("[watcher] stopped tail: %s", filepath.Base(filePath))
		}()

		seekInfo, replaying, savedOffset := w.resolveSeek(ctx, filePath)

		cfg := tail.Config{
			Follow:    true,
			ReOpen:    true,
			Poll:      true,
			MustExist: false,
			Location:  &seekInfo,
			Logger:    tail.DiscardingLogger,
		}

		t, err := tail.TailFile(filePath, cfg)
		if err != nil {
			log.Printf("[watcher] failed to tail %s: %v", filePath, err)
			return
		}
		defer t.Stop()

		filename := filepath.Base(filePath)
		fileInfo := w.parser.ParseFilename(filename)

		linesRead := 0
		linesSent := 0
		linesSkipped := 0
		bytesRead := seekInfo.Offset

		for {
			select {
			case <-ctx.Done():
				log.Printf("[watcher] %s: read=%d sent=%d skipped=%d (ctx done)", filename, linesRead, linesSent, linesSkipped)
				return
			case line, ok := <-t.Lines:
				if !ok {
					log.Printf("[watcher] %s: read=%d sent=%d skipped=%d (channel closed)", filename, linesRead, linesSent, linesSkipped)
					return
				}
				if line.Err != nil {
					log.Printf("[watcher] tail error on %s: %v", filename, line.Err)
					continue
				}
				linesRead++
				bytesRead += int64(len(line.Text)) + 1 // +1 for newline

				if linesRead == 1 {
					log.Printf("[watcher] %s: first line received (offset=%d)", filename, bytesRead)
				}

				entry := w.parser.ParseLine(line.Text, fileInfo, filePath)
				if entry == nil {
					linesSkipped++
					if linesSkipped == 1 {
						log.Printf("[watcher] %s: first SKIPPED line: %q", filename, line.Text)
					}
					continue
				}

				// Inject offset tracking metadata
				if entry.Extra == nil {
					entry.Extra = make(map[string]string)
				}
				entry.Extra["_byte_offset"] = strconv.FormatInt(bytesRead, 10)
				entry.Extra["_file_path"] = filePath

				// Mark as replayed if in rewind window
				if replaying {
					entry.Extra["replayed"] = "true"
					if bytesRead >= savedOffset {
						replaying = false
						log.Printf("[watcher] %s: replay complete at offset %d", filename, bytesRead)
					}
				}

				linesSent++
				select {
				case w.entryCh <- entry:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}
