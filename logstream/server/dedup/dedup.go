package dedup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	pb "logstream/server/pb"
)

const (
	dedupTTL         = 10 * time.Minute
	restartNotifyTTL = 5 * time.Minute
	dedupCountTTL    = 24 * time.Hour
)

// Deduplicator checks for duplicate log entries using Redis.
type Deduplicator struct {
	rdb       *redis.Client
	available bool
}

// New creates a Deduplicator. If Redis is not available, all entries pass through.
func New(redisURL string, redisDB int) *Deduplicator {
	if redisURL == "" {
		log.Printf("[dedup] no REDIS_URL configured — dedup disabled")
		return &Deduplicator{available: false}
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[dedup] bad redis URL: %v — dedup disabled", err)
		return &Deduplicator{available: false}
	}
	opt.DB = redisDB
	opt.DialTimeout = 3 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second

	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[dedup] Redis not reachable: %v — dedup disabled", err)
		return &Deduplicator{rdb: rdb, available: false}
	}
	log.Printf("[dedup] Redis connected (DB=%d)", redisDB)
	return &Deduplicator{rdb: rdb, available: true}
}

// IsDuplicate returns true if this entry was already seen.
// Only checks entries with Extra["replayed"]=="true".
// Entries without replayed flag always return false (no overhead).
func (d *Deduplicator) IsDuplicate(ctx context.Context, entry *pb.LogEntry) bool {
	if !d.available {
		return false
	}
	if entry.Extra == nil || entry.Extra["replayed"] != "true" {
		return false
	}

	hash := entryHash(entry)
	key := "logserver:dedup:" + hash

	// SetNX: set if not exists, with TTL
	set, err := d.rdb.SetNX(ctx, key, "1", dedupTTL).Result()
	if err != nil {
		// On error, let the entry through (fail-open)
		return false
	}
	if !set {
		// Key already existed — this is a duplicate
		d.incrementDropCount(ctx, entry.ServiceId)
		return true
	}
	return false
}

// ShouldNotifyRestart checks if this is the first replayed entry for a service
// in the current restart window. Returns true only once per 5-min window.
func (d *Deduplicator) ShouldNotifyRestart(ctx context.Context, serviceID string) bool {
	if !d.available {
		return false
	}
	key := "logserver:restart_notified:" + serviceID
	set, err := d.rdb.SetNX(ctx, key, "1", restartNotifyTTL).Result()
	if err != nil {
		return false
	}
	return set // true only the first time
}

// GetDropCount returns the number of duplicates dropped for a service today.
func (d *Deduplicator) GetDropCount(ctx context.Context, serviceID string) int64 {
	if !d.available {
		return 0
	}
	key := "logserver:dedup_count:" + serviceID
	val, err := d.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0
	}
	return val
}

func (d *Deduplicator) incrementDropCount(ctx context.Context, serviceID string) {
	key := "logserver:dedup_count:" + serviceID
	pipe := d.rdb.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, dedupCountTTL)
	pipe.Exec(ctx)
}

func entryHash(entry *pb.LogEntry) string {
	raw := fmt.Sprintf("%s|%s|%d|%s|%s",
		entry.ServiceId,
		entry.AgentId,
		entry.Timestamp,
		entry.Level.String(),
		entry.Message,
	)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h[:16]) // 32 hex chars
}

// Close closes the Redis connection.
func (d *Deduplicator) Close() error {
	if d.rdb != nil {
		return d.rdb.Close()
	}
	return nil
}
