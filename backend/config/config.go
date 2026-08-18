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
	DBDriver                string
	DatabaseURL             string
	SQLitePath              string
	JWTSecret               string
	AccessTokenHours        int
	CredentialEncryptionKey string
	APILogPath              string
	APILogQueueSize         int
	APILogBatchSize         int
	HTTPTimeoutSeconds      int
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
		DBDriver:                strings.ToLower(getEnv("DB_DRIVER", "sqlite")),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		SQLitePath:              getEnv("SQLITE_PATH", filepath.Join(dataDir, "db", "openlist-scraper.db")),
		JWTSecret:               getEnv("JWT_SECRET", defaultDevJWTSecret),
		AccessTokenHours:        getEnvInt("ACCESS_TOKEN_HOURS", defaultAccessTokenLifetime),
		CredentialEncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", defaultDevCredentialKey),
		APILogPath:              getEnv("API_LOG_PATH", filepath.Join(cacheDir, "logs", "app", "api-logs.db")),
		APILogQueueSize:         getEnvInt("API_LOG_QUEUE_SIZE", 5000),
		APILogBatchSize:         getEnvInt("API_LOG_BATCH_SIZE", 100),
		HTTPTimeoutSeconds:      getEnvInt("HTTP_TIMEOUT_SECONDS", 20),
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
