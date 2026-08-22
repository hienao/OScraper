package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDevJWTSecret        = "dev-jwt-secret-change-before-release-000000"
	defaultDevCredentialKey    = "0123456789abcdef0123456789abcdef"
	placeholderJWTSecret       = "replace-with-at-least-32-random-characters"
	placeholderCredentialKey   = "00000000000000000000000000000000"
	defaultServerPort          = "8080"
	defaultAccessTokenLifetime = 24
)

type Config struct {
	AppEnv                  string
	GinMode                 string
	ServerPort              string
	AppDataDir              string
	AppCacheDir             string
	SQLitePath              string
	LocalMediaRoot          string
	JWTSecret               string
	AccessTokenHours        int
	CredentialEncryptionKey string
	APILogPath              string
	APILogQueueSize         int
	APILogBatchSize         int
	HTTPTimeoutSeconds      int
	ScrapeWorkers           int
	ScrapeQueueSize         int
	ScanWorkers             int
	ScanQueueSize           int
	JobWorkDir              string
	MaxImageBytes           int64
	JobRetentionDays        int
	LogRetentionDays        int
	DataRetentionDays       int
}

func Load() *Config {
	appEnv := strings.ToLower(getEnv("APP_ENV", "development"))
	dataDir := getEnv("APP_DATA_DIR", "./runtime/data")
	cacheDir := getEnv("APP_CACHE_DIR", "./runtime/cache")
	ginMode := getEnv("GIN_MODE", "debug")
	if appEnv == "production" {
		ginMode = "release"
	}

	return &Config{
		AppEnv:                  appEnv,
		GinMode:                 ginMode,
		ServerPort:              getEnv("SERVER_PORT", defaultServerPort),
		AppDataDir:              dataDir,
		AppCacheDir:             cacheDir,
		SQLitePath:              getEnv("SQLITE_PATH", filepath.Join(dataDir, "db", "openlist-scraper.db")),
		LocalMediaRoot:          getEnv("LOCAL_MEDIA_ROOT", "/media"),
		JWTSecret:               getEnv("JWT_SECRET", defaultDevJWTSecret),
		AccessTokenHours:        getEnvInt("ACCESS_TOKEN_HOURS", defaultAccessTokenLifetime),
		CredentialEncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", defaultDevCredentialKey),
		APILogPath:              getEnv("API_LOG_PATH", filepath.Join(cacheDir, "logs", "app", "api-logs.db")),
		APILogQueueSize:         getEnvInt("API_LOG_QUEUE_SIZE", 5000),
		APILogBatchSize:         getEnvInt("API_LOG_BATCH_SIZE", 100),
		HTTPTimeoutSeconds:      getEnvInt("HTTP_TIMEOUT_SECONDS", 20),
		ScrapeWorkers:           getEnvInt("SCRAPE_WORKERS", 2),
		ScrapeQueueSize:         getEnvInt("SCRAPE_QUEUE_SIZE", 100),
		ScanWorkers:             getEnvInt("SCAN_WORKERS", 1),
		ScanQueueSize:           getEnvInt("SCAN_QUEUE_SIZE", 20),
		JobWorkDir:              getEnv("JOB_WORK_DIR", filepath.Join(dataDir, "work", "jobs")),
		MaxImageBytes:           int64(getEnvInt("MAX_IMAGE_BYTES", 20<<20)),
		JobRetentionDays:        getEnvInt("JOB_RETENTION_DAYS", 7),
		LogRetentionDays:        getEnvInt("LOG_RETENTION_DAYS", 7),
		DataRetentionDays:       getEnvInt("DATA_RETENTION_DAYS", 30),
	}
}

func (c *Config) Production() bool { return c.AppEnv == "production" }

func (c *Config) Validate() error {
	if c.Production() {
		if len(c.JWTSecret) < 32 || c.JWTSecret == defaultDevJWTSecret || c.JWTSecret == placeholderJWTSecret {
			return &ValidationError{Message: "JWT_SECRET must be at least 32 characters and cannot use the development default"}
		}
		if c.CredentialEncryptionKey == defaultDevCredentialKey || c.CredentialEncryptionKey == placeholderCredentialKey {
			return &ValidationError{Message: "CREDENTIAL_ENCRYPTION_KEY cannot use the development default"}
		}
	}
	if c.AccessTokenHours < 1 || c.AccessTokenHours > 168 {
		return &ValidationError{Message: "ACCESS_TOKEN_HOURS must be between 1 and 168"}
	}
	if c.ScrapeWorkers < 1 || c.ScrapeWorkers > 4 {
		return &ValidationError{Message: "SCRAPE_WORKERS must be between 1 and 4"}
	}
	if c.ScrapeQueueSize < 1 || c.ScrapeQueueSize > 10000 {
		return &ValidationError{Message: "SCRAPE_QUEUE_SIZE must be between 1 and 10000"}
	}
	if c.ScanWorkers < 1 || c.ScanWorkers > 4 {
		return &ValidationError{Message: "SCAN_WORKERS must be between 1 and 4"}
	}
	if c.ScanQueueSize < 1 || c.ScanQueueSize > 1000 {
		return &ValidationError{Message: "SCAN_QUEUE_SIZE must be between 1 and 1000"}
	}
	if c.MaxImageBytes < 1<<20 || c.MaxImageBytes > 100<<20 {
		return &ValidationError{Message: "MAX_IMAGE_BYTES must be between 1 MiB and 100 MiB"}
	}
	if c.JobRetentionDays < 1 || c.JobRetentionDays > 30 {
		return &ValidationError{Message: "JOB_RETENTION_DAYS must be between 1 and 30"}
	}
	if c.LogRetentionDays < 1 || c.LogRetentionDays > 30 {
		return &ValidationError{Message: "LOG_RETENTION_DAYS must be between 1 and 30"}
	}
	if c.DataRetentionDays < 1 || c.DataRetentionDays > 365 {
		return &ValidationError{Message: "DATA_RETENTION_DAYS must be between 1 and 365"}
	}
	return nil
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
