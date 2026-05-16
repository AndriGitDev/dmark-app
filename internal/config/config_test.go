package config

import "testing"

func TestValidateAcceptsDefaultLikeConfig(t *testing.T) {
	t.Parallel()

	// #nosec G101 -- test-only local Postgres DSN.
	cfg := Config{
		HTTPAddr:              ":8080",
		PublicBaseURL:         "http://localhost:8080",
		DatabaseURL:           "postgres://meil:meil@localhost:5432/meil?sslmode=disable",
		ClickHouseURL:         "http://localhost:8123",
		ClickHouseDatabase:    "meil",
		AuthTokenTTL:          15,
		PublicChecksPerMinute: 12,
		DMARCMaxReportBytes:   10 << 20,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	// #nosec G101 -- test-only local Postgres DSN.
	cfg := Config{
		HTTPAddr:              ":8080",
		PublicBaseURL:         "localhost:8080",
		DatabaseURL:           "postgres://meil:meil@localhost:5432/meil?sslmode=disable",
		ClickHouseURL:         "http://localhost:8123",
		ClickHouseDatabase:    "meil",
		AuthTokenTTL:          15,
		PublicChecksPerMinute: 12,
		DMARCMaxReportBytes:   10 << 20,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate returned nil error")
	}
}
