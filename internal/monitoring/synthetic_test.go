package monitoring

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeDNSResolver struct {
	txt   map[string][]string
	cname map[string]string
	err   error
}

func (f fakeDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.txt[name], nil
}

func (f fakeDNSResolver) LookupCNAME(ctx context.Context, name string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.cname[name], nil
}

func TestSyntheticRunnerPassesPublicFlowsAndDNS(t *testing.T) {
	t.Parallel()

	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer frontend.Close()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/public/scan":
			if r.URL.Query().Get("domain") != "kastro.is" {
				t.Fatalf("unexpected scan domain: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"domain":"kastro.is","checked_at":"2026-06-01T00:00:00Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	runner := SyntheticRunner{
		HTTPClient: frontend.Client(),
		Now:        func() time.Time { return time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC) },
		Resolver: fakeDNSResolver{
			txt:   map[string][]string{"route.example.com": {"v=spf1 include:_spf.example.net ~all"}},
			cname: map[string]string{"scan.example.com": "target.example.net."},
		},
	}
	result := runner.Run(context.Background(), SyntheticConfig{
		FrontendURL:    frontend.URL,
		BackendURL:     backend.URL,
		ScanDomain:     "kastro.is",
		RouteTXTName:   "route.example.com",
		RouteTXTWanted: "include:_spf.example.net",
		RouteCNAMEName: "scan.example.com",
		RouteCNAMEWant: "target.example.net.",
	})

	if result.Failed {
		t.Fatalf("expected all checks to pass: %+v", result)
	}
	wantNames := []string{"frontend_homepage", "backend_healthz", "public_scan", "route_txt", "route_cname"}
	if len(result.Checks) != len(wantNames) {
		t.Fatalf("expected %d checks, got %d: %+v", len(wantNames), len(result.Checks), result.Checks)
	}
	for i, want := range wantNames {
		if result.Checks[i].Name != want || result.Checks[i].Status != SyntheticStatusPass {
			t.Fatalf("check %d = %+v, want %s pass", i, result.Checks[i], want)
		}
	}
}

func TestSyntheticRunnerRunsAdditionalScanFixtureDomains(t *testing.T) {
	t.Parallel()

	seen := []string{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`ok`))
		case "/api/public/scan":
			domain := r.URL.Query().Get("domain")
			seen = append(seen, domain)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"domain":"` + domain + `","checked_at":"2026-06-01T00:00:00Z"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer backend.Close()

	runner := SyntheticRunner{
		HTTPClient: backend.Client(),
		Now:        func() time.Time { return time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC) },
	}
	result := runner.Run(context.Background(), SyntheticConfig{
		FrontendURL:        backend.URL,
		BackendURL:         backend.URL,
		ScanDomain:         "kastro.is",
		ScanFixtureDomains: []string{"missing-dmarc.example", "xn--e1afmkfd.xn--p1ai"},
	})

	if result.Failed {
		t.Fatalf("expected scan fixtures to pass: %+v", result)
	}
	wantNames := []string{"public_scan", "public_scan:missing-dmarc.example", "public_scan:xn--e1afmkfd.xn--p1ai"}
	for _, want := range wantNames {
		found := false
		for _, check := range result.Checks {
			if check.Name == want && check.Status == SyntheticStatusPass {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing passing check %q in %+v", want, result.Checks)
		}
	}
	if strings.Join(seen, ",") != "kastro.is,missing-dmarc.example,xn--e1afmkfd.xn--p1ai" {
		t.Fatalf("scan domains = %v", seen)
	}
}

func TestSyntheticRunnerFailsStalePublicScanEvidence(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_, _ = w.Write([]byte(`ok`))
		case "/api/public/scan":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"domain":"kastro.is","checked_at":"2026-06-01T00:00:00Z"}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer backend.Close()

	runner := SyntheticRunner{
		HTTPClient: backend.Client(),
		Now:        func() time.Time { return time.Date(2026, 6, 1, 0, 10, 1, 0, time.UTC) },
	}
	result := runner.Run(context.Background(), SyntheticConfig{FrontendURL: backend.URL, BackendURL: backend.URL, ScanDomain: "kastro.is"})

	if !result.Failed {
		t.Fatalf("expected stale scan evidence to fail: %+v", result)
	}
	var scanCheck SyntheticCheckResult
	for _, check := range result.Checks {
		if check.Name == "public_scan" {
			scanCheck = check
		}
	}
	if scanCheck.Status != SyntheticStatusFail || scanCheck.ErrorClass != "freshness" {
		t.Fatalf("public scan check = %+v, want freshness failure", scanCheck)
	}
}

func TestSyntheticRunnerFailsAndSanitizesHTTPAndSchemaErrors(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/public/scan" {
			_, _ = w.Write([]byte(`{"domain":"other.example"}`))
			return
		}
		http.Error(w, "token=secret should not leak", http.StatusInternalServerError)
	}))
	defer backend.Close()

	runner := SyntheticRunner{HTTPClient: backend.Client(), Resolver: fakeDNSResolver{err: errors.New("lookup failed with api_key=secret")}}
	result := runner.Run(context.Background(), SyntheticConfig{FrontendURL: backend.URL, BackendURL: backend.URL, ScanDomain: "kastro.is", RouteTXTName: "route.example.com", RouteTXTWanted: "wanted"})

	if !result.Failed {
		t.Fatalf("expected result to fail: %+v", result)
	}
	joined := ""
	for _, check := range result.Checks {
		joined += check.ErrorMessage + " "
		if check.Status == SyntheticStatusFail && check.ErrorClass == "" {
			t.Fatalf("failed check missing error class: %+v", check)
		}
	}
	if strings.Contains(strings.ToLower(joined), "secret") || strings.Contains(strings.ToLower(joined), "token=") || strings.Contains(strings.ToLower(joined), "api_key=") {
		t.Fatalf("error messages were not sanitized: %q", joined)
	}
}
