package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	pb "logstream/server/pb"
)

const dateLayout = "2006-01-02"

// Store handles file-based log persistence as JSONL files.
// Files are stored at: {baseDir}/{service_id}/{YYYY-MM-DD}.jsonl
type Store struct {
	baseDir string
	mu      sync.Mutex
}

// New creates a Store rooted at baseDir.
func New(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// serviceDir returns the directory for a given service.
func (s *Store) serviceDir(serviceID string) string {
	return filepath.Join(s.baseDir, serviceID)
}

// logFile returns the path to today's log file for a service.
func (s *Store) logFile(serviceID string, date time.Time) string {
	return filepath.Join(s.serviceDir(serviceID), date.Format(dateLayout)+".jsonl")
}

// Write appends a single LogEntry as a JSON line to today's log file.
func (s *Store) Write(entry *pb.LogEntry) error {
	if entry.ServiceId == "" {
		return fmt.Errorf("store: write: empty service_id")
	}

	dir := s.serviceDir(entry.ServiceId)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir: %w", err)
	}

	path := s.logFile(entry.ServiceId, time.Now().UTC())

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("store: open file: %w", err)
	}
	defer f.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	line = append(line, '\n')

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("store: write: %w", err)
	}
	return nil
}

// Query scans JSONL files matching the request filters and returns up to req.Limit entries.
func (s *Store) Query(req *pb.QueryRequest) ([]*pb.LogEntry, int, error) {
	if req.ServiceId == "" {
		return nil, 0, fmt.Errorf("store: query: empty service_id")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.GetOffset())

	// Collect the set of dates to scan, derived from from_ts / to_ts.
	dates, err := s.datesInRange(req.ServiceId, req.FromTs, req.ToTs)
	if err != nil {
		return nil, 0, err
	}

	// Build a level set for fast lookup.
	levelSet := make(map[string]bool, len(req.Levels))
	for _, l := range req.Levels {
		levelSet[strings.ToUpper(l)] = true
	}

	var matched []*pb.LogEntry
	total := 0

	for _, date := range dates {
		path := filepath.Join(s.serviceDir(req.ServiceId), date+".jsonl")
		entries, err := s.scanFile(path, req, levelSet)
		if err != nil {
			// Skip files that cannot be read.
			continue
		}
		for _, e := range entries {
			total++
			if total <= offset {
				continue
			}
			if len(matched) < limit {
				matched = append(matched, e)
			}
		}
	}

	return matched, total, nil
}

// scanFile reads all log entries from a single JSONL file that pass the filters.
func (s *Store) scanFile(path string, req *pb.QueryRequest, levelSet map[string]bool) ([]*pb.LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []*pb.LogEntry
	scanner := bufio.NewScanner(f)
	// Allow up to 1 MB per line.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry pb.LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if !matchesQuery(&entry, req, levelSet) {
			continue
		}
		results = append(results, &entry)
	}
	return results, scanner.Err()
}

// matchesQuery checks whether an entry satisfies all request filters.
func matchesQuery(e *pb.LogEntry, req *pb.QueryRequest, levelSet map[string]bool) bool {
	// Timestamp range (milliseconds).
	if req.FromTs > 0 && e.Timestamp < req.FromTs {
		return false
	}
	if req.ToTs > 0 && e.Timestamp > req.ToTs {
		return false
	}

	// Level filter.
	if len(levelSet) > 0 {
		if !levelSet[e.Level.String()] {
			return false
		}
	}

	// Field filters (case-sensitive substring match).
	if req.TaskId != "" && !strings.Contains(e.TaskId, req.TaskId) {
		return false
	}
	if req.Documento != "" && !strings.Contains(e.Documento, req.Documento) {
		return false
	}
	if req.Module != "" && !strings.Contains(e.Module, req.Module) {
		return false
	}
	if req.Search != "" {
		if !strings.Contains(e.Message, req.Search) &&
			!strings.Contains(e.TaskId, req.Search) &&
			!strings.Contains(e.Documento, req.Search) &&
			!strings.Contains(e.Module, req.Search) {
			return false
		}
	}
	return true
}

// DeleteOlderThan removes JSONL files for serviceID older than the cutoff date.
// It returns the number of files deleted.
func (s *Store) DeleteOlderThan(serviceID string, cutoff time.Time) (int, error) {
	dir := s.serviceDir(serviceID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: delete older than: read dir: %w", err)
	}

	cutoffStr := cutoff.UTC().Format(dateLayout)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		dateStr := strings.TrimSuffix(name, ".jsonl")
		// Lexicographic comparison works because dates are YYYY-MM-DD.
		if dateStr < cutoffStr {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return deleted, fmt.Errorf("store: remove %s: %w", path, err)
			}
			deleted++
		}
	}
	return deleted, nil
}

// ListDates returns sorted date strings (YYYY-MM-DD) that have log files for a service.
func (s *Store) ListDates(serviceID string) ([]string, error) {
	dir := s.serviceDir(serviceID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: list dates: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") {
			dates = append(dates, strings.TrimSuffix(name, ".jsonl"))
		}
	}
	sort.Strings(dates)
	return dates, nil
}

// TaskInfo holds metadata about a unique task_id found in the log store.
type TaskInfo struct {
	TaskID      string `json:"task_id"`
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	WorkerType  string `json:"worker_type"`
	Queue       string `json:"queue"`
	Count       int    `json:"count"`
	ErrorCount  int    `json:"error_count"`
	WarnCount   int    `json:"warn_count"`
	FirstSeen   int64  `json:"first_seen"` // unix ms
	LastSeen    int64  `json:"last_seen"`  // unix ms
}

// ListTasks scans JSONL files for a service within a date range and returns
// unique task_ids with aggregated metadata. If serviceID is empty, scans all services.
func (s *Store) ListTasks(serviceIDs []string, fromTs, toTs int64) ([]TaskInfo, error) {
	tasks := make(map[string]*TaskInfo) // key: serviceID + ":" + taskID

	for _, svcID := range serviceIDs {
		dates, err := s.datesInRange(svcID, fromTs, toTs)
		if err != nil {
			continue
		}
		for _, date := range dates {
			path := filepath.Join(s.serviceDir(svcID), date+".jsonl")
			s.scanTasksFromFile(path, svcID, tasks, fromTs, toTs)
		}
	}

	result := make([]TaskInfo, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen > result[j].LastSeen // most recent first
	})
	return result, nil
}

// scanTasksFromFile reads a JSONL file and accumulates task_id metadata.
func (s *Store) scanTasksFromFile(path, serviceID string, tasks map[string]*TaskInfo, fromTs, toTs int64) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry pb.LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if fromTs > 0 && entry.Timestamp < fromTs {
			continue
		}
		if toTs > 0 && entry.Timestamp > toTs {
			continue
		}
		if entry.TaskId == "" {
			continue
		}

		key := serviceID + ":" + entry.TaskId
		t, ok := tasks[key]
		if !ok {
			t = &TaskInfo{
				TaskID:      entry.TaskId,
				ServiceID:   serviceID,
				ServiceName: entry.ServiceName,
				WorkerType:  entry.WorkerType,
				Queue:       entry.Queue,
				FirstSeen:   entry.Timestamp,
				LastSeen:    entry.Timestamp,
			}
			tasks[key] = t
		}
		t.Count++
		if entry.Timestamp < t.FirstSeen {
			t.FirstSeen = entry.Timestamp
		}
		if entry.Timestamp > t.LastSeen {
			t.LastSeen = entry.Timestamp
		}
		lvl := strings.ToUpper(entry.Level.String())
		if lvl == "ERROR" || lvl == "FATAL" {
			t.ErrorCount++
		} else if lvl == "WARNING" {
			t.WarnCount++
		}
	}
}

// datesInRange returns the YYYY-MM-DD strings that fall within [fromTs, toTs] for a service.
// If fromTs/toTs are zero, all available dates are returned.
func (s *Store) datesInRange(serviceID string, fromTs, toTs int64) ([]string, error) {
	all, err := s.ListDates(serviceID)
	if err != nil {
		return nil, err
	}

	if fromTs == 0 && toTs == 0 {
		return all, nil
	}

	var fromDate, toDate string
	if fromTs > 0 {
		fromDate = time.UnixMilli(fromTs).UTC().Format(dateLayout)
	}
	if toTs > 0 {
		toDate = time.UnixMilli(toTs).UTC().Format(dateLayout)
	}

	var result []string
	for _, d := range all {
		if fromDate != "" && d < fromDate {
			continue
		}
		if toDate != "" && d > toDate {
			continue
		}
		result = append(result, d)
	}
	return result, nil
}
