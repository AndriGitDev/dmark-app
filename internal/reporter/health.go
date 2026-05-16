package reporter

import (
	"sort"
	"time"
)

type ReportReceipt struct {
	OrgName    string
	ReceivedAt time.Time
	Failed     bool
	Error      string
}

type HealthCheck struct {
	Now               time.Time
	ExpectedOrgs      []string
	Receipts          []ReportReceipt
	MissingAfter      time.Duration
	FailureAlertAfter int
}

type HealthResult struct {
	LastReportAt time.Time
	ReporterOrgs []string
	MissingOrgs  []string
	FailureCount int
	Status       string
	Message      string
}

func EvaluateHealth(check HealthCheck) HealthResult {
	if check.MissingAfter <= 0 {
		check.MissingAfter = 48 * time.Hour
	}
	if check.FailureAlertAfter <= 0 {
		check.FailureAlertAfter = 1
	}
	lastByOrg := map[string]time.Time{}
	failures := 0
	var last time.Time
	for _, receipt := range check.Receipts {
		if receipt.Failed {
			failures++
			continue
		}
		if receipt.ReceivedAt.After(last) {
			last = receipt.ReceivedAt
		}
		if receipt.ReceivedAt.After(lastByOrg[receipt.OrgName]) {
			lastByOrg[receipt.OrgName] = receipt.ReceivedAt
		}
	}
	orgs := make([]string, 0, len(lastByOrg))
	for org := range lastByOrg {
		orgs = append(orgs, org)
	}
	sort.Strings(orgs)
	missing := []string{}
	for _, org := range check.ExpectedOrgs {
		receivedAt := lastByOrg[org]
		if receivedAt.IsZero() || check.Now.Sub(receivedAt) > check.MissingAfter {
			missing = append(missing, org)
		}
	}
	sort.Strings(missing)
	result := HealthResult{
		LastReportAt: last,
		ReporterOrgs: orgs,
		MissingOrgs:  missing,
		FailureCount: failures,
		Status:       "healthy",
		Message:      "DMARC reports are arriving as expected.",
	}
	if len(missing) > 0 {
		result.Status = "missing_reports"
		result.Message = "Expected DMARC reporters have not sent recent reports."
	}
	if failures >= check.FailureAlertAfter {
		result.Status = "ingestion_failures"
		result.Message = "DMARC report ingestion has recent failures."
	}
	if last.IsZero() && len(check.ExpectedOrgs) == 0 && failures == 0 {
		result.Status = "no_reports"
		result.Message = "No DMARC aggregate reports have been received yet."
	}
	return result
}
