package ingest

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const sampleReport = `<?xml version="1.0" encoding="UTF-8"?>
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
    <adkim>s</adkim>
    <aspf>r</aspf>
    <p>reject</p>
    <sp>quarantine</sp>
    <pct>100</pct>
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
      <dkim>
        <domain>example.com</domain>
        <selector>selector1</selector>
        <result>fail</result>
      </dkim>
      <spf>
        <domain>mail.example.com</domain>
        <result>pass</result>
      </spf>
    </auth_results>
  </record>
</feedback>`

func TestParseAggregatePayloadPlainXML(t *testing.T) {
	t.Parallel()

	reports, err := ParseAggregatePayload(context.Background(), "report.xml", []byte(sampleReport), 0)
	if err != nil {
		t.Fatalf("ParseAggregatePayload returned error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("len(reports) = %d, want 1", len(reports))
	}
	report := reports[0]
	if report.ReportID != "report-1" || report.Domain != "example.com" || report.Policy.P != "reject" || report.Records[0].Count != 42 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.ContentHash == "" || len(report.RawXML) == 0 {
		t.Fatalf("expected retained hash and raw XML")
	}
	if len(report.AuthResults) != 2 {
		t.Fatalf("len(AuthResults) = %d, want 2", len(report.AuthResults))
	}
}

func TestParseAggregatePayloadGzip(t *testing.T) {
	t.Parallel()

	var payload bytes.Buffer
	writer := gzip.NewWriter(&payload)
	if _, err := writer.Write([]byte(sampleReport)); err != nil {
		t.Fatalf("writing gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing gzip: %v", err)
	}

	reports, err := ParseAggregatePayload(context.Background(), "report.xml.gz", payload.Bytes(), 0)
	if err != nil {
		t.Fatalf("ParseAggregatePayload returned error: %v", err)
	}
	if reports[0].ReportID != "report-1" {
		t.Fatalf("ReportID = %q, want report-1", reports[0].ReportID)
	}
}

func TestParseAggregatePayloadZip(t *testing.T) {
	t.Parallel()

	payload := zipPayload(t, "reports/report.xml", sampleReport)
	reports, err := ParseAggregatePayload(context.Background(), "reports.zip", payload, 0)
	if err != nil {
		t.Fatalf("ParseAggregatePayload returned error: %v", err)
	}
	if reports[0].SourceFile != "reports/report.xml" {
		t.Fatalf("SourceFile = %q, want reports/report.xml", reports[0].SourceFile)
	}
}

func TestParseAggregatePayloadRejectsZipTraversal(t *testing.T) {
	t.Parallel()

	payload := zipPayload(t, "../report.xml", sampleReport)
	if _, err := ParseAggregatePayload(context.Background(), "reports.zip", payload, 0); !errors.Is(err, ErrUnsafeZipPath) {
		t.Fatalf("error = %v, want ErrUnsafeZipPath", err)
	}
}

func TestParseAggregatePayloadRejectsOversize(t *testing.T) {
	t.Parallel()

	if _, err := ParseAggregatePayload(context.Background(), "report.xml", []byte(sampleReport), 10); !errors.Is(err, ErrReportTooLarge) {
		t.Fatalf("error = %v, want ErrReportTooLarge", err)
	}
}

func TestParseAggregatePayloadProviderFixtures(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		fixture     string
		orgName     string
		records     int
		authResults int
	}{
		{fixture: "google.xml", orgName: "google.com", records: 1, authResults: 2},
		{fixture: "microsoft.xml", orgName: "microsoft.com", records: 1, authResults: 1},
		{fixture: "yahoo.xml", orgName: "Yahoo", records: 1, authResults: 2},
		{fixture: "fastmail.xml", orgName: "Fastmail", records: 2, authResults: 3},
		{fixture: "proton.xml", orgName: "Proton Mail", records: 1, authResults: 2},
		{fixture: "mimecast.xml", orgName: "Mimecast", records: 1, authResults: 2},
		{fixture: "proofpoint.xml", orgName: "Proofpoint", records: 1, authResults: 2},
	} {
		tt := tt
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			// #nosec G304 -- fixture names are fixed by the test table above.
			payload, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			reports, err := ParseAggregatePayload(context.Background(), tt.fixture, payload, 0)
			if err != nil {
				t.Fatalf("ParseAggregatePayload returned error: %v", err)
			}
			if len(reports) != 1 || reports[0].OrgName != tt.orgName || len(reports[0].Records) != tt.records || len(reports[0].AuthResults) != tt.authResults {
				t.Fatalf("unexpected parsed fixture: %+v", reports)
			}
			for _, record := range reports[0].Records {
				if record.Count == 0 || record.SourceIP == nil || record.HeaderFrom == "" {
					t.Fatalf("unexpected parsed record: %+v", record)
				}
			}
		})
	}
}

func zipPayload(t *testing.T, name string, content string) []byte {
	t.Helper()

	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatalf("writing zip file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	return payload.Bytes()
}
