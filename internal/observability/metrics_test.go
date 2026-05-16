package observability

import "testing"

func TestCoreMetrics(t *testing.T) {
	t.Parallel()

	metrics := CoreMetrics()
	if len(metrics) < 10 {
		t.Fatalf("len(CoreMetrics()) = %d, want at least 10", len(metrics))
	}
	seen := map[string]struct{}{}
	for _, metric := range metrics {
		if metric.Name == "" || metric.Type == "" || metric.Description == "" {
			t.Fatalf("metric missing required fields: %+v", metric)
		}
		if _, ok := seen[metric.Name]; ok {
			t.Fatalf("duplicate metric name %q", metric.Name)
		}
		seen[metric.Name] = struct{}{}
	}
	if _, ok := seen["meil_drift_events_total"]; !ok {
		t.Fatalf("missing drift event metric")
	}
	if _, ok := seen["meil_dmarc_reports_ingested_total"]; !ok {
		t.Fatalf("missing DMARC ingestion metric")
	}
}
