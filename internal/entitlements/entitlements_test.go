package entitlements

import "testing"

func TestForPlan(t *testing.T) {
	t.Parallel()

	free := ForPlan(PlanFree)
	reporter := ForPlan(PlanReporter)
	business := ForPlan(PlanBusiness)

	if free.Reporter || free.Teams || !reporter.Reporter || !business.Teams {
		t.Fatalf("unexpected plan limits: free=%+v reporter=%+v business=%+v", free, reporter, business)
	}
	if !free.AllowsSavedDomain(2) || free.AllowsSavedDomain(3) {
		t.Fatalf("free saved-domain limit not enforced: %+v", free)
	}
	if !reporter.AllowsScanCadence(240) || reporter.AllowsScanCadence(60) {
		t.Fatalf("reporter scan cadence limit not enforced: %+v", reporter)
	}
	if free.AllowsReportRetention(1) || !reporter.AllowsReportRetention(365) || reporter.AllowsReportRetention(366) {
		t.Fatalf("report retention limits not enforced: free=%+v reporter=%+v", free, reporter)
	}
	if !reporter.AllowsAlertChannels(2) || reporter.AllowsAlertChannels(3) {
		t.Fatalf("alert channel limit not enforced: %+v", reporter)
	}
	if !reporter.AllowsReporterAccess() || !reporter.AllowsAPIAccess() || free.AllowsAPIAccess() {
		t.Fatalf("feature gates not enforced: free=%+v reporter=%+v", free, reporter)
	}
}

func TestHasPermission(t *testing.T) {
	t.Parallel()

	if !HasPermission(RoleOwner, PermissionManageBilling) {
		t.Fatalf("owner should manage billing")
	}
	if HasPermission(RoleAdmin, PermissionManageBilling) {
		t.Fatalf("admin should not manage billing")
	}
	if !HasPermission(RoleViewer, PermissionViewDomain) {
		t.Fatalf("viewer should view domains")
	}
	if HasPermission(RoleViewer, PermissionManageDomain) {
		t.Fatalf("viewer should not manage domains")
	}
}
