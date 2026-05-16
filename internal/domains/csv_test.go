package domains

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestParseCSV(t *testing.T) {
	t.Parallel()

	domains, err := ParseCSV(strings.NewReader("domain\nExample.com\nhttps://MAIL.example.org/path\nexample.com\n"))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}
	if len(domains) != 2 || domains[0] != "example.com" || domains[1] != "mail.example.org" {
		t.Fatalf("domains = %#v, want normalized unique domains", domains)
	}
}

func TestParseCSVRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	if _, err := ParseCSV(strings.NewReader("domain\nlocalhost\n")); err == nil {
		t.Fatalf("ParseCSV returned nil error for invalid domain")
	}
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteCSV(&out, []Domain{{
		Domain:    "example.com",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		LatestCheck: &Check{
			SPF:       scanner.CheckResult{Status: scanner.StatusPass},
			DKIM:      scanner.CheckResult{Status: scanner.StatusWarn},
			DMARC:     scanner.CheckResult{Status: scanner.StatusFail},
			BIMI:      scanner.CheckResult{Status: scanner.StatusMissing},
			CheckedAt: time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC),
		},
	}})
	if err != nil {
		t.Fatalf("WriteCSV returned error: %v", err)
	}
	output := out.String()
	for _, want := range []string{"domain,created_at,last_checked_at", "example.com", "pass,warn,fail,missing"} {
		if !strings.Contains(output, want) {
			t.Fatalf("CSV output missing %q: %s", want, output)
		}
	}
}
