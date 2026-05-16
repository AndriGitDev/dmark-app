package reporter

import (
	"testing"
	"time"
)

func TestBuildDashboardRowsAggregatesSourceEvidence(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	rows := []AggregateRow{
		{
			SourceIP:          "203.0.113.10",
			HeaderFrom:        "example.com",
			Provider:          "Example ESP",
			Confidence:        "high",
			SourceOrg:         "Example Network",
			SourceASN:         "AS64500",
			SourceCountry:     "IS",
			Disposition:       "none",
			DKIMAlignedResult: "pass",
			SPFAlignedResult:  "fail",
			Count:             40,
			BeginAt:           first,
			EndAt:             first,
		},
		{
			SourceIP:          "203.0.113.10",
			HeaderFrom:        "example.com",
			Provider:          "Example ESP",
			Disposition:       "reject",
			DKIMAlignedResult: "fail",
			SPFAlignedResult:  "fail",
			Count:             10,
			BeginAt:           last,
			EndAt:             last,
		},
		{
			SourceIP: "203.0.113.20",
			Count:    100,
			BeginAt:  first,
			EndAt:    first,
		},
	}

	dashboardRows := BuildDashboardRows(rows)
	if len(dashboardRows) != 2 {
		t.Fatalf("len(dashboardRows) = %d, want 2", len(dashboardRows))
	}
	if dashboardRows[0].SourceIP != "203.0.113.20" || dashboardRows[0].TotalCount != 100 {
		t.Fatalf("rows not sorted by volume: %+v", dashboardRows)
	}
	item := dashboardRows[1]
	if item.PassCount != 40 || item.FailCount != 10 || item.Disposition["reject"] != 10 {
		t.Fatalf("unexpected pass/fail disposition counts: %+v", item)
	}
	if item.SourceASN != "AS64500" || item.SourceCountry != "IS" || !item.FirstSeen.Equal(first) || !item.LastSeen.Equal(last) {
		t.Fatalf("source metadata/date range not preserved: %+v", item)
	}
}
