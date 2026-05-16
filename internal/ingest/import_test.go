package ingest

import "testing"

func TestReportKeyNormalizesMetadata(t *testing.T) {
	t.Parallel()

	key := Key(AggregateReport{
		OrgName:     " Example Mailbox ",
		ReportID:    " report-1 ",
		Domain:      " EXAMPLE.COM ",
		ContentHash: "abc123",
	})

	if key.OrgName != "example mailbox" || key.ReportID != "report-1" || key.Domain != "example.com" || key.ContentHash != "abc123" {
		t.Fatalf("key = %+v, want normalized metadata", key)
	}
}

func TestDeduplicateFlagsExistingAndBatchDuplicates(t *testing.T) {
	t.Parallel()

	report := AggregateReport{OrgName: "Mailbox", ReportID: "report-1", Domain: "example.com", ContentHash: "hash-1"}
	existing := map[string]struct{}{Key(report).String(): {}}
	decisions := Deduplicate([]AggregateReport{
		report,
		{OrgName: "Mailbox", ReportID: "report-2", Domain: "example.com", ContentHash: "hash-2"},
		{OrgName: "Mailbox", ReportID: "report-2", Domain: "example.com", ContentHash: "hash-2"},
	}, existing)

	if len(decisions) != 3 {
		t.Fatalf("len(decisions) = %d, want 3", len(decisions))
	}
	if !decisions[0].Duplicate || decisions[0].Reason != "already imported" {
		t.Fatalf("first decision = %+v, want existing duplicate", decisions[0])
	}
	if decisions[1].Duplicate {
		t.Fatalf("second decision = %+v, want import", decisions[1])
	}
	if !decisions[2].Duplicate || decisions[2].Reason != "duplicate in batch" {
		t.Fatalf("third decision = %+v, want batch duplicate", decisions[2])
	}
}
