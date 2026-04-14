package healthmon

import (
	"sync"
	"time"
)

type circuitStatus string

const (
	statusClosed   circuitStatus = "CLOSED"
	statusOpen     circuitStatus = "OPEN"
	statusHalfOpen circuitStatus = "HALF_OPEN"
)

// event is a single request outcome in the sliding window.
type event struct {
	ts      time.Time
	success bool
}

// Circuit implements a per-service circuit breaker with a sliding window.
type Circuit struct {
	def ServiceDef

	mu               sync.Mutex
	status           circuitStatus
	window           []event    // sliding window of outcomes
	openedAt         *time.Time
	halfOpenInFlight bool // true while a real request is allowed through

	Downtimes []Downtime
}

// Downtime records one open→closed episode.
type Downtime struct {
	OpenedAt    time.Time  `json:"opened_at"`
	RecoveredAt *time.Time `json:"recovered_at"`
	Duration    *float64   `json:"duration_seconds"` // nil while still open
}

func newCircuit(def ServiceDef) *Circuit {
	return &Circuit{
		def:    def,
		status: statusClosed,
	}
}

// Report records the result of a real request and potentially transitions state.
// Returns true if the circuit just opened (caller should emit event).
func (c *Circuit) Report(success bool) (justOpened bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.window = append(c.window, event{ts: now, success: success})
	c.pruneWindow(now)

	switch c.status {
	case statusClosed:
		if c.errorRate() > c.def.ErrorThreshold {
			c.open(now)
			return true
		}
	case statusHalfOpen:
		if success {
			c.close(now)
		} else {
			c.open(now)
			return true
		}
		c.halfOpenInFlight = false
	}
	return false
}

// PingResult records the result of a background ping.
// Returns true if the circuit transitioned to HALF_OPEN.
func (c *Circuit) PingResult(ok bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == statusOpen && ok {
		c.status = statusHalfOpen
		c.halfOpenInFlight = false
		return true
	}
	return false
}

// Check returns whether the service is considered available and whether
// to allow a real request (for HALF_OPEN token).
// Returns: available bool, useHalfOpen bool
func (c *Circuit) Check() (available bool, halfOpenToken bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.status {
	case statusClosed:
		return true, false
	case statusOpen:
		return false, false
	case statusHalfOpen:
		if !c.halfOpenInFlight {
			c.halfOpenInFlight = true
			return true, true
		}
		return false, false
	}
	return false, false
}

// Status returns a snapshot of the circuit state (safe for concurrent read).
func (c *Circuit) Status() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.pruneWindow(now)

	var totalReqs, totalErrs int64
	for _, e := range c.window {
		totalReqs++
		if !e.success {
			totalErrs++
		}
	}

	return CircuitState{
		ServiceID:     c.def.ID,
		Status:        string(c.status),
		ErrorRate:     c.errorRate(),
		TotalRequests: totalReqs,
		TotalErrors:   totalErrs,
		OpenedAt:      c.openedAt,
		Downtimes:     append([]Downtime{}, c.Downtimes...),
	}
}

// --- internal helpers (must be called with mu held) ---

func (c *Circuit) open(now time.Time) {
	if c.status != statusOpen {
		t := now
		c.openedAt = &t
		c.Downtimes = append(c.Downtimes, Downtime{OpenedAt: now})
	}
	c.status = statusOpen
	c.halfOpenInFlight = false
}

func (c *Circuit) close(now time.Time) {
	if len(c.Downtimes) > 0 {
		last := &c.Downtimes[len(c.Downtimes)-1]
		if last.RecoveredAt == nil {
			last.RecoveredAt = &now
			dur := now.Sub(last.OpenedAt).Seconds()
			last.Duration = &dur
		}
	}
	c.status = statusClosed
	c.openedAt = nil
	c.window = c.window[:0]
}

func (c *Circuit) pruneWindow(now time.Time) {
	cutoff := now.Add(-c.def.errorWindow)
	i := 0
	for i < len(c.window) && c.window[i].ts.Before(cutoff) {
		i++
	}
	c.window = c.window[i:]
}

func (c *Circuit) errorRate() float64 {
	if len(c.window) == 0 {
		return 0
	}
	errs := 0
	for _, e := range c.window {
		if !e.success {
			errs++
		}
	}
	return float64(errs) / float64(len(c.window))
}

// CircuitState is the exported snapshot used by the API.
type CircuitState struct {
	ServiceID     string     `json:"service_id"`
	Status        string     `json:"status"`
	ErrorRate     float64    `json:"error_rate"`
	TotalRequests int64      `json:"total_requests"`
	TotalErrors   int64      `json:"total_errors"`
	OpenedAt      *time.Time `json:"opened_at,omitempty"`
	RecoveredAt   *time.Time `json:"recovered_at,omitempty"`
	LastPingAt    *time.Time `json:"last_ping_at,omitempty"`
	LastPingOK    bool       `json:"last_ping_ok"`
	Downtimes     []Downtime `json:"downtimes"`
}
