package reporter

import (
	"testing"
	"time"
)

func TestEvaluateHealthHealthy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	result := EvaluateHealth(HealthCheck{
		Now:          now,
		ExpectedOrgs: []string{"Google", "Microsoft"},
		Receipts: []ReportReceipt{
			{OrgName: "Google", ReceivedAt: now.Add(-time.Hour)},
			{OrgName: "Microsoft", ReceivedAt: now.Add(-2 * time.Hour)},
		},
	})
	if result.Status != "healthy" || len(result.MissingOrgs) != 0 || len(result.ReporterOrgs) != 2 {
		t.Fatalf("unexpected healthy result: %+v", result)
	}
}

func TestEvaluateHealthMissingReports(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	result := EvaluateHealth(HealthCheck{
		Now:          now,
		ExpectedOrgs: []string{"Google", "Microsoft"},
		Receipts:     []ReportReceipt{{OrgName: "Google", ReceivedAt: now.Add(-time.Hour)}},
	})
	if result.Status != "missing_reports" || len(result.MissingOrgs) != 1 || result.MissingOrgs[0] != "Microsoft" {
		t.Fatalf("unexpected missing result: %+v", result)
	}
}

func TestEvaluateHealthIngestionFailures(t *testing.T) {
	t.Parallel()

	result := EvaluateHealth(HealthCheck{
		Now:               time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		FailureAlertAfter: 2,
		Receipts: []ReportReceipt{
			{OrgName: "Google", Failed: true, Error: "bad xml"},
			{OrgName: "Microsoft", Failed: true, Error: "bad zip"},
		},
	})
	if result.Status != "ingestion_failures" || result.FailureCount != 2 {
		t.Fatalf("unexpected failure result: %+v", result)
	}
}

func TestEvaluateHealthNoReports(t *testing.T) {
	t.Parallel()

	result := EvaluateHealth(HealthCheck{Now: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)})
	if result.Status != "no_reports" {
		t.Fatalf("status = %q, want no_reports", result.Status)
	}
}
