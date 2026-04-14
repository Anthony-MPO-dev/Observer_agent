package sender

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	stdjson "encoding/json"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpcencoding "google.golang.org/grpc/encoding"

	pb "logstream/agent/pb"
	"logstream/agent/buffer"
	"logstream/agent/config"
	"logstream/agent/offset"
)

func init() {
	// Register JSON codec so all gRPC messages are serialized as JSON.
	// This matches the server-side codec registration.
	grpcencoding.RegisterCodec(jsonCodec{})
}

// jsonCodec implements grpc/encoding.Codec using standard encoding/json.
type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return stdjson.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return stdjson.Unmarshal(data, v)
}

// batchOffsets holds the extracted offset info for a pending batch.
type batchOffsets struct {
	offsets map[string]int64 // file_path → max byte offset
}

// DependencyProvider is an interface that returns the current state of monitored
// external services. Implemented by healthmon.Monitor.
type DependencyProvider interface {
	DependencyStatuses() []*pb.DependencyStatus
}

// Sender manages the gRPC connection to the log server, batches log entries
// and sends them, falling back to the in-memory ring buffer when offline.
type Sender struct {
	cfg     *config.Config
	buf     *buffer.RingBuffer
	agentID string
	offsets *offset.Store
	deps    DependencyProvider // optional — nil if healthmon not configured

	// queue for entries waiting to be sent
	queue chan *pb.LogEntry

	// live config updated by server responses
	cfgMu     sync.RWMutex
	batchSize int
	flushMs   int

	// pending batch tracking for offset persistence
	pendingMu      sync.Mutex
	pendingBatches map[string]*batchOffsets

	// metrics (exported for /metrics endpoint)
	Connected    bool
	LogsPerSec   float32
	DroppedTotal int64
	BufferLen    int

	// internal counter for logs/sec calculation
	logsTicker int64
}

// New creates a Sender.
func New(cfg *config.Config, buf *buffer.RingBuffer, agentID string, offsets *offset.Store) *Sender {
	return &Sender{
		cfg:            cfg,
		buf:            buf,
		agentID:        agentID,
		offsets:        offsets,
		queue:          make(chan *pb.LogEntry, 8192),
		batchSize:      cfg.BatchSize,
		flushMs:        cfg.FlushMs,
		pendingBatches: make(map[string]*batchOffsets),
	}
}

// SetDependencyProvider sets the healthmon provider for heartbeat reporting.
func (s *Sender) SetDependencyProvider(dp DependencyProvider) {
	s.deps = dp
}

// Send queues an entry for sending (non-blocking).
// If the internal queue is full the entry goes directly to the ring buffer.
func (s *Sender) Send(entry *pb.LogEntry) {
	select {
	case s.queue <- entry:
	default:
		s.buf.Push(entry)
	}
}

// Start begins the send loop.  It blocks until ctx is done.
func (s *Sender) Start(ctx context.Context) {
	// metrics ticker: compute logs/sec every second
	go s.metricsLoop(ctx)

	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}
		s.Connected = false
		s.BufferLen = s.buf.Len()
		s.DroppedTotal = s.buf.DroppedCount

		conn, err := s.dial()
		if err != nil {
			log.Printf("[sender] dial error: %v — retrying (attempt %d)", err, attempt)
			s.backoffSleep(ctx, attempt)
			attempt++
			continue
		}

		client := pb.NewLogServiceClient(conn)

		// Register with server
		if err := s.register(ctx, client); err != nil {
			log.Printf("[sender] register error: %v — reconnecting", err)
			conn.Close()
			s.backoffSleep(ctx, attempt)
			attempt++
			continue
		}

		attempt = 0 // reset backoff on successful connection
		s.Connected = true
		log.Printf("[sender] connected and registered with %s", s.cfg.ServerAddr)

		// Start heartbeat goroutine
		hbCtx, hbCancel := context.WithCancel(ctx)
		go s.heartbeatLoop(hbCtx, client)

		// Run the streaming send loop
		if err := s.streamLoop(ctx, client); err != nil && ctx.Err() == nil {
			log.Printf("[sender] stream error: %v — reconnecting", err)
		}

		hbCancel()
		conn.Close()
		s.Connected = false

		if ctx.Err() != nil {
			return
		}
		attempt++
		s.backoffSleep(ctx, attempt)
	}
}

// dial creates a gRPC connection with optional TLS and JSON codec.
func (s *Sender) dial() (*grpc.ClientConn, error) {
	var dialOpts []grpc.DialOption

	// Force JSON codec on every call
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(
		grpc.ForceCodec(jsonCodec{}),
	))

	if s.cfg.TLSEnabled && s.cfg.TLSCertFile != "" {
		certPool := x509.NewCertPool()
		caCert, err := os.ReadFile(s.cfg.TLSCertFile)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(caCert)
		creds := credentials.NewTLS(&tls.Config{RootCAs: certPool})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	//nolint:staticcheck
	conn, err := grpc.Dial(s.cfg.ServerAddr, dialOpts...)
	return conn, err
}

// register sends agent info to the server and applies received config.
func (s *Sender) register(ctx context.Context, client pb.LogServiceClient) error {
	hostname, _ := os.Hostname()
	req := &pb.RegisterRequest{
		Agent: &pb.AgentInfo{
			AgentId:   s.agentID,
			ServiceId: s.cfg.ServiceID,
			Name:      s.cfg.ServiceName,
			Version:   s.cfg.Version,
			Hostname:  hostname,
		},
	}

	rCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.Register(rCtx, req)
	if err != nil {
		return err
	}

	if cfg := resp.GetConfig(); cfg != nil {
		s.applyConfig(cfg)
	}
	return nil
}

// streamLoop opens a bidirectional stream and sends batches until the stream breaks.
func (s *Sender) streamLoop(ctx context.Context, client pb.LogServiceClient) error {
	stream, err := client.StreamLogs(ctx)
	if err != nil {
		return err
	}

	// Drain any buffered entries in batchSize chunks to avoid exceeding gRPC message size limits.
	buffered := s.buf.DrainAll()
	if len(buffered) > 0 {
		log.Printf("[sender] draining %d buffered entries", len(buffered))
		s.cfgMu.RLock()
		bs := s.batchSize
		s.cfgMu.RUnlock()
		for i := 0; i < len(buffered); i += bs {
			end := i + bs
			if end > len(buffered) {
				end = len(buffered)
			}
			if err := s.sendBatch(stream, buffered[i:end]); err != nil {
				// Put unsent entries back into the buffer and abort.
				for _, e := range buffered[i:] {
					s.buf.Push(e)
				}
				return err
			}
		}
	}

	s.cfgMu.RLock()
	batchSize := s.batchSize
	flushMs := s.flushMs
	s.cfgMu.RUnlock()

	batch := make([]*pb.LogEntry, 0, batchSize)
	ticker := time.NewTicker(time.Duration(flushMs) * time.Millisecond)
	defer ticker.Stop()

	// Receive acks from server in background
	recvErrCh := make(chan error, 1)
	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				recvErrCh <- err
				return
			}
			if cfg := resp.GetUpdatedConfig(); cfg != nil {
				s.applyConfig(cfg)
				s.cfgMu.RLock()
				batchSize = s.batchSize
				s.cfgMu.RUnlock()
			}
			// Save offsets on successful ACK
			if resp.GetAccepted() > 0 {
				batchID := resp.GetBatchId()
				s.pendingMu.Lock()
				bo := s.pendingBatches[batchID]
				delete(s.pendingBatches, batchID)
				s.pendingMu.Unlock()
				if bo != nil {
					s.saveOffsets(ctx, bo)
				}
			}
		}
	}()

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		toSend := batch
		batch = make([]*pb.LogEntry, 0, batchSize)
		return s.sendBatch(stream, toSend)
	}

	for {
		select {
		case <-ctx.Done():
			_ = flush()
			return stream.CloseSend()

		case err := <-recvErrCh:
			if err == io.EOF {
				return nil
			}
			return err

		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}

		case entry, ok := <-s.queue:
			if !ok {
				return nil
			}
			atomic.AddInt64(&s.logsTicker, 1)
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
}

// sendBatch assembles a LogBatch and sends it on the stream.
func (s *Sender) sendBatch(stream pb.LogService_StreamLogsClient, entries []*pb.LogEntry) error {
	// Extract offset info before stripping internal fields
	bo := &batchOffsets{offsets: make(map[string]int64)}
	for _, e := range entries {
		if e.Extra != nil {
			fp := e.Extra["_file_path"]
			offStr := e.Extra["_byte_offset"]
			if fp != "" && offStr != "" {
				if off, err := strconv.ParseInt(offStr, 10, 64); err == nil {
					if off > bo.offsets[fp] {
						bo.offsets[fp] = off
					}
				}
			}
			// Strip internal tracking fields before sending to server
			delete(e.Extra, "_byte_offset")
			delete(e.Extra, "_file_path")
		}
	}

	batchID := uuid.NewString()
	batch := &pb.LogBatch{
		AgentId:   s.agentID,
		ServiceId: s.cfg.ServiceID,
		Entries:   entries,
		BatchId:   batchID,
		SentAt:    time.Now().UnixMilli(),
	}

	// Track pending batch for offset persistence on ACK
	if len(bo.offsets) > 0 {
		s.pendingMu.Lock()
		s.pendingBatches[batchID] = bo
		s.pendingMu.Unlock()
	}

	log.Printf("[sender] sending batch: %d entries", len(entries))
	if err := stream.Send(batch); err != nil {
		log.Printf("[sender] sendBatch error: %v", err)
		// Clean up pending batch on send failure
		s.pendingMu.Lock()
		delete(s.pendingBatches, batchID)
		s.pendingMu.Unlock()
		return err
	}
	return nil
}

// saveOffsets persists max byte offset per file to Redis after server ACK.
func (s *Sender) saveOffsets(ctx context.Context, bo *batchOffsets) {
	if s.offsets == nil || !s.offsets.Available() {
		return
	}
	for fp, off := range bo.offsets {
		inode, _ := offset.FileInode(fp)
		if err := s.offsets.Save(ctx, fp, off, inode); err != nil {
			log.Printf("[sender] offset save error for %s: %v", filepath.Base(fp), err)
		}
	}
}

// heartbeatLoop sends a heartbeat every 10 seconds until ctx is done.
func (s *Sender) heartbeatLoop(ctx context.Context, client pb.LogServiceClient) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req := &pb.HeartbeatRequest{
				AgentId:      s.agentID,
				ServiceId:    s.cfg.ServiceID,
				BufferUsed:   int64(s.buf.Len()),
				DroppedTotal: s.buf.DroppedCount,
				LogsPerSec:   s.LogsPerSec,
			}
			if s.deps != nil {
				req.Dependencies = s.deps.DependencyStatuses()
			}
			hCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			resp, err := client.Heartbeat(hCtx, req)
			cancel()
			if err != nil {
				log.Printf("[sender] heartbeat error: %v", err)
				continue
			}
			if cfg := resp.GetConfig(); cfg != nil {
				s.applyConfig(cfg)
			}
		}
	}
}

// metricsLoop updates LogsPerSec every second.
func (s *Sender) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := atomic.SwapInt64(&s.logsTicker, 0)
			s.LogsPerSec = float32(current)
			s.DroppedTotal = s.buf.DroppedCount
			s.BufferLen = s.buf.Len()
		}
	}
}

// applyConfig updates batch/flush settings from a server-supplied config.
func (s *Sender) applyConfig(cfg *pb.ServiceConfig) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if cfg.GetBatchSize() > 0 {
		s.batchSize = int(cfg.GetBatchSize())
	}
	if cfg.GetFlushMs() > 0 {
		s.flushMs = int(cfg.GetFlushMs())
	}
}

// backoffSleep sleeps for min(2^attempt, 60) seconds.
func (s *Sender) backoffSleep(ctx context.Context, attempt int) {
	secs := math.Min(math.Pow(2, float64(attempt)), 60)
	delay := time.Duration(secs) * time.Second
	select {
	case <-time.After(delay):
	case <-ctx.Done():
	}
}
