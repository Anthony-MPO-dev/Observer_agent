package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "logstream/server/pb"

	_ "modernc.org/sqlite"
)

// DB wraps an SQLite connection and exposes typed operations for service metadata.
type DB struct {
	conn *sql.DB
}

// Open creates (if necessary) the data directory, opens the SQLite database, and applies the schema.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("db: create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "logstream.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite: %w", err)
	}

	// Recommended pragmas for a server workload.
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			return nil, fmt.Errorf("db: pragma %q: %w", p, err)
		}
	}

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// migrate applies the schema DDL (idempotent with IF NOT EXISTS).
func (d *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS services (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    last_seen INTEGER,
    status    TEXT DEFAULT 'offline',
    agent_id  TEXT,
    version   TEXT
);

CREATE TABLE IF NOT EXISTS service_configs (
    service_id TEXT PRIMARY KEY,
    ttl_days   INTEGER DEFAULT 30,
    min_level  TEXT    DEFAULT 'INFO',
    batch_size INTEGER DEFAULT 100,
    flush_ms   INTEGER DEFAULT 500,
    enabled    INTEGER DEFAULT 1,
    FOREIGN KEY(service_id) REFERENCES services(id)
);

CREATE TABLE IF NOT EXISTS agent_stats (
    agent_id     TEXT PRIMARY KEY,
    service_id   TEXT,
    buffer_used  INTEGER,
    dropped_total INTEGER,
    logs_per_sec  REAL,
    updated_at   INTEGER
);
`
	_, err := d.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}

// UpsertService inserts or updates a service row and seeds a config row with defaults.
func (d *DB) UpsertService(serviceID, name, agentID, version string) error {
	now := time.Now().UnixMilli()
	_, err := d.conn.Exec(`
		INSERT INTO services (id, name, last_seen, status, agent_id, version)
		VALUES (?, ?, ?, 'online', ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name      = excluded.name,
			last_seen = excluded.last_seen,
			status    = 'online',
			agent_id  = excluded.agent_id,
			version   = excluded.version
	`, serviceID, name, now, agentID, version)
	if err != nil {
		return fmt.Errorf("db: upsert service: %w", err)
	}

	// Seed config row if not present.
	_, err = d.conn.Exec(`
		INSERT OR IGNORE INTO service_configs (service_id)
		VALUES (?)
	`, serviceID)
	if err != nil {
		return fmt.Errorf("db: seed config: %w", err)
	}
	return nil
}

// SetServiceStatus updates the status field of a service.
func (d *DB) SetServiceStatus(serviceID, status string) error {
	_, err := d.conn.Exec(`
		UPDATE services SET status = ?, last_seen = ? WHERE id = ?
	`, status, time.Now().UnixMilli(), serviceID)
	if err != nil {
		return fmt.Errorf("db: set service status: %w", err)
	}
	return nil
}

// GetConfig returns the ServiceConfig for the given service. If no config row
// exists yet it returns sensible defaults.
func (d *DB) GetConfig(serviceID string) (*pb.ServiceConfig, error) {
	row := d.conn.QueryRow(`
		SELECT ttl_days, min_level, batch_size, flush_ms, enabled
		FROM service_configs
		WHERE service_id = ?
	`, serviceID)

	var ttlDays, batchSize, flushMs int32
	var minLevel string
	var enabledInt int
	err := row.Scan(&ttlDays, &minLevel, &batchSize, &flushMs, &enabledInt)
	if err == sql.ErrNoRows {
		return defaultConfig(serviceID), nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get config: %w", err)
	}

	return &pb.ServiceConfig{
		ServiceId: serviceID,
		TtlDays:   ttlDays,
		MinLevel:  pb.LogLevelFromString(minLevel),
		BatchSize: batchSize,
		FlushMs:   flushMs,
		Enabled:   enabledInt != 0,
	}, nil
}

// UpsertConfig persists a ServiceConfig, creating the row if needed.
func (d *DB) UpsertConfig(cfg *pb.ServiceConfig) error {
	_, err := d.conn.Exec(`
		INSERT INTO service_configs (service_id, ttl_days, min_level, batch_size, flush_ms, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(service_id) DO UPDATE SET
			ttl_days   = excluded.ttl_days,
			min_level  = excluded.min_level,
			batch_size = excluded.batch_size,
			flush_ms   = excluded.flush_ms,
			enabled    = excluded.enabled
	`, cfg.ServiceId, cfg.TtlDays, cfg.MinLevel.String(), cfg.BatchSize, cfg.FlushMs, boolToInt(cfg.Enabled))
	if err != nil {
		return fmt.Errorf("db: upsert config: %w", err)
	}
	return nil
}

// ServiceRow is the DTO returned by ListServices.
type ServiceRow struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	LastSeen int64            `json:"last_seen"`
	Status   string           `json:"status"`
	AgentID  string           `json:"agent_id"`
	Version  string           `json:"version"`
	Config   *pb.ServiceConfig `json:"config"`
}

// ListServices returns all services joined with their configs.
func (d *DB) ListServices() ([]*ServiceRow, error) {
	rows, err := d.conn.Query(`
		SELECT s.id, s.name, s.last_seen, s.status, COALESCE(s.agent_id,''), COALESCE(s.version,''),
		       COALESCE(c.ttl_days,30), COALESCE(c.min_level,'INFO'),
		       COALESCE(c.batch_size,100), COALESCE(c.flush_ms,500), COALESCE(c.enabled,1)
		FROM services s
		LEFT JOIN service_configs c ON c.service_id = s.id
		ORDER BY s.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db: list services: %w", err)
	}
	defer rows.Close()

	var result []*ServiceRow
	for rows.Next() {
		var row ServiceRow
		var ttlDays, batchSize, flushMs int32
		var minLevel string
		var enabledInt int
		if err := rows.Scan(
			&row.ID, &row.Name, &row.LastSeen, &row.Status, &row.AgentID, &row.Version,
			&ttlDays, &minLevel, &batchSize, &flushMs, &enabledInt,
		); err != nil {
			return nil, fmt.Errorf("db: scan service: %w", err)
		}
		row.Config = &pb.ServiceConfig{
			ServiceId: row.ID,
			TtlDays:   ttlDays,
			MinLevel:  pb.LogLevelFromString(minLevel),
			BatchSize: batchSize,
			FlushMs:   flushMs,
			Enabled:   enabledInt != 0,
		}
		result = append(result, &row)
	}
	return result, rows.Err()
}

// UpdateAgentStats upserts runtime statistics for an agent.
func (d *DB) UpdateAgentStats(agentID, serviceID string, bufferUsed, dropped int64, logsPerSec float32) error {
	now := time.Now().UnixMilli()
	_, err := d.conn.Exec(`
		INSERT INTO agent_stats (agent_id, service_id, buffer_used, dropped_total, logs_per_sec, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			service_id    = excluded.service_id,
			buffer_used   = excluded.buffer_used,
			dropped_total = excluded.dropped_total,
			logs_per_sec  = excluded.logs_per_sec,
			updated_at    = excluded.updated_at
	`, agentID, serviceID, bufferUsed, dropped, logsPerSec, now)
	if err != nil {
		return fmt.Errorf("db: update agent stats: %w", err)
	}
	return nil
}

// CountServicesOnline returns the number of services with status 'online'.
func (d *DB) CountServicesOnline() (int, error) {
	var n int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM services WHERE status = 'online'`).Scan(&n)
	return n, err
}

// helpers

func defaultConfig(serviceID string) *pb.ServiceConfig {
	return &pb.ServiceConfig{
		ServiceId: serviceID,
		TtlDays:   30,
		MinLevel:  pb.LogLevel_INFO,
		BatchSize: 100,
		FlushMs:   500,
		Enabled:   true,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
