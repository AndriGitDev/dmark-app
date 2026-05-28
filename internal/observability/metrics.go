package observability

type MetricType string

const (
	Counter   MetricType = "counter"
	Histogram MetricType = "histogram"
	Gauge     MetricType = "gauge"
)

type Metric struct {
	Name        string
	Type        MetricType
	Description string
	Labels      []string
}

func CoreMetrics() []Metric {
	return []Metric{
		{Name: "meil_http_requests_total", Type: Counter, Description: "HTTP requests by route, method, and status.", Labels: []string{"route", "method", "status"}},
		{Name: "meil_http_request_duration_seconds", Type: Histogram, Description: "HTTP request latency by route and method.", Labels: []string{"route", "method"}},
		{Name: "meil_dns_lookup_duration_seconds", Type: Histogram, Description: "DNS TXT lookup latency by check type and resolver error class.", Labels: []string{"check", "error_class"}},
		{Name: "meil_scan_jobs_queued", Type: Gauge, Description: "Queued scan jobs ready for workers.", Labels: []string{"status"}},
		{Name: "meil_scan_job_attempts_total", Type: Counter, Description: "Scan job attempts by result.", Labels: []string{"result"}},
		{Name: "meil_drift_events_total", Type: Counter, Description: "Drift events by check, kind, and severity.", Labels: []string{"check", "kind", "severity"}},
		{Name: "meil_notification_dispatch_total", Type: Counter, Description: "Notification dispatches by channel and result.", Labels: []string{"channel", "result"}},
		{Name: "meil_dmarc_reports_ingested_total", Type: Counter, Description: "DMARC aggregate reports ingested by organization.", Labels: []string{"org_name"}},
		{Name: "meil_dmarc_report_parse_failures_total", Type: Counter, Description: "DMARC aggregate report parser failures by reason.", Labels: []string{"reason"}},
		{Name: "meil_clickhouse_query_duration_seconds", Type: Histogram, Description: "Reporter query latency by query type.", Labels: []string{"query"}},
	}
}
