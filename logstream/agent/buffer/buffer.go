package buffer

import (
	"sync"

	pb "logstream/agent/pb"
)

// RingBuffer is a fixed-size circular buffer for LogEntry pointers.
// When the buffer is full, the oldest entry is silently dropped and
// DroppedCount is incremented.
type RingBuffer struct {
	entries      []*pb.LogEntry
	size         int
	head         int // index of next write
	tail         int // index of next read
	count        int
	DroppedCount int64
	mu           sync.Mutex
}

// New creates a RingBuffer with the given capacity.
func New(size int) *RingBuffer {
	return &RingBuffer{
		entries: make([]*pb.LogEntry, size),
		size:    size,
	}
}

// Push adds an entry to the buffer. If the buffer is full, the oldest entry
// is overwritten and DroppedCount is incremented.
func (r *RingBuffer) Push(entry *pb.LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == r.size {
		// Buffer full: overwrite oldest (tail), advance tail
		r.tail = (r.tail + 1) % r.size
		r.DroppedCount++
	} else {
		r.count++
	}

	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.size
}

// DrainAll removes and returns all entries currently in the buffer, resetting it.
func (r *RingBuffer) DrainAll() []*pb.LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return nil
	}

	result := make([]*pb.LogEntry, 0, r.count)
	for i := 0; i < r.count; i++ {
		result = append(result, r.entries[(r.tail+i)%r.size])
	}

	// Reset
	r.head = 0
	r.tail = 0
	r.count = 0

	return result
}

// Len returns the current number of entries in the buffer.
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}
