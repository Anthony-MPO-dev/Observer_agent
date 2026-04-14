// Package depstate stores the latest healthmon dependency status reported by agents.
// Data is kept in memory only — lost on server restart (repopulated by next heartbeat).
package depstate

import (
	"sync"
	"time"

	pb "logstream/server/pb"
)

// ServiceDeps holds the dependency statuses for one agent service.
type ServiceDeps struct {
	ServiceID    string                 `json:"service_id"`
	Dependencies []*pb.DependencyStatus `json:"dependencies"`
	UpdatedAt    int64                  `json:"updated_at"` // unix ms
}

// Store is an in-memory store for dependency statuses keyed by service_id.
type Store struct {
	mu   sync.RWMutex
	data map[string]*ServiceDeps
}

// New creates a new dependency state store.
func New() *Store {
	return &Store{data: make(map[string]*ServiceDeps)}
}

// Update replaces the dependency list for a service.
func (s *Store) Update(serviceID string, deps []*pb.DependencyStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[serviceID] = &ServiceDeps{
		ServiceID:    serviceID,
		Dependencies: deps,
		UpdatedAt:    time.Now().UnixMilli(),
	}
}

// Get returns the dependency statuses for a single service.
func (s *Store) Get(serviceID string) *ServiceDeps {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[serviceID]
}

// All returns dependency statuses for all services.
func (s *Store) All() []*ServiceDeps {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*ServiceDeps, 0, len(s.data))
	for _, v := range s.data {
		result = append(result, v)
	}
	return result
}
