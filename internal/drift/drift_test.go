package drift

import (
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestCompareDetectsPolicyRelaxation(t *testing.T) {
	t.Parallel()

	events := Compare(scanner.Result{
		SPF:   check("SPF", scanner.StatusPass, "v=spf1 include:_spf.example.net -all"),
		DMARC: check("DMARC", scanner.StatusPass, "v=DMARC1; p=reject"),
	}, scanner.Result{
		SPF:   check("SPF", scanner.StatusWarn, "v=spf1 include:_spf.example.net ~all"),
		DMARC: check("DMARC", scanner.StatusWarn, "v=DMARC1; p=none"),
	})

	assertEvent(t, events, "SPF", KindPolicyRelaxed)
	assertEvent(t, events, "DMARC", KindPolicyRelaxed)
	assertEvent(t, events, "SPF", KindStatusRegression)
	assertEvent(t, events, "DMARC", KindStatusRegression)
}

func TestCompareDetectsRemovedSPFIncludeAndLookupLimit(t *testing.T) {
	t.Parallel()

	events := Compare(scanner.Result{
		SPF: check("SPF", scanner.StatusPass, "v=spf1 include:a.example include:b.example -all"),
	}, scanner.Result{
		SPF: check("SPF", scanner.StatusFail, "v=spf1 include:a.example include:c.example include:d.example include:e.example include:f.example include:g.example include:h.example include:i.example include:j.example include:k.example include:l.example -all"),
	})

	assertEvent(t, events, "SPF", KindIncludeRemoved)
	assertEvent(t, events, "SPF", KindLookupLimit)
}

func TestCompareDetectsDKIMAndBIMIRemoval(t *testing.T) {
	t.Parallel()

	events := Compare(scanner.Result{
		DKIM: scanner.CheckResult{
			Name:   "DKIM",
			Status: scanner.StatusPass,
			Selectors: []scanner.SelectorResult{
				{Selector: "google", Status: scanner.StatusPass},
				{Selector: "selector1", Status: scanner.StatusPass},
			},
		},
		BIMI: check("BIMI", scanner.StatusPass, "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem"),
	}, scanner.Result{
		DKIM: scanner.CheckResult{
			Name:   "DKIM",
			Status: scanner.StatusPass,
			Selectors: []scanner.SelectorResult{
				{Selector: "google", Status: scanner.StatusPass},
				{Selector: "selector1", Status: scanner.StatusMissing},
			},
		},
		BIMI: scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
	})

	assertEvent(t, events, "DKIM", KindDKIMRemoved)
	assertEvent(t, events, "BIMI", KindBIMIRemoved)
	assertEvent(t, events, "BIMI", KindRecordMissing)
}

func TestCompareDetectsRecovery(t *testing.T) {
	t.Parallel()

	events := Compare(scanner.Result{
		DMARC: check("DMARC", scanner.StatusWarn, "v=DMARC1; p=none"),
	}, scanner.Result{
		DMARC: check("DMARC", scanner.StatusPass, "v=DMARC1; p=reject"),
	})

	assertEvent(t, events, "DMARC", KindStatusRecovery)
}

func check(name string, status scanner.Status, record string) scanner.CheckResult {
	return scanner.CheckResult{Name: name, Status: status, Record: record}
}

func assertEvent(t *testing.T, events []Event, checkName string, kind Kind) {
	t.Helper()
	for _, event := range events {
		if event.CheckName == checkName && event.Kind == kind {
			return
		}
	}
	t.Fatalf("missing %s %s event in %#v", checkName, kind, events)
}
