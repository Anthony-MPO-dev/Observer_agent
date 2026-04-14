package hub

import (
	"strings"
	"sync"

	"github.com/google/uuid"
	pb "logstream/server/pb"
)

const subscriberBufferSize = 256

// Filter holds the criteria a subscriber uses to select log entries.
type Filter struct {
	ServiceIDs []string
	Levels     []string
	TaskID     string
	Documento  string
	Module     string
	Search     string
}

// Subscriber represents a single active subscriber.
type Subscriber struct {
	ID      string
	Filter  Filter
	Ch      chan *pb.LogEntry
	EventCh chan interface{} // for non-log events (restart, etc.)
}

// Hub is a thread-safe pub/sub broker that fans out LogEntry messages to subscribers.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]*Subscriber
}

// New creates and returns a new Hub.
func New() *Hub {
	return &Hub{
		subs: make(map[string]*Subscriber),
	}
}

// Subscribe registers a new subscriber with the given filter and returns it.
// The caller must call Unsubscribe when done to avoid leaks.
func (h *Hub) Subscribe(filter Filter) *Subscriber {
	sub := &Subscriber{
		ID:      uuid.NewString(),
		Filter:  filter,
		Ch:      make(chan *pb.LogEntry, subscriberBufferSize),
		EventCh: make(chan interface{}, 16),
	}
	h.mu.Lock()
	h.subs[sub.ID] = sub
	h.mu.Unlock()
	return sub
}

// Unsubscribe removes the subscriber and closes its channel.
func (h *Hub) Unsubscribe(subID string) {
	h.mu.Lock()
	sub, ok := h.subs[subID]
	if ok {
		delete(h.subs, subID)
	}
	h.mu.Unlock()

	if ok {
		close(sub.Ch)
		if sub.EventCh != nil {
			close(sub.EventCh)
		}
	}
}

// PublishEvent sends a non-log event (e.g., agent restart notification) to all subscribers.
func (h *Hub) PublishEvent(event interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sub := range h.subs {
		if sub.EventCh == nil {
			continue
		}
		// Non-blocking send: drop if slow consumer.
		select {
		case sub.EventCh <- event:
		default:
		}
	}
}

// Publish fans out an entry to all matching subscribers.
// Subscribers with a full channel are skipped (entries are dropped, never blocked).
func (h *Hub) Publish(entry *pb.LogEntry) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subs {
		if !matches(sub, entry) {
			continue
		}
		// Non-blocking send: drop if slow consumer.
		select {
		case sub.Ch <- entry:
		default:
		}
	}
}

// matches returns true when entry satisfies all filters of the subscriber.
func matches(sub *Subscriber, entry *pb.LogEntry) bool {
	f := sub.Filter

	// Service IDs filter.
	if len(f.ServiceIDs) > 0 {
		found := false
		for _, id := range f.ServiceIDs {
			if id == entry.ServiceId {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Levels filter.
	if len(f.Levels) > 0 {
		levelStr := entry.Level.String()
		found := false
		for _, l := range f.Levels {
			if strings.EqualFold(l, levelStr) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Exact / substring filters.
	if f.TaskID != "" && !strings.Contains(entry.TaskId, f.TaskID) {
		return false
	}
	if f.Documento != "" && !strings.Contains(entry.Documento, f.Documento) {
		return false
	}
	if f.Module != "" && !strings.Contains(entry.Module, f.Module) {
		return false
	}
	if f.Search != "" {
		if !strings.Contains(entry.Message, f.Search) &&
			!strings.Contains(entry.TaskId, f.Search) &&
			!strings.Contains(entry.Documento, f.Search) &&
			!strings.Contains(entry.Module, f.Search) {
			return false
		}
	}

	return true
}
