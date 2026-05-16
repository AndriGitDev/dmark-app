package domains

import (
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestBuildDashboardStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	items := BuildDashboard([]Domain{
		{ID: "empty", Domain: "empty.example.com"},
		{
			ID:     "healthy",
			Domain: "healthy.example.com",
			LatestCheck: &Check{
				CheckedAt: now.Add(-time.Hour),
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
			},
		},
		{
			ID:     "failed",
			Domain: "failed.example.com",
			LatestCheck: &Check{
				CheckedAt: now.Add(-time.Hour),
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusFail, Message: "SPF failed"},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
			},
		},
		{
			ID:     "stale",
			Domain: "stale.example.com",
			LatestCheck: &Check{
				CheckedAt: now.Add(-25 * time.Hour),
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
			},
		},
	}, DashboardOptions{
		Now:           now,
		Cadence:       24 * time.Hour,
		InProgressIDs: map[string]struct{}{"empty": {}},
	})

	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}
	byID := map[string]DashboardDomain{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if byID["empty"].State != ScanStateInProgress {
		t.Fatalf("empty state = %s, want in_progress", byID["empty"].State)
	}
	if byID["healthy"].State != ScanStateHealthy || byID["healthy"].NextCheckAt.IsZero() {
		t.Fatalf("healthy item = %+v, want healthy with next check", byID["healthy"])
	}
	if byID["failed"].State != ScanStateFailed || byID["failed"].TopIssue == "" {
		t.Fatalf("failed item = %+v, want failed with issue", byID["failed"])
	}
	if byID["stale"].State != ScanStatePending {
		t.Fatalf("stale state = %s, want pending", byID["stale"].State)
	}
}

func TestBuildDashboardSortsActionableStatesFirst(t *testing.T) {
	t.Parallel()

	items := BuildDashboard([]Domain{
		{ID: "healthy", Domain: "healthy.example.com", LatestCheck: &Check{SPF: scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass}, DKIM: scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass}, DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass}, BIMI: scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass}}},
		{ID: "empty", Domain: "empty.example.com"},
	}, DashboardOptions{FailedScanIDs: map[string]struct{}{"empty": {}}})

	if items[0].ID != "empty" || items[0].State != ScanStateFailed {
		t.Fatalf("items = %+v, want failed item first", items)
	}
}
