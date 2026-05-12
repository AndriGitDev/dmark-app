package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestScanCommandRequiresDomain(t *testing.T) {
	t.Parallel()

	cmd := newScanCommand()
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute returned nil error")
	}
}

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
	for _, want := range []string{"example.com", "SPF", "pass", "DMARC", "warn", "DKIM", "missing"} {
		if !strings.Contains(result, want) {
			t.Fatalf("output missing %q: %s", want, result)
		}
	}
}
