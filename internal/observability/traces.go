package observability

type TraceSpan struct {
	Name        string
	Description string
	Attributes  []string
}

func CoreTraceSpans() []TraceSpan {
	return []TraceSpan{
		{
			Name:        "http.request",
			Description: "Inbound HTTP request handling for public, app, and API routes.",
			Attributes:  []string{"route", "method", "status", "request_id", "organization_id"},
		},
		{
			Name:        "scanner.scan",
			Description: "End-to-end email-auth scanner execution for one domain.",
			Attributes:  []string{"domain", "result_status", "duration_ms", "resolver_error_class"},
		},
		{
			Name:        "worker.job",
			Description: "Background River job execution.",
			Attributes:  []string{"job_kind", "queue", "attempt", "result", "domain_id"},
		},
		{
			Name:        "ingest.dmarc_report",
			Description: "DMARC aggregate report parsing, deduplication, and aggregate persistence.",
			Attributes:  []string{"org_name", "report_id", "source_file", "deduped", "records"},
		},
		{
			Name:        "reporter.query",
			Description: "Reporter dashboard and source inventory query execution.",
			Attributes:  []string{"domain_id", "query", "date_range", "row_count"},
		},
	}
}
