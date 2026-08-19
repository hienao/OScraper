package config

import "testing"

func TestProductionRejectsPlaceholderSecrets(t *testing.T) {
	cfg := &Config{
		AppEnv:                  "production",
		JWTSecret:               placeholderJWTSecret,
		CredentialEncryptionKey: placeholderCredentialKey,
		AccessTokenHours:        24,
		ScrapeWorkers:           2, ScrapeQueueSize: 100, MaxImageBytes: 20 << 20, JobRetentionDays: 7, LogRetentionDays: 7,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production placeholder secrets to be rejected")
	}
}

func TestProductionAcceptsConfiguredSecrets(t *testing.T) {
	cfg := &Config{
		AppEnv:                  "production",
		JWTSecret:               "a-strong-jwt-secret-with-more-than-32-characters",
		CredentialEncryptionKey: "abcdef0123456789abcdef0123456789",
		AccessTokenHours:        24,
		ScrapeWorkers:           2, ScrapeQueueSize: 100, MaxImageBytes: 20 << 20, JobRetentionDays: 7, LogRetentionDays: 7,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
