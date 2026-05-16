package reporter

import (
	"net"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/ingest"
)

func TestApplyFilterByDomainProviderResultAndDate(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	rows := []RowObservation{
		{
			Domain:     "example.com",
			Provider:   "Google Workspace",
			ObservedAt: base.Add(time.Hour),
			Record: ingest.AggregateRecord{
				SourceIP:          net.ParseIP("203.0.113.10"),
				Count:             10,
				DKIMAlignedResult: "pass",
				SPFAlignedResult:  "fail",
				Disposition:       "none",
			},
		},
		{
			Domain:     "example.com",
			Provider:   "Unknown",
			ObservedAt: base.Add(2 * time.Hour),
			Record: ingest.AggregateRecord{
				SourceIP:          net.ParseIP("203.0.113.20"),
				Count:             5,
				DKIMAlignedResult: "fail",
				SPFAlignedResult:  "fail",
				Disposition:       "reject",
			},
		},
		{
			Domain:     "other.com",
			Provider:   "Google Workspace",
			ObservedAt: base.Add(3 * time.Hour),
			Record:     ingest.AggregateRecord{SourceIP: net.ParseIP("203.0.113.30"), DKIMAlignedResult: "pass"},
		},
	}

	filtered := ApplyFilter(rows, Filter{
		Domain:   "EXAMPLE.COM",
		Provider: "unknown",
		Result:   "fail",
		From:     base,
		To:       base.Add(3 * time.Hour),
	})

	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1: %+v", len(filtered), filtered)
	}
	if filtered[0].Record.SourceIP.String() != "203.0.113.20" {
		t.Fatalf("unexpected row: %+v", filtered[0])
	}
}

func TestApplyFilterBySourceAndDisposition(t *testing.T) {
	t.Parallel()

	rows := []RowObservation{
		{Record: ingest.AggregateRecord{SourceIP: net.ParseIP("203.0.113.10"), Disposition: "none"}},
		{Record: ingest.AggregateRecord{SourceIP: net.ParseIP("203.0.113.20"), Disposition: "quarantine"}},
	}

	filtered := ApplyFilter(rows, Filter{SourceIP: net.ParseIP("203.0.113.20"), Result: "quarantine"})
	if len(filtered) != 1 || filtered[0].Record.Disposition != "quarantine" {
		t.Fatalf("filtered = %+v, want quarantine row", filtered)
	}
}
