package cleaner

import (
	"context"
	"log"
	"time"

	"logstream/server/config"
	"logstream/server/db"
	"logstream/server/store"
)

// Cleaner runs a daily log-file cleanup job.
// For each service it reads the TTL from the database (set by the dashboard)
// and removes JSONL files older than that many days.
type Cleaner struct {
	cfg   *config.Config
	db    *db.DB
	store *store.Store
}

// New creates a new Cleaner.
func New(cfg *config.Config, database *db.DB, st *store.Store) *Cleaner {
	return &Cleaner{cfg: cfg, db: database, store: st}
}

// Start runs the cleanup loop. It blocks until ctx is cancelled.
// The first run is triggered at the next occurrence of cfg.CleanupHour (UTC).
func (c *Cleaner) Start(ctx context.Context) {
	for {
		next := c.nextRun()
		log.Printf("cleaner: next cleanup scheduled at %s", next.Format(time.RFC3339))

		select {
		case <-ctx.Done():
			log.Println("cleaner: shutting down")
			return
		case <-time.After(time.Until(next)):
		}

		c.runOnce()
	}
}

// runOnce performs a single cleanup pass over all known services.
// The TTL for each service is read from the database so the dashboard controls it.
func (c *Cleaner) runOnce() {
	log.Println("cleaner: starting cleanup run")
	services, err := c.db.ListServices()
	if err != nil {
		log.Printf("cleaner: list services: %v", err)
		return
	}

	totalDeleted := 0
	for _, svc := range services {
		// Resolve TTL: prefer per-service config (controlled by dashboard), fall back to default.
		ttlDays := c.cfg.DefaultTTLDays
		if svc.Config != nil && svc.Config.TtlDays > 0 {
			ttlDays = int(svc.Config.TtlDays)
		}

		cutoff := time.Now().UTC().AddDate(0, 0, -ttlDays)
		deleted, err := c.store.DeleteOlderThan(svc.ID, cutoff)
		if err != nil {
			log.Printf("cleaner: service %s: delete error: %v", svc.ID, err)
			continue
		}
		if deleted > 0 {
			log.Printf("cleaner: service %s: deleted %d file(s) older than %s (ttl=%d days)",
				svc.ID, deleted, cutoff.Format("2006-01-02"), ttlDays)
			totalDeleted += deleted
		}
	}
	log.Printf("cleaner: cleanup run complete — %d file(s) deleted", totalDeleted)
}

// nextRun returns the next wall-clock time at which the cleanup hour should fire (UTC).
func (c *Cleaner) nextRun() time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), c.cfg.CleanupHour, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
