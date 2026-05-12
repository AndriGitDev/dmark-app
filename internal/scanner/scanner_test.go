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

	domain, err := NormalizeDomain(" HTTPS://Example.COM/path ")
	if err != nil {
		t.Fatalf("NormalizeDomain returned error: %v", err)
	}
	if domain != "example.com" {
		t.Fatalf("domain = %q, want %q", domain, "example.com")
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
		"_dmarc.example.com":            {records: []string{"v=DMARC1; p=none; rua=mailto:dmarc@example.com"}},
		"default._bimi.example.com":     {records: []string{"v=BIMI1; l=https://example.com/bimi.svg"}},
		"google._domainkey.example.com": {records: []string{"v=DKIM1; k=rsa; p=abc123"}},
	}).Scan(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertStatus(t, result.SPF, StatusPass)
	assertStatus(t, result.DMARC, StatusWarn)
	assertStatus(t, result.BIMI, StatusPass)
	assertStatus(t, result.DKIM, StatusPass)
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

func assertStatus(t *testing.T, check CheckResult, want Status) {
	t.Helper()
	if check.Status != want {
		t.Fatalf("%s status = %q, want %q (%s)", check.Name, check.Status, want, check.Message)
	}
}
