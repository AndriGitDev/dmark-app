package privacy

import "testing"

func TestNewExportManifest(t *testing.T) {
	t.Parallel()

	manifest := NewExportManifest("user-1", "user@example.com", true)
	if manifest.UserID != "user-1" || manifest.Email != "user@example.com" || !manifest.IncludesRawXML {
		t.Fatalf("unexpected manifest metadata: %+v", manifest)
	}
	if len(manifest.Sections) < 8 {
		t.Fatalf("expected raw XML section in manifest: %+v", manifest.Sections)
	}
}

func TestNewDeletionPlan(t *testing.T) {
	t.Parallel()

	plan := NewDeletionPlan("user-1")
	if plan.UserID != "user-1" || len(plan.Steps) == 0 {
		t.Fatalf("unexpected deletion plan: %+v", plan)
	}
	if !plan.Steps[0].Destructive {
		t.Fatalf("first deletion step should be destructive: %+v", plan.Steps[0])
	}
	if plan.Steps[len(plan.Steps)-1].Destructive {
		t.Fatalf("audit tombstone step should not be destructive: %+v", plan.Steps[len(plan.Steps)-1])
	}
}

func TestValidateRetentionRequest(t *testing.T) {
	t.Parallel()

	limits := RetentionLimits{MaxRawReportDays: 90, MaxAggregateRowDays: 365, MaxNotificationLogDays: 180}
	request, err := ValidateRetentionRequest(RetentionRequest{RawReportDays: 30, AggregateRowDays: 365, NotificationLogDays: 30}, limits)
	if err != nil {
		t.Fatalf("ValidateRetentionRequest returned error: %v", err)
	}
	if request.RawReportDays != 30 {
		t.Fatalf("request = %+v, want retained values", request)
	}
	if _, err := ValidateRetentionRequest(RetentionRequest{RawReportDays: 100, AggregateRowDays: 365}, limits); err == nil {
		t.Fatalf("expected plan-limit error")
	}
	if _, err := ValidateRetentionRequest(RetentionRequest{RawReportDays: 90, AggregateRowDays: 30}, limits); err == nil {
		t.Fatalf("expected raw-greater-than-aggregate error")
	}
}
