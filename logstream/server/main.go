package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"logstream/server/api"
	"logstream/server/cleaner"
	"logstream/server/config"
	"logstream/server/db"
	"logstream/server/dedup"
	"logstream/server/depstate"
	"logstream/server/gateway"
	grpcserver "logstream/server/grpc"
	"logstream/server/hub"
	pb "logstream/server/pb"
	"logstream/server/store"
)

func main() {
	// ── 1. Load configuration ──────────────────────────────────────────────────
	cfg := config.Load()
	log.Printf("server: starting (grpc=%s http=%s logsDir=%s dataDir=%s)",
		cfg.GRPCPort, cfg.HTTPPort, cfg.LogsDir, cfg.DataDir)

	// ── 2. Open SQLite database ────────────────────────────────────────────────
	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("server: open db: %v", err)
	}
	defer database.Close()

	// ── 3. Create store, hub, dedup, and dependency state ──────────────────────
	logStore := store.New(cfg.LogsDir)
	h := hub.New()
	ds := depstate.New()

	dd := dedup.New(cfg.RedisURL, cfg.RedisDB)
	defer dd.Close()

	// ── 4. Start cleaner goroutine ─────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cl := cleaner.New(cfg, database, logStore)
	go cl.Start(ctx)

	// ── 5. Create gRPC server ──────────────────────────────────────────────────
	var grpcOpts []grpc.ServerOption
	if cfg.TLSEnabled {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			log.Fatalf("server: load TLS certs: %v", err)
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	grpcSrv := grpc.NewServer(grpcOpts...)

	// ── 6. Register LogService ─────────────────────────────────────────────────
	logSvc := grpcserver.New(cfg, database, logStore, h, dd, ds)
	pb.RegisterLogServiceServer(grpcSrv, logSvc)

	// ── 7. Start gRPC listener ─────────────────────────────────────────────────
	grpcLis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("server: grpc listen %s: %v", cfg.GRPCPort, err)
	}
	go func() {
		log.Printf("server: gRPC listening on %s", cfg.GRPCPort)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Printf("server: gRPC serve error: %v", err)
		}
	}()

	// ── 8. Build HTTP mux ──────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// REST API (includes /api/auth/login public + protected routes).
	restAPI := api.New(cfg, database, logStore, dd, ds)
	restAPI.RegisterRoutes(mux, cfg.JWTSecret)

	// WebSocket live-stream endpoint.
	mux.Handle("/ws/logs", gateway.Handler(h, cfg.JWTSecret))

	httpSrv := &http.Server{
		Addr:         cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 0 = no timeout (needed for WebSocket / SSE)
		IdleTimeout:  120 * time.Second,
	}

	// ── 9. Start HTTP server ───────────────────────────────────────────────────
	go func() {
		log.Printf("server: HTTP listening on %s", cfg.HTTPPort)
		var err error
		if cfg.TLSEnabled {
			err = httpSrv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = httpSrv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Printf("server: HTTP serve error: %v", err)
		}
	}()

	// ── 10. Graceful shutdown on SIGTERM / SIGINT ──────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	log.Printf("server: received signal %s — shutting down", sig)

	// Cancel context (stops cleaner).
	cancel()

	// Stop accepting new gRPC requests and wait for in-flight ones to finish.
	grpcSrv.GracefulStop()
	log.Println("server: gRPC stopped")

	// Shut down HTTP with a deadline.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Printf("server: HTTP shutdown error: %v", err)
	}
	log.Println("server: HTTP stopped")
	log.Println("server: exiting")
}
