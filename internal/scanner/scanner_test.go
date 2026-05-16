package scanner

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeResolver map[string]fakeTXT

type fakeTXT struct {
	records []string
	err     error
}

func (r fakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	result, ok := r[name]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	return result.records, result.err
}

func TestNormalizeDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: " HTTPS://Example.COM/path ", want: "example.com"},
		{input: "bücher.example", want: "xn--bcher-kva.example"},
		{input: "https://BÜCHER.example/path", want: "xn--bcher-kva.example"},
	}
	for _, tt := range tests {
		domain, err := NormalizeDomain(tt.input)
		if err != nil {
			t.Fatalf("NormalizeDomain(%q) returned error: %v", tt.input, err)
		}
		if domain != tt.want {
			t.Fatalf("NormalizeDomain(%q) = %q, want %q", tt.input, domain, tt.want)
		}
	}
}

func TestNormalizeDomainRejectsInvalidDomains(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "localhost", "exa mple.com", "-example.com", "example..com"} {
		if _, err := NormalizeDomain(input); err == nil {
			t.Fatalf("NormalizeDomain(%q) returned nil error", input)
		}
	}
}

func TestScanClassifiesEmailAuthRecords(t *testing.T) {
	t.Parallel()

	result, err := New(fakeResolver{
		"example.com":                   {records: []string{"v=spf1 include:_spf.example.net -all"}},
		"_dmarc.example.com":            {records: []string{"v=DMARC1; p=reject; rua=mailto:dmarc@example.com"}},
		"default._bimi.example.com":     {records: []string{"v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem"}},
		"google._domainkey.example.com": {records: []string{"v=DKIM1; k=rsa; p=abc123"}},
	}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertStatus(t, result.SPF, StatusPass)
	assertStatus(t, result.DMARC, StatusPass)
	assertStatus(t, result.BIMI, StatusPass)
	assertStatus(t, result.DKIM, StatusPass)
	if result.CheckedAt.IsZero() || result.Duration < 0 {
		t.Fatalf("expected scan metadata, got checked_at=%v duration=%v", result.CheckedAt, result.Duration)
	}
	if len(result.DKIM.Selectors) == 0 {
		t.Fatalf("expected DKIM selector probe results")
	}
}

func TestScanMarksMissingRecords(t *testing.T) {
	t.Parallel()

	result, err := New(fakeResolver{}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertStatus(t, result.SPF, StatusMissing)
	assertStatus(t, result.DMARC, StatusMissing)
	assertStatus(t, result.BIMI, StatusMissing)
	assertStatus(t, result.DKIM, StatusMissing)
}

func TestScanMarksDNSErrorsUnknown(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("resolver unavailable")
	result, err := New(fakeResolver{
		"example.com":                    {err: lookupErr},
		"_dmarc.example.com":             {err: lookupErr},
		"default._bimi.example.com":      {err: lookupErr},
		"default._domainkey.example.com": {err: lookupErr},
	}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertStatus(t, result.SPF, StatusUnknown)
	assertStatus(t, result.DMARC, StatusUnknown)
	assertStatus(t, result.BIMI, StatusUnknown)
	assertStatus(t, result.DKIM, StatusUnknown)
}

func TestScanUsesCustomDKIMSelectors(t *testing.T) {
	t.Parallel()

	result, err := NewWithSelectors(fakeResolver{
		"custom._domainkey.example.com": {records: []string{"v=DKIM1; p=abc123"}},
	}, []string{" custom ", "custom._domainkey.example.com"}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	assertStatus(t, result.DKIM, StatusPass)
	if len(result.DKIM.Selectors) != 1 || result.DKIM.Selectors[0].Selector != "custom" {
		t.Fatalf("selectors = %#v, want single normalized custom selector", result.DKIM.Selectors)
	}
}

func TestProviderDKIMSelectors(t *testing.T) {
	t.Parallel()

	selectors := ProviderDKIMSelectors("google-workspace")
	if len(selectors) == 0 || selectors[0] != "google" {
		t.Fatalf("google selectors = %#v, want google preset", selectors)
	}
	if selectors := ProviderDKIMSelectors("unknown"); selectors != nil {
		t.Fatalf("unknown selectors = %#v, want nil", selectors)
	}
}

func TestSPFClassificationDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record string
		want   Status
	}{
		{name: "hard fail", record: "v=spf1 include:_spf.example.net -all", want: StatusPass},
		{name: "soft fail", record: "v=spf1 include:_spf.example.net ~all", want: StatusWarn},
		{name: "neutral all", record: "v=spf1 ?all", want: StatusWarn},
		{name: "plus all", record: "v=spf1 +all", want: StatusFail},
		{name: "deprecated ptr", record: "v=spf1 ptr -all", want: StatusWarn},
		{name: "too many DNS lookups", record: "v=spf1 include:a.example include:b.example include:c.example include:d.example include:e.example include:f.example include:g.example include:h.example include:i.example include:j.example include:k.example -all", want: StatusFail},
		{name: "malformed mechanism", record: "v=spf1 frobnicate -all", want: StatusFail},
		{name: "redirect", record: "v=spf1 redirect=_spf.example.net", want: StatusPass},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := New(fakeResolver{
				"example.com": {records: []string{tt.record}},
			}).Scan(context.Background(), "example.com")
			if err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			assertStatus(t, result.SPF, tt.want)
			if result.SPF.SourceName != "example.com" {
				t.Fatalf("SPF source = %q, want example.com", result.SPF.SourceName)
			}
		})
	}
}

func TestDMARCClassificationDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record string
		want   Status
	}{
		{name: "reject policy", record: "v=DMARC1; p=reject; rua=mailto:dmarc@example.com; adkim=s; aspf=r", want: StatusPass},
		{name: "none policy", record: "v=DMARC1; p=none", want: StatusWarn},
		{name: "subdomain none", record: "v=DMARC1; p=reject; sp=none", want: StatusWarn},
		{name: "partial pct", record: "v=DMARC1; p=reject; pct=50", want: StatusWarn},
		{name: "invalid pct", record: "v=DMARC1; p=reject; pct=101", want: StatusFail},
		{name: "invalid alignment", record: "v=DMARC1; p=reject; adkim=x", want: StatusFail},
		{name: "invalid rua", record: "v=DMARC1; p=reject; rua=https://example.com/report", want: StatusFail},
		{name: "malformed tag", record: "v=DMARC1; p=reject; malformed", want: StatusFail},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := New(fakeResolver{
				"_dmarc.example.com": {records: []string{tt.record}},
			}).Scan(context.Background(), "example.com")
			if err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			assertStatus(t, result.DMARC, tt.want)
		})
	}
}

func TestBIMIClassificationDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dmarcRecord string
		bimiRecord  string
		want        Status
	}{
		{name: "published with certificate", dmarcRecord: "v=DMARC1; p=reject", bimiRecord: "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem", want: StatusPass},
		{name: "missing logo", dmarcRecord: "v=DMARC1; p=reject", bimiRecord: "v=BIMI1; a=https://example.com/vmc.pem", want: StatusFail},
		{name: "invalid logo URI", dmarcRecord: "v=DMARC1; p=reject", bimiRecord: "v=BIMI1; l=http://example.com/bimi.png", want: StatusFail},
		{name: "missing certificate", dmarcRecord: "v=DMARC1; p=reject", bimiRecord: "v=BIMI1; l=https://example.com/bimi.svg", want: StatusWarn},
		{name: "dmarc not enforced", dmarcRecord: "v=DMARC1; p=none", bimiRecord: "v=BIMI1; l=https://example.com/bimi.svg; a=https://example.com/vmc.pem", want: StatusWarn},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := New(fakeResolver{
				"_dmarc.example.com":        {records: []string{tt.dmarcRecord}},
				"default._bimi.example.com": {records: []string{tt.bimiRecord}},
			}).Scan(context.Background(), "example.com")
			if err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			assertStatus(t, result.BIMI, tt.want)
		})
	}
}

func TestLookupErrorClassification(t *testing.T) {
	t.Parallel()

	timeoutErr := &net.DNSError{Err: "timeout", IsTimeout: true}
	result, err := New(fakeResolver{
		"example.com": {err: timeoutErr},
	}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	assertStatus(t, result.SPF, StatusUnknown)
	if result.SPF.ErrorClass != "timeout" {
		t.Fatalf("SPF error class = %q, want timeout", result.SPF.ErrorClass)
	}
}

func assertStatus(t *testing.T, check CheckResult, want Status) {
	t.Helper()
	if check.Status != want {
		t.Fatalf("%s status = %q, want %q (%s)", check.Name, check.Status, want, check.Message)
	}
}
