package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all server configuration values.
type Config struct {
	GRPCPort    string
	HTTPPort    string
	LogsDir     string
	DataDir     string

	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	JWTSecret string
	AdminUser string
	AdminPass string

	DefaultTTLDays int
	CleanupHour    int

	RedisURL string // REDIS_URL for dedup
	RedisDB  int    // REDIS_DB for dedup, default 4
}

// Load reads configuration from environment variables (and optional .env file).
func Load() *Config {
	// Best-effort load of .env; ignore error if file doesn't exist.
	_ = godotenv.Load()

	cfg := &Config{
		GRPCPort:       getEnv("GRPC_PORT", ":50051"),
		HTTPPort:       getEnv("HTTP_PORT", ":8080"),
		LogsDir:        getEnv("LOGS_DIR", "/logs"),
		DataDir:        getEnv("DATA_DIR", "/data"),
		TLSEnabled:     getBoolEnv("TLS_ENABLED", false),
		TLSCertFile:    getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:     getEnv("TLS_KEY_FILE", ""),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-to-a-random-secret-32chars"),
		AdminUser:      getEnv("ADMIN_USER", "admin"),
		AdminPass:      getEnv("ADMIN_PASS", "changeme"),
		DefaultTTLDays: getIntEnv("DEFAULT_TTL_DAYS", 30),
		CleanupHour:    getIntEnv("CLEANUP_HOUR", 0),
		RedisURL:       getEnv("REDIS_URL", ""),
		RedisDB:        getIntEnv("REDIS_DB", 4),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

func getIntEnv(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return i
}
