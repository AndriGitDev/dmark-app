package ch

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/ingest"
)

func TestRowsFromReport(t *testing.T) {
	t.Parallel()

	report := ingest.AggregateReport{
		ReportID:  "report-1",
		OrgName:   "Mailbox",
		Domain:    "example.com",
		BeginUnix: 1778889600,
		EndUnix:   1778975999,
		Records: []ingest.AggregateRecord{{
			SourceIP:          net.ParseIP("203.0.113.10"),
			Count:             42,
			Disposition:       "none",
			DKIMAlignedResult: "pass",
			SPFAlignedResult:  "fail",
			HeaderFrom:        "example.com",
		}},
	}
	rows := RowsFromReport(report, ReportContext{
		SourceOrg:     "Example ASN",
		SourceASN:     "AS64500",
		SourceCountry: "IS",
		Attribution: map[string]attribution.Result{
			"203.0.113.10": {Provider: "Example ESP", Confidence: attribution.ConfidenceHigh},
		},
	})

	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Domain != "example.com" || row.ReportID != "report-1" || row.Provider != "Example ESP" || row.Count != 42 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if row.BeginAt.IsZero() || row.EndAt.IsZero() {
		t.Fatalf("expected report date range: %+v", row)
	}
}

func TestDateRangeQuerySQL(t *testing.T) {
	t.Parallel()

	sql, args := (DateRangeQuery{
		Domain: "example.com",
		From:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		To:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Limit:  50,
	}).SQL()
	if !strings.Contains(sql, "FROM dmarc_aggregate_rows") || !strings.Contains(sql, "LIMIT ?") {
		t.Fatalf("unexpected SQL: %s", sql)
	}
	if len(args) != 4 || args[0] != "example.com" || args[3] != 50 {
		t.Fatalf("unexpected args: %#v", args)
	}
}
