package observability

import "testing"

func TestCoreTraceSpans(t *testing.T) {
	t.Parallel()

	spans := CoreTraceSpans()
	if len(spans) < 5 {
		t.Fatalf("len(CoreTraceSpans()) = %d, want at least 5", len(spans))
	}
	seen := map[string]struct{}{}
	for _, span := range spans {
		if span.Name == "" || span.Description == "" || len(span.Attributes) == 0 {
			t.Fatalf("invalid span: %+v", span)
		}
		if _, ok := seen[span.Name]; ok {
			t.Fatalf("duplicate span name %q", span.Name)
		}
		seen[span.Name] = struct{}{}
	}
	for _, required := range []string{"http.request", "scanner.scan", "worker.job", "ingest.dmarc_report", "reporter.query"} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("missing required trace span %q", required)
		}
	}
}
