package offset

import (
	"context"
	"crypto/sha1"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Meta stores supplementary information about a tracked file.
type Meta struct {
	Path       string
	Inode      uint64
	LastSentAt int64 // unix millis
	Filename   string
}

// Store persists file offsets in Redis.
type Store struct {
	rdb       *redis.Client
	prefix    string
	serviceID string
	available bool
	mu        sync.RWMutex
}

// New creates an offset Store. It pings Redis once; if unreachable, available=false
// and all operations become no-ops (fail-open).
func New(redisURL string, redisDB int, prefix, serviceID string) *Store {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[offset] bad redis URL %q: %v — offset tracking disabled", redisURL, err)
		return &Store{available: false, prefix: prefix, serviceID: serviceID}
	}
	opt.DB = redisDB
	opt.DialTimeout = 3 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second

	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[offset] Redis not reachable: %v — offset tracking disabled", err)
		return &Store{rdb: rdb, available: false, prefix: prefix, serviceID: serviceID}
	}
	log.Printf("[offset] Redis connected (DB=%d, prefix=%s)", redisDB, prefix)
	return &Store{rdb: rdb, available: true, prefix: prefix, serviceID: serviceID}
}

// Available returns whether the Redis connection is live.
func (s *Store) Available() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

// pathHash returns a stable hash for a file path to use as Redis key component.
func pathHash(path string) string {
	h := sha1.Sum([]byte(path))
	return fmt.Sprintf("%x", h[:10]) // 20 hex chars
}

func (s *Store) offsetKey(path string) string {
	return fmt.Sprintf("%s:offset:%s:%s", s.prefix, s.serviceID, pathHash(path))
}
func (s *Store) metaKey(path string) string {
	return fmt.Sprintf("%s:meta:%s:%s", s.prefix, s.serviceID, pathHash(path))
}

// Load retrieves the saved offset and meta for a file.
// Returns offset=-1 if no offset is saved (first execution).
// Returns error only on Redis communication failure (caller should fail-open).
func (s *Store) Load(ctx context.Context, path string) (int64, *Meta, error) {
	if !s.Available() {
		return -1, nil, nil
	}
	pipe := s.rdb.Pipeline()
	offsetCmd := pipe.Get(ctx, s.offsetKey(path))
	metaCmd := pipe.HGetAll(ctx, s.metaKey(path))
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// Check if offset simply doesn't exist
		if offsetCmd.Err() == redis.Nil {
			return -1, nil, nil
		}
		return -1, nil, fmt.Errorf("offset load: %w", err)
	}

	offsetStr, err := offsetCmd.Result()
	if err == redis.Nil {
		return -1, nil, nil
	}
	if err != nil {
		return -1, nil, fmt.Errorf("offset load get: %w", err)
	}

	off, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return -1, nil, fmt.Errorf("offset parse: %w", err)
	}

	metaMap := metaCmd.Val()
	meta := &Meta{
		Path:     metaMap["path"],
		Filename: metaMap["filename"],
	}
	if v, ok := metaMap["inode"]; ok {
		meta.Inode, _ = strconv.ParseUint(v, 10, 64)
	}
	if v, ok := metaMap["last_sent_at"]; ok {
		meta.LastSentAt, _ = strconv.ParseInt(v, 10, 64)
	}

	return off, meta, nil
}

// Save persists offset and meta atomically via pipeline.
func (s *Store) Save(ctx context.Context, path string, offset int64, inode uint64) error {
	if !s.Available() {
		return nil
	}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, s.offsetKey(path), strconv.FormatInt(offset, 10), 0)
	pipe.HSet(ctx, s.metaKey(path), map[string]interface{}{
		"path":         path,
		"inode":        strconv.FormatUint(inode, 10),
		"last_sent_at": strconv.FormatInt(time.Now().UnixMilli(), 10),
		"filename":     filepath.Base(path),
	})
	_, err := pipe.Exec(ctx)
	if err != nil {
		s.mu.Lock()
		s.available = false
		s.mu.Unlock()
		return fmt.Errorf("offset save: %w", err)
	}
	return nil
}

// IsRotated checks if a file was rotated by comparing inodes.
func IsRotated(savedInode, currentInode uint64) bool {
	return savedInode != 0 && savedInode != currentInode
}

// RewindPosition calculates the rewind position for the restart window.
// It estimates ~60KB of log data per minute as a rough heuristic.
func RewindPosition(savedOffset int64, windowMinutes int) int64 {
	if windowMinutes <= 0 || savedOffset <= 0 {
		return savedOffset
	}
	// Estimate ~60KB of log data per minute
	rewindBytes := int64(windowMinutes) * 60 * 1024
	pos := savedOffset - rewindBytes
	if pos < 0 {
		pos = 0
	}
	return pos
}

// Close closes the Redis client.
func (s *Store) Close() error {
	if s.rdb != nil {
		return s.rdb.Close()
	}
	return nil
}
