package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"logstream/agent/buffer"
	"logstream/agent/config"
	"logstream/agent/healthmon"
	"logstream/agent/offset"
	"logstream/agent/parser"
	"logstream/agent/sender"
	"logstream/agent/watcher"
)

func main() {
	cfg := config.Load()

	agentID := uuid.NewString()

	log.Printf("[logstream-agent] starting")
	log.Printf("  service_id  : %s", cfg.ServiceID)
	log.Printf("  service_name: %s", cfg.ServiceName)
	log.Printf("  server_addr : %s", cfg.ServerAddr)
	log.Printf("  log_volume  : %s", cfg.LogVolume)
	log.Printf("  agent_id    : %s", agentID)
	log.Printf("  file_prefix : %s", cfg.FilenamePrefix)
	log.Printf("  batch_size  : %d", cfg.BatchSize)
	log.Printf("  flush_ms    : %d", cfg.FlushMs)
	log.Printf("  buffer_size : %d", cfg.BufferSize)
	log.Printf("  tls_enabled : %v", cfg.TLSEnabled)
	log.Printf("  redis_url   : %s", cfg.RedisURL)
	log.Printf("  redis_db    : %d", cfg.RedisDB)
	log.Printf("  restart_win : %d min", cfg.RestartWindowMinutes)
	log.Printf("  read_exist  : %v", cfg.ReadExisting)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGTERM / SIGINT
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigs
		log.Printf("[logstream-agent] received signal %s — shutting down", sig)
		cancel()
	}()

	buf := buffer.New(cfg.BufferSize)
	offsets := offset.New(cfg.RedisURL, cfg.RedisDB, cfg.RedisKeyPrefix, cfg.ServiceID)
	defer offsets.Close()
	p := parser.New(cfg.ServiceID, cfg.ServiceName, agentID, cfg.FilenamePrefix)
	w := watcher.New(cfg.LogVolume, p, offsets, cfg)
	s := sender.New(cfg, buf, agentID, offsets)

	// healthmon — circuit breaker + health API on HEALTHMON_PORT
	hmDefs, err := healthmon.LoadServices()
	if err != nil {
		log.Printf("[healthmon] config error: %v — skipping healthmon", err)
	} else {
		hm := healthmon.New(hmDefs, cfg.ServiceID, cfg.ServiceName, agentID, cfg.HealthmonPort, s.Send)
		s.SetDependencyProvider(hm)
		go hm.Start(ctx)
	}

	// /metrics HTTP endpoint
	go serveMetrics(cfg.MetricsPort, s, buf)

	// Fan entries from watcher → sender
	go func() {
		forwarded := 0
		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-w.EntryCh():
				if !ok {
					return
				}
				s.Send(entry)
				forwarded++
				if forwarded == 1 || forwarded%500 == 0 {
					log.Printf("[main] forwarded %d entries to sender (queue)", forwarded)
				}
			}
		}
	}()

	// Start watcher (background goroutine)
	go func() {
		if err := w.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[logstream-agent] watcher error: %v", err)
		}
	}()

	// Start sender (blocks until ctx done)
	s.Start(ctx)

	log.Printf("[logstream-agent] stopped")
}

func serveMetrics(addr string, s *sender.Sender, buf *buffer.RingBuffer) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "connected %v\n", s.Connected)
		fmt.Fprintf(w, "buffer_used %d\n", buf.Len())
		fmt.Fprintf(w, "dropped_total %d\n", buf.DroppedCount)
		fmt.Fprintf(w, "logs_per_sec %.2f\n", s.LogsPerSec)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	log.Printf("[logstream-agent] metrics listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("[logstream-agent] metrics server error: %v", err)
	}
}
