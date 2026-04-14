package grpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"logstream/server/config"
	"logstream/server/db"
	"logstream/server/dedup"
	"logstream/server/depstate"
	"logstream/server/hub"
	pb "logstream/server/pb"
	"logstream/server/store"
)

// Server implements pb.LogServiceServer.
type Server struct {
	pb.UnimplementedLogServiceServer
	cfg      *config.Config
	db       *db.DB
	store    *store.Store
	hub      *hub.Hub
	dedup    *dedup.Deduplicator
	depState *depstate.Store
}

// New creates a new gRPC Server.
func New(cfg *config.Config, database *db.DB, st *store.Store, h *hub.Hub, dd *dedup.Deduplicator, ds *depstate.Store) *Server {
	return &Server{
		cfg:      cfg,
		db:       database,
		store:    st,
		hub:      h,
		dedup:    dd,
		depState: ds,
	}
}

// Register upserts the service in the database and returns its current config.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.Agent == nil {
		return nil, status.Error(codes.InvalidArgument, "agent info is required")
	}
	agent := req.Agent
	if agent.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id is required")
	}

	name := agent.Name
	if name == "" {
		name = agent.ServiceId
	}

	if err := s.db.UpsertService(agent.ServiceId, name, agent.AgentId, agent.Version); err != nil {
		log.Printf("grpc: Register: upsert service %s: %v", agent.ServiceId, err)
		return nil, status.Errorf(codes.Internal, "failed to register service: %v", err)
	}

	cfg, err := s.db.GetConfig(agent.ServiceId)
	if err != nil {
		log.Printf("grpc: Register: get config %s: %v", agent.ServiceId, err)
		return nil, status.Errorf(codes.Internal, "failed to get config: %v", err)
	}

	log.Printf("grpc: agent %s registered for service %s (version %s)", agent.AgentId, agent.ServiceId, agent.Version)

	return &pb.RegisterResponse{
		Config:     cfg,
		ServerTime: time.Now().UnixMilli(),
	}, nil
}

// StreamLogs receives batches of log entries from agents via a bidirectional stream.
// For each batch it persists each entry and fans out to dashboard subscribers.
func (s *Server) StreamLogs(stream pb.LogService_StreamLogsServer) error {
	var serviceID string

	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if serviceID != "" {
				_ = s.db.SetServiceStatus(serviceID, "offline")
			}
			return err
		}

		if serviceID == "" {
			serviceID = batch.ServiceId
		}

		accepted := int32(0)
		for _, entry := range batch.Entries {
			if entry == nil {
				continue
			}
			// Fill in service/agent IDs if missing from the entry itself.
			if entry.ServiceId == "" {
				entry.ServiceId = batch.ServiceId
			}
			if entry.AgentId == "" {
				entry.AgentId = batch.AgentId
			}
			// Generate a stable ID so the dashboard can deduplicate entries.
			if entry.Id == "" {
				entry.Id = uuid.NewString()
			}
			// Convert Timestamp (ms) to UnixTs (seconds) for the frontend.
			if entry.UnixTs == 0 && entry.Timestamp > 0 {
				entry.UnixTs = entry.Timestamp / 1000
			}
			// Promote Extra fields to dedicated struct fields.
			if entry.Extra != nil {
				if v, ok := entry.Extra["log_file"]; ok && entry.LogFile == "" {
					entry.LogFile = v
				}
				if v, ok := entry.Extra["worker_type"]; ok && entry.WorkerType == "" {
					entry.WorkerType = v
				}
				if v, ok := entry.Extra["queue"]; ok && entry.Queue == "" {
					entry.Queue = v
				}
				if v, ok := entry.Extra["service"]; ok && entry.ServiceName == "" {
					entry.ServiceName = v
				}
			}

			// Check for duplicates on replayed entries
			if s.dedup != nil && s.dedup.IsDuplicate(stream.Context(), entry) {
				continue // skip this entry
			}

			// Emit restart event if this is the first replayed entry
			if entry.Extra != nil && entry.Extra["replayed"] == "true" {
				if s.dedup != nil && s.dedup.ShouldNotifyRestart(stream.Context(), entry.ServiceId) {
					s.hub.PublishEvent(map[string]interface{}{
						"type":                  "agent_restart",
						"service_id":            entry.ServiceId,
						"restart_detected_at":   time.Now().Format(time.RFC3339),
						"replay_window_minutes": 5,
					})
					log.Printf("grpc: restart detected for service %s — replayed entries arriving", entry.ServiceId)
				}
			}

			// Strip the replayed flag before persisting
			if entry.Extra != nil {
				delete(entry.Extra, "replayed")
			}

			if err := s.store.Write(entry); err != nil {
				log.Printf("grpc: StreamLogs: write entry: %v", err)
				continue
			}
			s.hub.Publish(entry)
			accepted++
		}

		// Send acknowledgement.
		resp := &pb.StreamResponse{
			BatchId:  batch.BatchId,
			Accepted: accepted,
		}

		// Optionally piggyback updated config.
		if cfg, err := s.db.GetConfig(batch.ServiceId); err == nil {
			resp.UpdatedConfig = cfg
		}

		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("grpc: StreamLogs: send ack: %w", err)
		}
	}

	if serviceID != "" {
		_ = s.db.SetServiceStatus(serviceID, "offline")
		log.Printf("grpc: stream closed for service %s", serviceID)
	}
	return nil
}

// Heartbeat updates agent statistics and returns the current service config.
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id is required")
	}

	// Keep last_seen fresh.
	if err := s.db.SetServiceStatus(req.ServiceId, "online"); err != nil {
		log.Printf("grpc: Heartbeat: set status: %v", err)
	}

	if err := s.db.UpdateAgentStats(req.AgentId, req.ServiceId, req.BufferUsed, req.DroppedTotal, req.LogsPerSec); err != nil {
		log.Printf("grpc: Heartbeat: update stats: %v", err)
	}

	// Store healthmon dependency states if present.
	if len(req.Dependencies) > 0 && s.depState != nil {
		s.depState.Update(req.ServiceId, req.Dependencies)
	}

	cfg, err := s.db.GetConfig(req.ServiceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get config: %v", err)
	}

	return &pb.HeartbeatResponse{
		Config:     cfg,
		ServerTime: time.Now().UnixMilli(),
	}, nil
}

// Subscribe streams live log entries to a client (e.g. dashboard) until the context is cancelled.
func (s *Server) Subscribe(req *pb.SubscribeRequest, stream pb.LogService_SubscribeServer) error {
	filter := hub.Filter{
		ServiceIDs: req.ServiceIds,
		Levels:     req.Levels,
		TaskID:     req.TaskId,
		Documento:  req.Documento,
		Module:     req.Module,
		Search:     req.Search,
	}

	sub := s.hub.Subscribe(filter)
	defer s.hub.Unsubscribe(sub.ID)

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entry, ok := <-sub.Ch:
			if !ok {
				return nil
			}
			if err := stream.Send(entry); err != nil {
				return fmt.Errorf("grpc: Subscribe: send: %w", err)
			}
		}
	}
}

// Query performs a historical log search via the file store.
func (s *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if req.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id is required")
	}

	entries, total, err := s.store.Query(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query: %v", err)
	}

	return &pb.QueryResponse{
		Entries: entries,
		Total:   int32(total),
	}, nil
}

// UpdateConfig persists an updated ServiceConfig and returns it.
func (s *Server) UpdateConfig(ctx context.Context, req *pb.UpdateConfigRequest) (*pb.UpdateConfigResponse, error) {
	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	if req.Config.ServiceId == "" {
		return nil, status.Error(codes.InvalidArgument, "config.service_id is required")
	}

	if err := s.db.UpsertConfig(req.Config); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert config: %v", err)
	}

	updated, err := s.db.GetConfig(req.Config.ServiceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get config: %v", err)
	}

	log.Printf("grpc: UpdateConfig: service %s ttl=%d minLevel=%s enabled=%v",
		updated.ServiceId, updated.TtlDays, updated.MinLevel.String(), updated.Enabled)

	return &pb.UpdateConfigResponse{
		Config:  updated,
		Success: true,
	}, nil
}
