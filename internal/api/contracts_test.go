package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/brandrisk"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

func TestNewScanResponse(t *testing.T) {
	t.Parallel()

	response := NewScanResponse(scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		Duration:  12 * time.Millisecond,
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok", SourceName: "example.com", LookupDuration: time.Millisecond},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
	})

	if response.Domain != "example.com" || response.DurationMS != 12 || len(response.Checks) != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Summary.Score == 0 || response.Checks[0].LookupDurationMS != 1 {
		t.Fatalf("summary/check metadata missing: %+v", response)
	}
	if response.Audit.Grade != "C" || response.Audit.Risk == "" || response.Audit.CTA == "" {
		t.Fatalf("audit metadata missing: %+v", response.Audit)
	}
	if response.Checks[1].Explanation == nil {
		t.Fatalf("DKIM explanation missing: %+v", response.Checks[1])
	}
	if response.Checks[1].Explanation.Severity != "high" {
		t.Fatalf("DKIM severity = %q, want high", response.Checks[1].Explanation.Severity)
	}
	if !strings.Contains(response.Checks[1].Explanation.WhatItMeans, "DKIM") {
		t.Fatalf("DKIM explanation not customer-facing: %+v", response.Checks[1].Explanation)
	}
	if !strings.Contains(response.Checks[1].Explanation.NextAction, "provider") {
		t.Fatalf("DKIM next action missing provider guidance: %+v", response.Checks[1].Explanation)
	}
	if response.Checks[1].Explanation.ExpectedPropagation != "DNS TXT changes usually propagate within minutes, but allow up to 24 hours before treating stale answers as failure." {
		t.Fatalf("unexpected propagation copy: %+v", response.Checks[1].Explanation)
	}
	if _, err := json.Marshal(response); err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
}

func TestNewScanResponseIncludesCustomerFacingExplanations(t *testing.T) {
	t.Parallel()

	response := NewScanResponse(scanner.Result{
		Domain:    "example.com",
		CheckedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusFail, Message: "SPF uses +all", Record: "v=spf1 +all"},
		DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
		DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusUnknown, Message: "lookup timed out", ErrorClass: "timeout"},
		BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass, Message: "BIMI ok", Record: "v=BIMI1; l=https://example.com/bimi.svg"},
	})

	if response.Checks[0].Explanation == nil {
		t.Fatalf("SPF explanation missing: %+v", response.Checks[0])
	}
	if response.Checks[0].Explanation.Severity != "high" || response.Checks[0].Explanation.Confidence != "high" {
		t.Fatalf("SPF explanation severity/confidence mismatch: %+v", response.Checks[0].Explanation)
	}
	if response.Checks[0].Explanation.ExactRecord != "v=spf1 +all" {
		t.Fatalf("SPF exact record not preserved: %+v", response.Checks[0].Explanation)
	}
	if response.Checks[2].Explanation == nil || response.Checks[2].Explanation.Confidence != "low" || response.Checks[2].Explanation.Severity != "low" {
		t.Fatalf("unknown/error explanation should be low confidence and low severity: %+v", response.Checks[2].Explanation)
	}
	if response.Checks[3].Explanation != nil {
		t.Fatalf("pass check should not emit explanation: %+v", response.Checks[3].Explanation)
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	checks, ok := payload["checks"].([]any)
	if !ok || len(checks) != 4 {
		t.Fatalf("checks missing from JSON: %s", data)
	}
	bimi, ok := checks[3].(map[string]any)
	if !ok {
		t.Fatalf("BIMI check JSON has unexpected shape: %#v", checks[3])
	}
	if _, ok := bimi["explanation"]; ok {
		t.Fatalf("pass check JSON should omit explanation: %s", data)
	}
}

func TestNewScanResponseTreatsNotFoundMissingAsMediumConfidence(t *testing.T) {
	t.Parallel()

	response := NewScanResponse(scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusMissing, Message: "No SPF record found", ErrorClass: "not_found"},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass, Record: "v=DMARC1; p=reject"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusPass},
	})

	if response.Checks[0].Explanation == nil {
		t.Fatalf("SPF explanation missing: %+v", response.Checks[0])
	}
	if response.Checks[0].Explanation.Confidence != "medium" {
		t.Fatalf("not_found missing SPF confidence = %q, want medium", response.Checks[0].Explanation.Confidence)
	}
}

func TestNewDomainResponse(t *testing.T) {
	t.Parallel()

	response := NewDomainResponse(domains.Domain{
		ID:        "domain-1",
		Domain:    "example.com",
		CreatedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		LatestCheck: &domains.Check{
			CheckedAt: time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC),
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusPass},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusPass},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing},
		},
	})

	if response.ID != "domain-1" || response.Domain != "example.com" || response.LatestCheck == nil {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.LatestCheck.SPF.Status != scanner.StatusPass {
		t.Fatalf("latest check not mapped: %+v", response.LatestCheck)
	}
}

func TestNewDashboardBrandRiskResponse(t *testing.T) {
	t.Parallel()

	response := NewDashboardBrandRiskResponse(domains.Domain{
		ID:     "domain-1",
		Domain: "example.com",
		LatestCheck: &domains.Check{
			CheckedAt: time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC),
			SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok"},
			DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
			DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
			BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		},
	}, []brandrisk.LookalikeFinding{{
		Domain:            "exmple.com",
		Kind:              brandrisk.CandidateTypo,
		Reason:            "single-character deletion",
		HasAddressRecords: true,
		Severity:          brandrisk.SeverityMedium,
		Evidence:          "DNS evidence: A/AAAA records are published.",
		RecommendedAction: "Review ownership.",
	}}, time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC))

	if response.DomainID != "domain-1" || response.Domain != "example.com" {
		t.Fatalf("domain mapping mismatch: %+v", response)
	}
	if len(response.Findings) == 0 || response.Findings[0].Type != "dmarc_policy_relaxed" {
		t.Fatalf("scan findings missing: %+v", response.Findings)
	}
	if len(response.Lookalikes) != 1 || response.Lookalikes[0].Domain != "exmple.com" || response.Lookalikes[0].Severity != "Medium" {
		t.Fatalf("lookalike mapping mismatch: %+v", response.Lookalikes)
	}
	if _, err := json.Marshal(response); err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
}

func TestNewDashboardSPFRouteResponseOmitsZeroLastSyncedAtAndSanitizesErrors(t *testing.T) {
	t.Parallel()

	response := NewDashboardSPFRouteResponse(spfroutes.Route{
		ID:               "route-1",
		DomainID:         "domain-1",
		Domain:           "example.com",
		RouteFQDN:        "example-com.route.dmark.is",
		LastSyncStatus:   spfroutes.StatusFailed,
		LastSyncError:    "Bunny DNS zone 801471 request failed with api_key=[REDACTED]",
		MechanismPolicy:  spfroutes.PolicySoftfail,
		LastPublishedSPF: "v=spf1 include:_spf.example.net ~all",
		Sources:          []spfroutes.Source{{Kind: spfroutes.SourceKindInclude, Value: "_spf.example.net", Enabled: true}},
	})

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "last_synced_at") {
		t.Fatalf("zero last_synced_at was not omitted: %s", body)
	}
	lowerBody := strings.ToLower(body)
	if strings.Contains(lowerBody, "bunny") || strings.Contains(body, "801471") || strings.Contains(lowerBody, "api_key") {
		t.Fatalf("response leaked provider details: %s", body)
	}
	if response.LastSyncError != "SPF route sync failed; see operational events for operator details" {
		t.Fatalf("last sync error = %q, want sanitized frontend-safe message", response.LastSyncError)
	}
}
