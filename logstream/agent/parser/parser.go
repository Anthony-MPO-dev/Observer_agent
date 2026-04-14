package parser

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "logstream/agent/pb"
)

// reModule extracts the module name from "[consulta_service.py:93]" → "consulta_service"
var reModule = regexp.MustCompile(`^([^\.]+)`)

// Compiled regexes for log lines.
// Expected format: 2026-03-31 12:17:30 [INFO] [uuid=-] [file.py:line] func() - [DOC:xxx] message
var (
	reWithDoc = regexp.MustCompile(
		`^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[(?P<level>\w+)\] \[uuid=[^\]]*\] \[(?P<module>[^\]]+)\] \S+\(\) - \[DOC:(?P<documento>[^\]]+)\] (?P<message>.+)$`,
	)
	reWithoutDoc = regexp.MustCompile(
		`^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) \[(?P<level>\w+)\] \[uuid=[^\]]*\] \[(?P<module>[^\]]+)\] \S+\(\) - (?P<message>.+)$`,
	)

	// reDispatch captures [DISPATCH:task_id] from log messages for cross-worker tracing.
	reDispatch = regexp.MustCompile(`\[DISPATCH:([a-f0-9\-]{36})\]`)
)

// buildFilenameRegex builds a regex to extract metadata from log filenames.
// The prefix is configurable so the same agent binary works with any API.
// Pattern: {prefix}_worker_consulta_{uuid}_2026-03-31_12-00-00.log
//          {prefix}_background_2026-03-31_12-00-00.log
func buildFilenameRegex(prefix string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(prefix)
	pattern := `^` + escaped + `_(?P<log_type>worker_(?:consulta|background)_(?P<task_id>[a-f0-9\-]{36})|[^_]+)_\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}\.log$`
	return regexp.MustCompile(pattern)
}

// FileInfo holds metadata extracted from a log filename.
type FileInfo struct {
	TaskID     string
	WorkerType string // "core" | "quick" | "background" | "unknown"
	Queue      string
}

// Parser parses log lines into LogEntry objects.
type Parser struct {
	serviceID      string
	serviceName    string
	agentID        string
	reFilename     *regexp.Regexp
	filenamePrefix string
}

// New creates a Parser with the given service identifiers.
// filenamePrefix is the log filename prefix (e.g. "dados_basicos").
// If empty, falls back to serviceID.
func New(serviceID, serviceName, agentID, filenamePrefix string) *Parser {
	if filenamePrefix == "" {
		filenamePrefix = serviceID
	}
	return &Parser{
		serviceID:      serviceID,
		serviceName:    serviceName,
		agentID:        agentID,
		reFilename:     buildFilenameRegex(filenamePrefix),
		filenamePrefix: filenamePrefix,
	}
}

// ParseFilename extracts metadata from a log filename.
// Returns a FileInfo with best-effort fields; unknown fields default to "unknown".
func (p *Parser) ParseFilename(filename string) FileInfo {
	base := filepath.Base(filename)
	m := p.reFilename.FindStringSubmatch(base)
	if m == nil {
		return FileInfo{WorkerType: "unknown", Queue: "unknown"}
	}

	names := p.reFilename.SubexpNames()
	info := FileInfo{WorkerType: "unknown", Queue: "unknown"}

	for i, name := range names {
		if i == 0 || m[i] == "" {
			continue
		}
		switch name {
		case "task_id":
			info.TaskID = m[i]
		case "log_type":
			logType := m[i]
			switch {
			case strings.HasPrefix(logType, "worker_consulta_"):
				info.WorkerType = "core"
				info.Queue = "consulta"
			case strings.HasPrefix(logType, "worker_background_"):
				info.WorkerType = "background"
				info.Queue = "background"
			case logType == "quick":
				info.WorkerType = "quick"
				info.Queue = "quick"
			case logType == "background":
				info.WorkerType = "background"
				info.Queue = "background"
			default:
				info.WorkerType = logType
				info.Queue = logType
			}
		}
	}

	return info
}

// ParseLine parses a single log line into a LogEntry.
// Returns nil if the line does not match any known pattern.
// fileInfo provides metadata derived from the log filename.
func (p *Parser) ParseLine(line string, fileInfo FileInfo, logFile string) *pb.LogEntry {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return nil
	}

	// Try with-document pattern first
	var (
		timestamp string
		level     string
		module    string
		documento string
		message   string
	)

	if m := reWithDoc.FindStringSubmatch(line); m != nil {
		names := reWithDoc.SubexpNames()
		for i, name := range names {
			if i == 0 {
				continue
			}
			switch name {
			case "timestamp":
				timestamp = m[i]
			case "level":
				level = m[i]
			case "module":
				// "[consulta_service.py:93]" → "consulta_service"
				if mm := reModule.FindString(m[i]); mm != "" {
					module = mm
				} else {
					module = m[i]
				}
			case "documento":
				documento = m[i]
			case "message":
				message = m[i]
			}
		}
	} else if m := reWithoutDoc.FindStringSubmatch(line); m != nil {
		names := reWithoutDoc.SubexpNames()
		for i, name := range names {
			if i == 0 {
				continue
			}
			switch name {
			case "timestamp":
				timestamp = m[i]
			case "level":
				level = m[i]
			case "module":
				if mm := reModule.FindString(m[i]); mm != "" {
					module = mm
				} else {
					module = m[i]
				}
			case "message":
				message = m[i]
			}
		}
	} else {
		// Line doesn't match any pattern
		return nil
	}

	// Parse timestamp using the agent's local timezone (set via TZ env var).
	// Python workers log in local time; treating as UTC would shift the display.
	var unixMs int64
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", timestamp, time.Local); err == nil {
		unixMs = t.UnixMilli()
	} else {
		unixMs = time.Now().UnixMilli()
	}

	extra := map[string]string{
		"log_file":    logFile,
		"worker_type": fileInfo.WorkerType,
		"queue":       fileInfo.Queue,
		"service":     p.serviceName,
	}

	// Extract [DISPATCH:task_id] from message for cross-worker tracing
	if dm := reDispatch.FindStringSubmatch(message); dm != nil {
		extra["dispatch_task_id"] = dm[1]
	}

	entry := &pb.LogEntry{
		ServiceId: p.serviceID,
		Level:     ParseLevel(level),
		Message:   message,
		Timestamp: unixMs,
		TaskId:    fileInfo.TaskID,
		Documento: documento,
		Module:    module,
		AgentId:   p.agentID,
		TraceId:   uuid.NewString(),
		Extra:     extra,
	}

	return entry
}

// ParseLevel converts a level string to the corresponding pb.LogLevel.
func ParseLevel(s string) pb.LogLevel {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return pb.LogLevel_DEBUG
	case "INFO":
		return pb.LogLevel_INFO
	case "WARNING", "WARN":
		return pb.LogLevel_WARNING
	case "ERROR":
		return pb.LogLevel_ERROR
	case "CRITICAL", "FATAL":
		return pb.LogLevel_FATAL
	default:
		return pb.LogLevel_INFO
	}
}
