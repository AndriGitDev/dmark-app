package attribution

import (
	"testing"
	"time"
)

func TestNewEnrichmentEntryCopiesEvidenceAndSetsExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	result := Result{Provider: "Google Workspace", Confidence: ConfidenceHigh, Evidence: []string{"ASN matches AS15169"}}
	entry := NewEnrichmentEntry(
		Signal{IP: "203.0.113.10", ReverseDNS: "mail.google.com", ASN: "AS15169"},
		result,
		"US",
		now,
		CachePolicy{TTL: time.Hour},
	)
	result.Evidence[0] = "mutated"

	if entry.ProviderHint != "Google Workspace" || entry.Confidence != ConfidenceHigh || entry.CountryCode != "US" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !entry.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %s, want %s", entry.ExpiresAt, now.Add(time.Hour))
	}
	if entry.Evidence[0] != "ASN matches AS15169" {
		t.Fatalf("evidence was not copied: %+v", entry.Evidence)
	}
}

func TestRefreshDueSortsByExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	entries := []EnrichmentEntry{
		{IP: "203.0.113.30", RefreshedAt: now.Add(-time.Hour), ExpiresAt: now.Add(48 * time.Hour)},
		{IP: "203.0.113.20", RefreshedAt: now.Add(-time.Hour), ExpiresAt: now.Add(30 * time.Minute)},
		{IP: "203.0.113.10", RefreshedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)},
	}

	due := RefreshDue(entries, now, CachePolicy{TTL: 48 * time.Hour, RefreshBefore: time.Hour})
	if len(due) != 2 {
		t.Fatalf("len(due) = %d, want 2: %+v", len(due), due)
	}
	if due[0].IP != "203.0.113.10" || due[1].IP != "203.0.113.20" {
		t.Fatalf("due entries not sorted by expiry: %+v", due)
	}
}
