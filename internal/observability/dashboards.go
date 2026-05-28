package observability

type Panel struct {
	Title  string
	Metric string
	Query  string
}

type Dashboard struct {
	Name   string
	Panels []Panel
}

func CoreDashboards() []Dashboard {
	return []Dashboard{
		{
			Name: "api_scan",
			Panels: []Panel{
				{Title: "Request rate", Metric: "meil_http_requests_total", Query: "sum by (route, status) (rate(meil_http_requests_total{route=\"/api/v1/scan\"}[5m]))"},
				{Title: "Request latency", Metric: "meil_http_request_duration_seconds", Query: "histogram_quantile(0.95, sum by (le) (rate(meil_http_request_duration_seconds{route=\"/api/v1/scan\"}[5m])))"},
			},
		},
		{
			Name: "workers",
			Panels: []Panel{
				{Title: "Queued scan jobs", Metric: "meil_scan_jobs_queued", Query: "sum by (status) (meil_scan_jobs_queued)"},
				{Title: "Scan attempts", Metric: "meil_scan_job_attempts_total", Query: "sum by (result) (rate(meil_scan_job_attempts_total[5m]))"},
				{Title: "Notification dispatches", Metric: "meil_notification_dispatch_total", Query: "sum by (channel, result) (rate(meil_notification_dispatch_total[5m]))"},
			},
		},
		{
			Name: "reporter",
			Panels: []Panel{
				{Title: "Reports ingested", Metric: "meil_dmarc_reports_ingested_total", Query: "sum by (org_name) (rate(meil_dmarc_reports_ingested_total[1h]))"},
				{Title: "Parse failures", Metric: "meil_dmarc_report_parse_failures_total", Query: "sum by (reason) (rate(meil_dmarc_report_parse_failures_total[15m]))"},
				{Title: "Query latency", Metric: "meil_clickhouse_query_duration_seconds", Query: "histogram_quantile(0.95, sum by (le, query) (rate(meil_clickhouse_query_duration_seconds[5m])))"},
				{Title: "Reporter freshness", Metric: "meil_reporter_freshness_seconds", Query: "max by (domain_id) (meil_reporter_freshness_seconds)"},
			},
		},
		{
			Name: "dashboard_api",
			Panels: []Panel{
				{Title: "Dashboard API failures", Metric: "meil_dashboard_api_failures_total", Query: "sum by (route, status) (rate(meil_dashboard_api_failures_total[5m]))"},
				{Title: "Dashboard rechecks", Metric: "meil_dashboard_rechecks_total", Query: "sum by (result) (rate(meil_dashboard_rechecks_total[15m]))"},
				{Title: "Alert actions", Metric: "meil_dashboard_alert_actions_total", Query: "sum by (action) (rate(meil_dashboard_alert_actions_total[15m]))"},
			},
		},
	}
}

type AlertRule struct {
	Name        string
	Expression  string
	Severity    string
	Description string
}

func CoreAlertRules() []AlertRule {
	return []AlertRule{
		{Name: "api_scan_high_error_rate", Severity: "page", Expression: `sum(rate(meil_http_requests_total{route="/api/v1/scan",status=~"5.."}[5m])) > 0.05`, Description: "API scan endpoint is returning elevated server errors."},
		{Name: "scan_jobs_backlogged", Severity: "ticket", Expression: `sum(meil_scan_jobs_queued{status="queued"}) > 100`, Description: "Scan job queue backlog is above the operating threshold."},
		{Name: "notification_dispatch_failures", Severity: "ticket", Expression: `sum(rate(meil_notification_dispatch_total{result="failed"}[15m])) > 0`, Description: "Notification dispatches are failing."},
		{Name: "dmarc_parse_failures", Severity: "ticket", Expression: `sum(rate(meil_dmarc_report_parse_failures_total[15m])) > 0`, Description: "DMARC ingestion parser failures are occurring."},
		{Name: "dashboard_api_failures", Severity: "ticket", Expression: `sum(rate(meil_dashboard_api_failures_total[5m])) > 0.05`, Description: "Dashboard API failure rate is above the operating threshold."},
	}
}
