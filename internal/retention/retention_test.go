package retention

import (
	"testing"
	"time"
)

func TestPolicies(t *testing.T) {
	t.Parallel()

	if FreePolicy().RawReportDays != 0 {
		t.Fatalf("free policy should not retain raw reports")
	}
	if ReporterPolicy().AggregateRowDays <= ReporterPolicy().RawReportDays {
		t.Fatalf("reporter aggregate rows should outlive raw reports")
	}
	if BusinessPolicy().AuditLogDays <= ReporterPolicy().AuditLogDays {
		t.Fatalf("business audit retention should exceed reporter retention")
	}
}

func TestShouldDelete(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	if !ShouldDelete(now.AddDate(0, 0, -91), now, 90) {
		t.Fatalf("old record should be deleted")
	}
	if ShouldDelete(now.AddDate(0, 0, -10), now, 90) {
		t.Fatalf("recent record should be retained")
	}
	if !ShouldDelete(now, now, 0) {
		t.Fatalf("zero-day retention should delete immediately")
	}
}
