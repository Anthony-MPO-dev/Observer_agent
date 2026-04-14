package healthmon

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ServiceDef configures one monitored external service.
type ServiceDef struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	HealthPath     string   `json:"health_path"`
	HealthMethod   string   `json:"health_method"`    // HTTP method for ping, default "GET"
	HealthHeaders  map[string]string `json:"health_headers"` // extra headers (e.g. Basic auth)
	AcceptStatus   []int    `json:"accept_status"`    // status codes considered OK, default: any < 500
	Essential      bool     `json:"essential"`
	Fallbacks      []string `json:"fallbacks"`
	ErrorThreshold float64  `json:"error_threshold"`  // 0.0–1.0, default 0.50
	PingIntervalS  int      `json:"ping_interval_s"`  // seconds, default 60
	PingTimeoutS   int      `json:"ping_timeout_s"`   // seconds, default 5

	// resolved durations (not from JSON)
	errorWindow  time.Duration
	pingInterval time.Duration
	pingTimeout  time.Duration
}

func (s *ServiceDef) resolve() {
	if s.ErrorThreshold <= 0 {
		s.ErrorThreshold = 0.50
	}
	if s.PingIntervalS <= 0 {
		s.PingIntervalS = 60
	}
	if s.PingTimeoutS <= 0 {
		s.PingTimeoutS = 5
	}
	if s.HealthPath == "" {
		s.HealthPath = "/"
	}
	if s.HealthMethod == "" {
		s.HealthMethod = "GET"
	}
	s.errorWindow  = time.Minute
	s.pingInterval = time.Duration(s.PingIntervalS) * time.Second
	s.pingTimeout  = time.Duration(s.PingTimeoutS) * time.Second
}

// LoadServices reads HEALTHMON_SERVICES env var (JSON array of ServiceDef).
func LoadServices() ([]ServiceDef, error) {
	raw := os.Getenv("HEALTHMON_SERVICES")
	if raw == "" {
		return nil, nil
	}
	var defs []ServiceDef
	if err := json.Unmarshal([]byte(raw), &defs); err != nil {
		return nil, fmt.Errorf("healthmon: parse HEALTHMON_SERVICES: %w", err)
	}
	for i := range defs {
		defs[i].resolve()
	}
	return defs, nil
}

// HealthmonPort returns the configured HTTP port for the healthmon API.
func HealthmonPort() string {
	if v := os.Getenv("HEALTHMON_PORT"); v != "" {
		if v[0] != ':' {
			return ":" + v
		}
		return v
	}
	return ":9091"
}
