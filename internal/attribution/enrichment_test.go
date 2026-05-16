package attribution

import (
	"context"
	"testing"
	"time"
)

func TestServiceRefreshBuildsCachedClassification(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	service := NewService(StaticEnricher{
		"203.0.113.10": {
			IP:          "203.0.113.10",
			ReverseDNS:  "mail-sor-f41.google.com.",
			ASN:         "AS15169",
			CountryCode: "us",
		},
	}, DefaultClassifier(), CachePolicy{TTL: time.Hour})

	entry, err := service.Refresh(context.Background(), "203.0.113.10", "_spf.google.com", now)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if entry.ProviderHint != "Google Workspace" || entry.Confidence != ConfidenceHigh || entry.CountryCode != "US" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !entry.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %s, want %s", entry.ExpiresAt, now.Add(time.Hour))
	}
}

func TestServiceRefreshRequiresEnricherAndIP(t *testing.T) {
	t.Parallel()

	service := NewService(nil, DefaultClassifier(), CachePolicy{})
	if _, err := service.Refresh(context.Background(), "203.0.113.10", "", time.Now()); err == nil {
		t.Fatalf("Refresh returned nil error without enricher")
	}
	service = NewService(StaticEnricher{}, DefaultClassifier(), CachePolicy{})
	if _, err := service.Refresh(context.Background(), "", "", time.Now()); err == nil {
		t.Fatalf("Refresh returned nil error without IP")
	}
}
