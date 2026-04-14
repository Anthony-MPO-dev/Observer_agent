// Escrito manualmente — NÃO EDITAR via protoc.
// Usa structs Go puras com codec JSON (sem protoimpl).
// O JSON é suficiente porque o codec JSON está registrado no sender.go.

package pb

import "encoding/json"

// LogLevel define o nível de severidade de um log.
// Os valores inteiros são iguais aos do servidor para que a serialização JSON round-trip funcione.
type LogLevel int32

const (
	LogLevel_DEBUG   LogLevel = 0
	LogLevel_INFO    LogLevel = 1
	LogLevel_WARNING LogLevel = 2
	LogLevel_ERROR   LogLevel = 3
	LogLevel_FATAL   LogLevel = 4
)

// String converte o nível para texto legível.
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

// MarshalJSON serializes LogLevel as a string so the server/dashboard can read it.
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

// LogLevelFromString converte uma string de nível para o enum LogLevel.
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

// ── Mensagens ─────────────────────────────────────────────────────────────────

// LogEntry representa uma única linha de log parseada.
type LogEntry struct {
	ServiceId string            `json:"service_id,omitempty"`
	Level     LogLevel          `json:"level,omitempty"`
	Message   string            `json:"message,omitempty"`
	Timestamp int64             `json:"timestamp,omitempty"`
	TaskId    string            `json:"task_id,omitempty"`
	Documento string            `json:"documento,omitempty"`
	Module    string            `json:"module,omitempty"`
	AgentId   string            `json:"agent_id,omitempty"`
	TraceId   string            `json:"trace_id,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

func (x *LogEntry) Reset()        {}
func (x *LogEntry) String() string { return x.Message }
func (x *LogEntry) ProtoMessage() {}

func (x *LogEntry) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}
func (x *LogEntry) GetLevel() LogLevel {
	if x != nil {
		return x.Level
	}
	return LogLevel_INFO
}
func (x *LogEntry) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}
func (x *LogEntry) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}
func (x *LogEntry) GetTaskId() string {
	if x != nil {
		return x.TaskId
	}
	return ""
}
func (x *LogEntry) GetDocumento() string {
	if x != nil {
		return x.Documento
	}
	return ""
}
func (x *LogEntry) GetModule() string {
	if x != nil {
		return x.Module
	}
	return ""
}
func (x *LogEntry) GetAgentId() string {
	if x != nil {
		return x.AgentId
	}
	return ""
}
func (x *LogEntry) GetTraceId() string {
	if x != nil {
		return x.TraceId
	}
	return ""
}
func (x *LogEntry) GetExtra() map[string]string {
	if x != nil {
		return x.Extra
	}
	return nil
}

// LogBatch é um pacote de múltiplas entradas enviadas de uma vez ao servidor.
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

func (x *LogBatch) GetAgentId() string {
	if x != nil {
		return x.AgentId
	}
	return ""
}
func (x *LogBatch) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}
func (x *LogBatch) GetEntries() []*LogEntry {
	if x != nil {
		return x.Entries
	}
	return nil
}
func (x *LogBatch) GetBatchId() string {
	if x != nil {
		return x.BatchId
	}
	return ""
}
func (x *LogBatch) GetSentAt() int64 {
	if x != nil {
		return x.SentAt
	}
	return 0
}

// AgentInfo contém metadados do agente enviados durante o registro.
type AgentInfo struct {
	AgentId   string `json:"agent_id,omitempty"`
	ServiceId string `json:"service_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

func (x *AgentInfo) Reset()        {}
func (x *AgentInfo) String() string {
	if x != nil {
		return x.Name
	}
	return ""
}
func (x *AgentInfo) ProtoMessage() {}

func (x *AgentInfo) GetAgentId() string {
	if x != nil {
		return x.AgentId
	}
	return ""
}
func (x *AgentInfo) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}
func (x *AgentInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}
func (x *AgentInfo) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}
func (x *AgentInfo) GetHostname() string {
	if x != nil {
		return x.Hostname
	}
	return ""
}

// RegisterRequest é enviado pelo agente ao se conectar ao servidor.
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

// RegisterResponse é retornado pelo servidor após o registro, com configuração inicial.
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
func (x *RegisterResponse) GetServerTime() int64 {
	if x != nil {
		return x.ServerTime
	}
	return 0
}

// ServiceConfig contém configurações por serviço, definidas via dashboard.
type ServiceConfig struct {
	ServiceId string   `json:"service_id,omitempty"`
	TtlDays   int32    `json:"ttl_days,omitempty"`
	MinLevel  LogLevel `json:"min_level,omitempty"`
	BatchSize int32    `json:"batch_size,omitempty"`
	FlushMs   int32    `json:"flush_ms,omitempty"`
	Enabled   bool     `json:"enabled,omitempty"`
}

func (x *ServiceConfig) Reset()        {}
func (x *ServiceConfig) String() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}
func (x *ServiceConfig) ProtoMessage() {}

func (x *ServiceConfig) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}
func (x *ServiceConfig) GetTtlDays() int32 {
	if x != nil {
		return x.TtlDays
	}
	return 30
}
func (x *ServiceConfig) GetMinLevel() LogLevel {
	if x != nil {
		return x.MinLevel
	}
	return LogLevel_INFO
}
func (x *ServiceConfig) GetBatchSize() int32 {
	if x != nil {
		return x.BatchSize
	}
	return 100
}
func (x *ServiceConfig) GetFlushMs() int32 {
	if x != nil {
		return x.FlushMs
	}
	return 500
}
func (x *ServiceConfig) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return true
}

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

// HeartbeatRequest carrega estatísticas de runtime do agente.
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

// HeartbeatResponse retorna a configuração atualizada do servidor para o agente.
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
func (x *HeartbeatResponse) GetServerTime() int64 {
	if x != nil {
		return x.ServerTime
	}
	return 0
}

// StreamRequest é a mensagem inicial enviada pelo agente para abrir o stream bidirecional.
type StreamRequest struct {
	AgentId   string `json:"agent_id,omitempty"`
	ServiceId string `json:"service_id,omitempty"`
}

func (x *StreamRequest) Reset()        {}
func (x *StreamRequest) String() string { return "" }
func (x *StreamRequest) ProtoMessage() {}

func (x *StreamRequest) GetAgentId() string {
	if x != nil {
		return x.AgentId
	}
	return ""
}
func (x *StreamRequest) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}

// StreamResponse é enviado pelo servidor após cada batch recebido (confirmação).
type StreamResponse struct {
	BatchId       string         `json:"batch_id,omitempty"`
	Accepted      int32          `json:"accepted,omitempty"`
	UpdatedConfig *ServiceConfig `json:"updated_config,omitempty"`
}

func (x *StreamResponse) Reset()        {}
func (x *StreamResponse) String() string { return "" }
func (x *StreamResponse) ProtoMessage() {}

func (x *StreamResponse) GetBatchId() string { return x.BatchId }
func (x *StreamResponse) GetAccepted() int32  { return x.Accepted }
func (x *StreamResponse) GetUpdatedConfig() *ServiceConfig {
	if x != nil {
		return x.UpdatedConfig
	}
	return nil
}
