package observability

import "testing"

func TestCoreDashboards(t *testing.T) {
	t.Parallel()

	dashboards := CoreDashboards()
	if len(dashboards) < 3 {
		t.Fatalf("len(CoreDashboards()) = %d, want at least 3", len(dashboards))
	}
	for _, dashboard := range dashboards {
		if dashboard.Name == "" || len(dashboard.Panels) == 0 {
			t.Fatalf("invalid dashboard: %+v", dashboard)
		}
		for _, panel := range dashboard.Panels {
			if panel.Title == "" || panel.Metric == "" || panel.Query == "" {
				t.Fatalf("invalid panel: %+v", panel)
			}
		}
	}
}

func TestCoreAlertRules(t *testing.T) {
	t.Parallel()

	rules := CoreAlertRules()
	if len(rules) < 4 {
		t.Fatalf("len(CoreAlertRules()) = %d, want at least 4", len(rules))
	}
	for _, rule := range rules {
		if rule.Name == "" || rule.Expression == "" || rule.Severity == "" || rule.Description == "" {
			t.Fatalf("invalid alert rule: %+v", rule)
		}
	}
}
