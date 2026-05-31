package config

import (
	"testing"
)

func testPostgresURL() string {
	return "postgres://meil:" + "meil" + "@localhost:5432/meil?sslmode=disable"
}

func testBunnyAPIKey() string {
	return "test-" + "key"
}

func TestLoadSPFRouteBunnyDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", testPostgresURL())
	t.Setenv("PUBLIC_BASE_URL", "https://dmark.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.SPFRouteZone != "route.dmark.is" {
		t.Fatalf("SPFRouteZone = %q, want route.dmark.is", cfg.SPFRouteZone)
	}
	if cfg.SPFDNSZone != "dmark.is" {
		t.Fatalf("SPFDNSZone = %q, want dmark.is", cfg.SPFDNSZone)
	}
	if cfg.SPFRouteTTL != 300 {
		t.Fatalf("SPFRouteTTL = %d, want 300", cfg.SPFRouteTTL)
	}
}

func TestValidateSPFRouteBunnyConfig(t *testing.T) {
	cfg := Config{
		HTTPAddr:              ":8080",
		PublicBaseURL:         "https://dmark.example",
		DatabaseURL:           testPostgresURL(),
		ClickHouseURL:         "http://localhost:8123",
		ClickHouseDatabase:    "meil",
		AuthTokenTTL:          15,
		PublicChecksPerMinute: 12,
		DMARCMaxReportBytes:   1024,
		BunnyDNSAPIKey:        testBunnyAPIKey(),
		BunnyDNSZoneID:        801471,
		SPFDNSZone:            "dmark.is",
		SPFRouteZone:          "route.dmark.is",
		SPFRouteTTL:           300,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateSPFRouteBunnyConfigAcceptsLegacyBunnyAPIKey(t *testing.T) {
	cfg := Config{
		HTTPAddr:              ":8080",
		PublicBaseURL:         "https://dmark.example",
		DatabaseURL:           testPostgresURL(),
		ClickHouseURL:         "http://localhost:8123",
		ClickHouseDatabase:    "meil",
		AuthTokenTTL:          15,
		PublicChecksPerMinute: 12,
		DMARCMaxReportBytes:   1024,
		BunnyAPIKey:           testBunnyAPIKey(),
		BunnyDNSZoneID:        801471,
		SPFDNSZone:            "dmark.is",
		SPFRouteZone:          "route.dmark.is",
		SPFRouteTTL:           300,
	}
	if got := cfg.EffectiveBunnyDNSAPIKey(); got != testBunnyAPIKey() {
		t.Fatalf("EffectiveBunnyDNSAPIKey = %q", got)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateSPFRouteBunnyConfigRequiresCompleteCredentialSet(t *testing.T) {
	cfg := Config{
		HTTPAddr:              ":8080",
		PublicBaseURL:         "https://dmark.example",
		DatabaseURL:           testPostgresURL(),
		ClickHouseURL:         "http://localhost:8123",
		ClickHouseDatabase:    "meil",
		AuthTokenTTL:          15,
		PublicChecksPerMinute: 12,
		DMARCMaxReportBytes:   1024,
		BunnyDNSAPIKey:        testBunnyAPIKey(),
		SPFRouteZone:          "route.dmark.is",
		SPFRouteTTL:           300,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted Bunny DNS API key without zone ID")
	}
}
