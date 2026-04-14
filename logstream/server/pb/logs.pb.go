// Manually written — DO NOT EDIT.
// Replaces protoc-generated file. Uses plain Go structs + JSON codec (no protoimpl).

package pb

import "encoding/json"

// LogLevel enum — values match the agent's pb package exactly so JSON integers decode correctly.
type LogLevel int32

const (
	LogLevel_DEBUG   LogLevel = 0
	LogLevel_INFO    LogLevel = 1
	LogLevel_WARNING LogLevel = 2
	LogLevel_ERROR   LogLevel = 3
	LogLevel_FATAL   LogLevel = 4
)

func (l LogLevel) String() string {
	switch l {
	case LogLevel_DEBUG:
		return "DEBUG"
	case LogLevel_INFO:
		return "INFO"
	case LogLevel_WARNING:
		return "WARNING"
	case LogLevel_ERROR:
		return "ERROR"
	case LogLevel_FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// MarshalJSON serializes LogLevel as a string (e.g. "INFO") so the dashboard can display it.
func (l LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

// UnmarshalJSON accepts both string ("INFO") and integer (1) representations.
func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*l = LogLevelFromString(s)
		return nil
	}
	var n int32
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*l = LogLevel(n)
	return nil
}

func LogLevelFromString(s string) LogLevel {
	switch s {
	case "DEBUG":
		return LogLevel_DEBUG
	case "WARNING":
		return LogLevel_WARNING
	case "ERROR":
		return LogLevel_ERROR
	case "FATAL", "CRITICAL":
		return LogLevel_FATAL
	default:
		return LogLevel_INFO
	}
}

// ── Messages ─────────────────────────────────────────────────────────────────

type LogEntry struct {
	Id             string            `json:"id,omitempty"`
	ServiceId      string            `json:"service_id,omitempty"`
	ServiceName    string            `json:"service_name,omitempty"`
	AgentId        string            `json:"agent_id,omitempty"`
	Level          LogLevel          `json:"level,omitempty"`
	Message        string            `json:"message,omitempty"`
	Timestamp      int64             `json:"timestamp,omitempty"`
	TaskId         string            `json:"task_id,omitempty"`
	Documento      string            `json:"documento,omitempty"`
	Module         string            `json:"module,omitempty"`
	WorkerType     string            `json:"worker_type,omitempty"`
	Queue          string            `json:"queue,omitempty"`
	LogFile        string            `json:"log_file,omitempty"`
	UnixTs         int64             `json:"unix_ts,omitempty"`
	TimestampStr   string            `json:"timestamp_str,omitempty"`
	IsContinuation bool              `json:"is_continuation,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
}

func (x *LogEntry) Reset()        {}
func (x *LogEntry) String() string { return x.Message }
func (x *LogEntry) ProtoMessage() {}

func (x *LogEntry) GetServiceId() string  { return x.ServiceId }
func (x *LogEntry) GetLevel() LogLevel    { return x.Level }
func (x *LogEntry) GetMessage() string    { return x.Message }
func (x *LogEntry) GetTimestamp() int64   { return x.Timestamp }
func (x *LogEntry) GetTaskId() string     { return x.TaskId }
func (x *LogEntry) GetDocumento() string  { return x.Documento }
func (x *LogEntry) GetModule() string     { return x.Module }
func (x *LogEntry) GetAgentId() string    { return x.AgentId }

type LogBatch struct {
	AgentId   string      `json:"agent_id,omitempty"`
	ServiceId string      `json:"service_id,omitempty"`
	Entries   []*LogEntry `json:"entries,omitempty"`
	BatchId   string      `json:"batch_id,omitempty"`
	SentAt    int64       `json:"sent_at,omitempty"`
}

func (x *LogBatch) Reset()        {}
func (x *LogBatch) String() string { return x.BatchId }
func (x *LogBatch) ProtoMessage() {}

func (x *LogBatch) GetAgentId() string    { return x.AgentId }
func (x *LogBatch) GetServiceId() string  { return x.ServiceId }
func (x *LogBatch) GetEntries() []*LogEntry { return x.Entries }
func (x *LogBatch) GetBatchId() string    { return x.BatchId }
func (x *LogBatch) GetSentAt() int64      { return x.SentAt }

type AgentInfo struct {
	AgentId   string `json:"agent_id,omitempty"`
	ServiceId string `json:"service_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

func (x *AgentInfo) Reset()        {}
func (x *AgentInfo) String() string { return x.Name }
func (x *AgentInfo) ProtoMessage() {}

func (x *AgentInfo) GetAgentId() string   { return x.AgentId }
func (x *AgentInfo) GetServiceId() string { return x.ServiceId }
func (x *AgentInfo) GetName() string      { return x.Name }
func (x *AgentInfo) GetVersion() string   { return x.Version }
func (x *AgentInfo) GetHostname() string  { return x.Hostname }

type RegisterRequest struct {
	Agent *AgentInfo `json:"agent,omitempty"`
}

func (x *RegisterRequest) Reset()        {}
func (x *RegisterRequest) String() string { return "" }
func (x *RegisterRequest) ProtoMessage() {}

func (x *RegisterRequest) GetAgent() *AgentInfo {
	if x != nil {
		return x.Agent
	}
	return nil
}

type RegisterResponse struct {
	Config     *ServiceConfig `json:"config,omitempty"`
	ServerTime int64          `json:"server_time,omitempty"`
}

func (x *RegisterResponse) Reset()        {}
func (x *RegisterResponse) String() string { return "" }
func (x *RegisterResponse) ProtoMessage() {}

func (x *RegisterResponse) GetConfig() *ServiceConfig {
	if x != nil {
		return x.Config
	}
	return nil
}
func (x *RegisterResponse) GetServerTime() int64 { return x.ServerTime }

type ServiceConfig struct {
	ServiceId string   `json:"service_id,omitempty"`
	TtlDays   int32    `json:"ttl_days,omitempty"`
	MinLevel  LogLevel `json:"min_level,omitempty"`
	BatchSize int32    `json:"batch_size,omitempty"`
	FlushMs   int32    `json:"flush_ms,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

func (x *ServiceConfig) Reset()        {}
func (x *ServiceConfig) String() string { return x.ServiceId }
func (x *ServiceConfig) ProtoMessage() {}

func (x *ServiceConfig) GetServiceId() string { return x.ServiceId }
func (x *ServiceConfig) GetTtlDays() int32    { return x.TtlDays }
func (x *ServiceConfig) GetMinLevel() LogLevel { return x.MinLevel }
func (x *ServiceConfig) GetBatchSize() int32   { return x.BatchSize }
func (x *ServiceConfig) GetFlushMs() int32     { return x.FlushMs }
func (x *ServiceConfig) GetEnabled() bool      { return x.Enabled }

// DependencyStatus represents the circuit breaker state for one external service.
type DependencyStatus struct {
	ServiceID     string   `json:"service_id"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`         // CLOSED, OPEN, HALF_OPEN
	ErrorRate     float64  `json:"error_rate"`
	TotalRequests int64    `json:"total_requests"`
	TotalErrors   int64    `json:"total_errors"`
	Essential     bool     `json:"essential"`
	Fallbacks     []string `json:"fallbacks,omitempty"`
	OpenedAt      *int64   `json:"opened_at,omitempty"`  // unix ms, nil if not open
	LastPingAt    *int64   `json:"last_ping_at,omitempty"`
	LastPingOK    bool     `json:"last_ping_ok"`
}

type HeartbeatRequest struct {
	AgentId      string              `json:"agent_id,omitempty"`
	ServiceId    string              `json:"service_id,omitempty"`
	BufferUsed   int64               `json:"buffer_used,omitempty"`
	DroppedTotal int64               `json:"dropped_total,omitempty"`
	LogsPerSec   float32             `json:"logs_per_sec,omitempty"`
	Dependencies []*DependencyStatus `json:"dependencies,omitempty"`
}

func (x *HeartbeatRequest) Reset()        {}
func (x *HeartbeatRequest) String() string { return "" }
func (x *HeartbeatRequest) ProtoMessage() {}

func (x *HeartbeatRequest) GetAgentId() string              { return x.AgentId }
func (x *HeartbeatRequest) GetServiceId() string            { return x.ServiceId }
func (x *HeartbeatRequest) GetBufferUsed() int64            { return x.BufferUsed }
func (x *HeartbeatRequest) GetDroppedTotal() int64          { return x.DroppedTotal }
func (x *HeartbeatRequest) GetLogsPerSec() float32          { return x.LogsPerSec }
func (x *HeartbeatRequest) GetDependencies() []*DependencyStatus {
	if x != nil {
		return x.Dependencies
	}
	return nil
}

type HeartbeatResponse struct {
	Config     *ServiceConfig `json:"config,omitempty"`
	ServerTime int64          `json:"server_time,omitempty"`
}

func (x *HeartbeatResponse) Reset()        {}
func (x *HeartbeatResponse) String() string { return "" }
func (x *HeartbeatResponse) ProtoMessage() {}

func (x *HeartbeatResponse) GetConfig() *ServiceConfig {
	if x != nil {
		return x.Config
	}
	return nil
}
func (x *HeartbeatResponse) GetServerTime() int64 { return x.ServerTime }

type StreamResponse struct {
	BatchId       string         `json:"batch_id,omitempty"`
	Accepted      int32          `json:"accepted,omitempty"`
	UpdatedConfig *ServiceConfig `json:"updated_config,omitempty"`
}

func (x *StreamResponse) Reset()        {}
func (x *StreamResponse) String() string { return "" }
func (x *StreamResponse) ProtoMessage() {}

func (x *StreamResponse) GetBatchId() string { return x.BatchId }
func (x *StreamResponse) GetAccepted() int32 { return x.Accepted }
func (x *StreamResponse) GetUpdatedConfig() *ServiceConfig {
	if x != nil {
		return x.UpdatedConfig
	}
	return nil
}

type SubscribeRequest struct {
	ServiceIds []string `json:"service_ids,omitempty"`
	Levels     []string `json:"levels,omitempty"`
	TaskId     string   `json:"task_id,omitempty"`
	Documento  string   `json:"documento,omitempty"`
	Module     string   `json:"module,omitempty"`
	Search     string   `json:"search,omitempty"`
}

func (x *SubscribeRequest) Reset()        {}
func (x *SubscribeRequest) String() string { return "" }
func (x *SubscribeRequest) ProtoMessage() {}

func (x *SubscribeRequest) GetServiceIds() []string { return x.ServiceIds }
func (x *SubscribeRequest) GetLevels() []string     { return x.Levels }
func (x *SubscribeRequest) GetTaskId() string       { return x.TaskId }
func (x *SubscribeRequest) GetDocumento() string    { return x.Documento }
func (x *SubscribeRequest) GetModule() string       { return x.Module }
func (x *SubscribeRequest) GetSearch() string       { return x.Search }

type QueryRequest struct {
	ServiceId string   `json:"service_id,omitempty"`
	Levels    []string `json:"levels,omitempty"`
	TaskId    string   `json:"task_id,omitempty"`
	Documento string   `json:"documento,omitempty"`
	Module    string   `json:"module,omitempty"`
	Search    string   `json:"search,omitempty"`
	FromTs    int64    `json:"from_ts,omitempty"`
	ToTs      int64    `json:"to_ts,omitempty"`
	Limit     int32    `json:"limit,omitempty"`
	Offset    int32    `json:"offset,omitempty"`
}

func (x *QueryRequest) Reset()        {}
func (x *QueryRequest) String() string { return "" }
func (x *QueryRequest) ProtoMessage() {}

func (x *QueryRequest) GetServiceId() string { return x.ServiceId }
func (x *QueryRequest) GetLevels() []string  { return x.Levels }
func (x *QueryRequest) GetTaskId() string    { return x.TaskId }
func (x *QueryRequest) GetDocumento() string { return x.Documento }
func (x *QueryRequest) GetModule() string    { return x.Module }
func (x *QueryRequest) GetSearch() string    { return x.Search }
func (x *QueryRequest) GetFromTs() int64     { return x.FromTs }
func (x *QueryRequest) GetToTs() int64       { return x.ToTs }
func (x *QueryRequest) GetLimit() int32      { return x.Limit }
func (x *QueryRequest) GetOffset() int32     { return x.Offset }

type QueryResponse struct {
	Entries []*LogEntry `json:"entries,omitempty"`
	Total   int32       `json:"total,omitempty"`
}

func (x *QueryResponse) Reset()        {}
func (x *QueryResponse) String() string { return "" }
func (x *QueryResponse) ProtoMessage() {}

func (x *QueryResponse) GetEntries() []*LogEntry { return x.Entries }
func (x *QueryResponse) GetTotal() int32         { return x.Total }

type UpdateConfigRequest struct {
	Config *ServiceConfig `json:"config,omitempty"`
}

func (x *UpdateConfigRequest) Reset()        {}
func (x *UpdateConfigRequest) String() string { return "" }
func (x *UpdateConfigRequest) ProtoMessage() {}

func (x *UpdateConfigRequest) GetConfig() *ServiceConfig {
	if x != nil {
		return x.Config
	}
	return nil
}

type UpdateConfigResponse struct {
	Config  *ServiceConfig `json:"config,omitempty"`
	Success bool           `json:"success,omitempty"`
}

func (x *UpdateConfigResponse) Reset()        {}
func (x *UpdateConfigResponse) String() string { return "" }
func (x *UpdateConfigResponse) ProtoMessage() {}

func (x *UpdateConfigResponse) GetConfig() *ServiceConfig {
	if x != nil {
		return x.Config
	}
	return nil
}
func (x *UpdateConfigResponse) GetSuccess() bool { return x.Success }
