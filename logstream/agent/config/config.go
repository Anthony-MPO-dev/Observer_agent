package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all agent configuration loaded from environment variables.
type Config struct {
	ServerAddr  string // LOG_SERVER_ADDR, e.g. "log-server:50051"
	ServiceID   string // LOG_SERVICE_ID, e.g. "dados_basicos"
	ServiceName string // LOG_SERVICE_NAME
	LogVolume   string // LOG_VOLUME_PATH, e.g. "/app/logs"
	Version     string // LOG_VERSION, default "1.0.0"

	DefaultTTLDays int    // LOG_DEFAULT_TTL_DAYS, default 30
	MinLevel       string // LOG_MIN_LEVEL, default "INFO"
	BufferSize     int    // LOG_BUFFER_SIZE, default 50000
	BatchSize      int    // LOG_BATCH_SIZE, default 100
	FlushMs        int    // LOG_FLUSH_MS, default 500

	FilenamePrefix string // LOG_FILENAME_PREFIX, e.g. "dados_basicos" — used by parser to match log filenames

	TLSEnabled  bool   // LOG_TLS_ENABLED
	TLSCertFile string // LOG_TLS_CERT_FILE

	MetricsPort  string // LOG_METRICS_PORT, default ":9090"
	HealthmonPort string // HEALTHMON_PORT, default ":9091"

	RedisURL             string // REDIS_URL, default "redis://redis:6379"
	RedisDB              int    // REDIS_DB, default 3
	RedisKeyPrefix       string // REDIS_KEY_PREFIX, default "logagent"
	RestartWindowMinutes int    // AGENT_RESTART_WINDOW_MINUTES, default 5
	OffsetFlushMs        int    // AGENT_OFFSET_FLUSH_INTERVAL_MS, default 500
	ReadExisting         bool   // LOG_READ_EXISTING, default false — if true, reads log files from start on first run
}

// Load reads configuration from environment, optionally loading a .env file first.
func Load() *Config {
	// Try to load .env — ignore error if not present
	_ = godotenv.Load()

	return &Config{
		ServerAddr:  getEnv("LOG_SERVER_ADDR", "log-server:50051"),
		ServiceID:   getEnv("LOG_SERVICE_ID", "unknown"),
		ServiceName: getEnv("LOG_SERVICE_NAME", "unknown"),
		LogVolume:   getEnv("LOG_VOLUME_PATH", "/app/logs"),
		Version:     getEnv("LOG_VERSION", "1.0.0"),

		DefaultTTLDays: getEnvInt("LOG_DEFAULT_TTL_DAYS", 30),
		MinLevel:       getEnv("LOG_MIN_LEVEL", "INFO"),
		BufferSize:     getEnvInt("LOG_BUFFER_SIZE", 50000),
		BatchSize:      getEnvInt("LOG_BATCH_SIZE", 100),
		FlushMs:        getEnvInt("LOG_FLUSH_MS", 500),

		FilenamePrefix: getEnv("LOG_FILENAME_PREFIX", ""),

		TLSEnabled:  getEnvBool("LOG_TLS_ENABLED", false),
		TLSCertFile: getEnv("LOG_TLS_CERT_FILE", ""),

		MetricsPort:  getEnv("LOG_METRICS_PORT", ":9090"),
		HealthmonPort: getEnv("HEALTHMON_PORT", ":9091"),

		RedisURL:             getEnv("REDIS_URL", "redis://redis:6379"),
		RedisDB:              getEnvInt("REDIS_DB", 3),
		RedisKeyPrefix:       getEnv("REDIS_KEY_PREFIX", "logagent"),
		RestartWindowMinutes: getEnvInt("AGENT_RESTART_WINDOW_MINUTES", 5),
		OffsetFlushMs:        getEnvInt("AGENT_OFFSET_FLUSH_INTERVAL_MS", 500),
		ReadExisting:         getEnvBool("LOG_READ_EXISTING", false),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
