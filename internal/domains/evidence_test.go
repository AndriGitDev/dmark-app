package domains

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestNewScanEvidenceSnapshotSanitizesRawRecordsAndErrors(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 2, 10, 30, 0, 0, time.UTC)
	snapshot := NewScanEvidenceSnapshot(scanner.Result{
		Domain:    "example.com",
		CheckedAt: checkedAt,
		Duration:  42 * time.Millisecond,
		SPF: scanner.CheckResult{
			Name:           "SPF",
			Status:         scanner.StatusWarn,
			Message:        "SPF softfail",
			Record:         "v=spf1 include:secret-mailer.example ~all",
			SourceName:     "example.com",
			LookupDuration: 7 * time.Millisecond,
			ErrorClass:     "",
		},
		DKIM: scanner.CheckResult{
			Name:       "DKIM",
			Status:     scanner.StatusMissing,
			SourceName: "default._domainkey.example.com",
			ErrorClass: "not_found",
			Selectors: []scanner.SelectorResult{{
				Selector: "default",
				Record:   "v=DKIM1; p=SECRET_PUBLIC_KEY_MATERIAL",
				Status:   scanner.StatusMissing,
			}},
		},
		DMARC: scanner.CheckResult{
			Name:       "DMARC",
			Status:     scanner.StatusUnknown,
			Message:    "provider api_key=super-secret-token failed with raw DNS response",
			ErrorClass: "timeout",
		},
		BIMI: scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
	})

	if snapshot.Domain != "example.com" || !snapshot.CheckedAt.Equal(checkedAt) || snapshot.DurationMS != 42 {
		t.Fatalf("snapshot metadata = %+v, want stable domain/time/duration", snapshot)
	}
	if len(snapshot.Checks) != 4 {
		t.Fatalf("snapshot checks length = %d, want 4", len(snapshot.Checks))
	}
	if snapshot.Checks[0].Name != "SPF" || !snapshot.Checks[0].RecordPresent || snapshot.Checks[0].RecordLength != len("v=spf1 include:secret-mailer.example ~all") {
		t.Fatalf("SPF evidence = %+v, want name/status/source and record presence/length", snapshot.Checks[0])
	}
	if len(snapshot.Checks[1].Selectors) != 1 || !snapshot.Checks[1].Selectors[0].RecordPresent {
		t.Fatalf("DKIM selector evidence = %+v, want selector record presence only", snapshot.Checks[1].Selectors)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	body := string(encoded)
	for _, forbidden := range []string{"secret-mailer", "SECRET_PUBLIC_KEY_MATERIAL", "super-secret-token", "api_key", "raw DNS response"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("evidence snapshot leaked %q in %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "\"error_class\":\"timeout\"") {
		t.Fatalf("evidence snapshot omitted sanitized error class: %s", body)
	}
}
