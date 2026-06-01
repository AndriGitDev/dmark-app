package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/monitoring"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
	"github.com/AndriGitDev/meil/internal/spfroutescheduler"
)

func TestScanCommandRequiresDomain(t *testing.T) {
	t.Parallel()

	cmd := newScanCommand()
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute returned nil error")
	}
}

func TestScanCommandAcceptsSelectorFlags(t *testing.T) {
	t.Parallel()

	cmd := newScanCommand()
	cmd.SetArgs([]string{"--selector", "custom", "--provider", "google", "example.com"})
	if err := cmd.ParseFlags([]string{"--selector", "custom", "--provider", "google"}); err != nil {
		t.Fatalf("ParseFlags returned error: %v", err)
	}
}

func TestIngestCommandParsesXMLAsJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "report.xml")
	if err := os.WriteFile(path, []byte(testAggregateReport), 0o600); err != nil {
		t.Fatalf("writing report: %v", err)
	}

	var out bytes.Buffer
	cmd := newIngestCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var payload ingestSummaryPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if payload.TotalReports != 1 || payload.TotalRecords != 1 || payload.TotalMessages != 42 {
		t.Fatalf("unexpected totals: %+v", payload)
	}
	if payload.Files[0].Reports[0].Domain != "example.com" || payload.Files[0].Reports[0].PolicyApplied != "reject" {
		t.Fatalf("unexpected report summary: %+v", payload.Files[0].Reports[0])
	}
}

func TestIngestCommandReadsXMLFromStdin(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := newIngestCommand()
	cmd.SetIn(strings.NewReader(testAggregateReport))
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	var payload ingestSummaryPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if payload.TotalReports != 1 || payload.Files[0].Reports[0].SourceFile != "stdin.xml" {
		t.Fatalf("unexpected stdin ingest summary: %+v", payload)
	}
}

func TestWorkerCommandListsRegisteredJobs(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:8080")
	t.Setenv("DATABASE_URL", "postgres://meil:***@localhost:5432/meil?sslmode=disable")
	t.Setenv("AUTH_TOKEN_TTL_MINUTES", "15")
	t.Setenv("PUBLIC_CHECKS_PER_MINUTE", "12")
	t.Setenv("DMARC_MAX_REPORT_BYTES", "10485760")

	var out bytes.Buffer
	cmd := newWorkerCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list-jobs"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	result := out.String()
	for _, want := range []string{"scan_domain", "refresh_enrichment", "sync_spf_route"} {
		if !strings.Contains(result, want) {
			t.Fatalf("output missing %q: %s", want, result)
		}
	}
}

func TestPrintSPFRouteAuditJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	audit := spfroutescheduler.AuditResult{
		Total:   2,
		Enabled: 2,
		Failed:  1,
		Stale:   1,
		Attention: []spfroutescheduler.AttentionItem{
			{RouteID: "route-1", Domain: "example.com", Status: spfroutes.StatusFailed, Reason: spfroutescheduler.ReasonFailed, LastError: "publisher unavailable"},
		},
	}
	if err := printSPFRouteAuditJSON(&out, audit); err != nil {
		t.Fatalf("printSPFRouteAuditJSON returned error: %v", err)
	}

	var payload spfroutescheduler.AuditResult
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if payload.Total != 2 || payload.Enabled != 2 || payload.Failed != 1 || payload.Stale != 1 {
		t.Fatalf("unexpected audit counts: %+v", payload)
	}
	if len(payload.Attention) != 1 || payload.Attention[0].RouteID != "route-1" || payload.Attention[0].LastError != "publisher unavailable" {
		t.Fatalf("unexpected attention payload: %+v", payload.Attention)
	}
}

func TestSPFRouteCommandIncludesAuditSubcommand(t *testing.T) {
	t.Parallel()

	cmd := newSPFRouteCommand()
	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "audit" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("spf-route command missing audit subcommand")
	}
}

func TestRootCommandIncludesSyntheticCheckSubcommand(t *testing.T) {
	t.Parallel()

	cmd := newRootCommand()
	found := false
	for _, child := range cmd.Commands() {
		if child.Name() == "synthetic-check" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("root command missing synthetic-check subcommand")
	}
}

func TestPrintSyntheticCheckJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	payload := monitoring.SyntheticReport{
		Failed: false,
		Checks: []monitoring.SyntheticCheckResult{{Name: "frontend_homepage", Status: monitoring.SyntheticStatusPass, Target: "https://example.test"}},
	}
	if err := printSyntheticCheckJSON(&out, payload); err != nil {
		t.Fatalf("printSyntheticCheckJSON returned error: %v", err)
	}
	var decoded struct {
		Failed bool `json:"failed"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if decoded.Failed || len(decoded.Checks) != 1 || decoded.Checks[0].Name != "frontend_homepage" || decoded.Checks[0].Status != "pass" {
		t.Fatalf("unexpected payload: %+v", decoded)
	}
}

func TestMigrationTargetParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantTarget    string
		wantDirection string
		wantErr       bool
	}{
		{name: "legacy postgres", args: []string{"up"}, wantTarget: "postgres", wantDirection: "up"},
		{name: "explicit postgres", args: []string{"postgres", "status"}, wantTarget: "postgres", wantDirection: "status"},
		{name: "clickhouse", args: []string{"clickhouse", "up"}, wantTarget: "clickhouse", wantDirection: "up"},
		{name: "bad target", args: []string{"mysql", "up"}, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target, direction, err := migrationTarget(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("migrationTarget returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationTarget returned error: %v", err)
			}
			if target != tt.wantTarget || direction != tt.wantDirection {
				t.Fatalf("migrationTarget = %s/%s, want %s/%s", target, direction, tt.wantTarget, tt.wantDirection)
			}
		})
	}
}

func TestMigrationDirFallsBackToRootPath(t *testing.T) {
	t.Parallel()

	got := migrationDir("definitely-missing/migrations")
	want := filepath.Join(string(os.PathSeparator), "definitely-missing/migrations")
	if got != want {
		t.Fatalf("migrationDir fallback = %q, want %q", got, want)
	}
}

func TestPrintScanResultJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := printScanResultJSON(&out, scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		Duration:  12 * time.Millisecond,
		SPF: scanner.CheckResult{
			Name:           "SPF",
			Status:         scanner.StatusPass,
			Message:        "SPF ok",
			SourceName:     "example.com",
			LookupDuration: time.Millisecond,
		},
		DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
		BIMI:  scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		DKIM:  scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
	}); err != nil {
		t.Fatalf("printScanResultJSON returned error: %v", err)
	}

	var payload struct {
		Domain     string `json:"domain"`
		CheckedAt  string `json:"checked_at"`
		DurationMS int64  `json:"duration_ms"`
		Summary    struct {
			Score  int            `json:"score"`
			Status scanner.Status `json:"status"`
		} `json:"summary"`
		Checks []struct {
			Name             string         `json:"name"`
			Status           scanner.Status `json:"status"`
			SourceName       string         `json:"source_name"`
			LookupDurationMS int64          `json:"lookup_duration_ms"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("JSON output did not parse: %v\n%s", err, out.String())
	}
	if payload.Domain != "example.com" || payload.CheckedAt != "2026-05-16T12:00:00Z" || payload.DurationMS != 12 {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if payload.Summary.Score == 0 || payload.Summary.Status == "" {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}
	if len(payload.Checks) != 4 || payload.Checks[0].Name != "SPF" || payload.Checks[0].Status != scanner.StatusPass || payload.Checks[0].SourceName != "example.com" || payload.Checks[0].LookupDurationMS != 1 {
		t.Fatalf("unexpected JSON checks: %+v", payload.Checks)
	}
}

const testAggregateReport = `<?xml version="1.0" encoding="UTF-8"?>
<feedback>
  <report_metadata>
    <org_name>Example Mailbox</org_name>
    <email>dmarc@example.net</email>
    <report_id>report-1</report_id>
    <date_range>
      <begin>1778889600</begin>
      <end>1778975999</end>
    </date_range>
  </report_metadata>
  <policy_published>
    <domain>example.com</domain>
    <p>reject</p>
  </policy_published>
  <record>
    <row>
      <source_ip>203.0.113.10</source_ip>
      <count>42</count>
      <policy_evaluated>
        <disposition>reject</disposition>
        <dkim>fail</dkim>
        <spf>pass</spf>
      </policy_evaluated>
    </row>
    <identifiers>
      <header_from>example.com</header_from>
    </identifiers>
    <auth_results>
      <spf>
        <domain>mail.example.com</domain>
        <result>pass</result>
      </spf>
    </auth_results>
  </record>
</feedback>`

func TestPrintScanResult(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := printScanResult(&out, scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
	}); err != nil {
		t.Fatalf("printScanResult returned error: %v", err)
	}

	result := out.String()
	for _, want := range []string{"example.com", "Score:", "Top issue:", "SPF", "pass", "DMARC", "warn", "DKIM", "missing"} {
		if !strings.Contains(result, want) {
			t.Fatalf("output missing %q: %s", want, result)
		}
	}
}
