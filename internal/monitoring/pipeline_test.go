package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/alerts"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type fakeScanner struct {
	result scanner.Result
	err    error
}

func (s fakeScanner) Scan(ctx context.Context, domain string) (scanner.Result, error) {
	return s.result, s.err
}

func TestRunScan(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	output, err := RunScan(context.Background(), fakeScanner{result: scanner.Result{
		Domain:    "example.com",
		CheckedAt: now,
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Record: "v=DMARC1; p=none"},
	}}, ScanInput{
		Domain: "example.com",
		Previous: scanner.Result{
			DMARC: scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject"},
		},
		AlertRule: alerts.Rule{Minimum: drift.SeverityWarn, Enabled: true},
	})
	if err != nil {
		t.Fatalf("RunScan returned error: %v", err)
	}
	if len(output.Events) == 0 {
		t.Fatalf("expected drift events")
	}
	if len(output.Emails) == 0 {
		t.Fatalf("expected alert email payload")
	}
}
