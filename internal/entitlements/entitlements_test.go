package entitlements

import "testing"

func TestHasPermission(t *testing.T) {
	t.Parallel()

	if !HasPermission(RoleViewer, PermissionViewDomain) {
		t.Fatalf("viewer should view domains")
	}
	if HasPermission(RoleViewer, PermissionManageDomain) {
		t.Fatalf("viewer should not manage domains")
	}
}

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	if !limits.AllowsSavedDomain(249) || limits.AllowsSavedDomain(250) {
		t.Fatalf("saved-domain resource boundary is incorrect: %+v", limits)
	}
	if !limits.AllowsScanCadence(60) || limits.AllowsScanCadence(59) {
		t.Fatalf("scan cadence resource boundary is incorrect: %+v", limits)
	}
	if !limits.AllowsReportRetention(730) || limits.AllowsReportRetention(731) {
		t.Fatalf("retention resource boundary is incorrect: %+v", limits)
	}
	if !limits.AllowsAlertChannels(4) || limits.AllowsAlertChannels(5) {
		t.Fatalf("alert channel resource boundary is incorrect: %+v", limits)
	}
	if !limits.AllowsAPIAccess() || !limits.AllowsReporterAccess() || !limits.Teams {
		t.Fatalf("shared capabilities unavailable: %+v", limits)
	}
}
