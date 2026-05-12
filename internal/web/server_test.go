package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type fakeScanner struct {
	result scanner.Result
	err    error
}

func (s fakeScanner) Scan(ctx context.Context, domain string) (scanner.Result, error) {
	return s.result, s.err
}

func TestHomeRenders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "meil") {
		t.Fatalf("body does not contain meil: %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `/assets/css/marketing-20260512.css`) {
		t.Fatalf("body does not contain fingerprinted stylesheet: %q", rec.Body.String())
	}
}

func TestMarketingStylesheetRenders(t *testing.T) {
	t.Setenv("ASSETS_DIR", "../../assets")

	req := httptest.NewRequest(http.MethodGet, "/assets/css/marketing-20260512.css", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", contentType)
	}
	if !strings.Contains(rec.Body.String(), ".bg-background") {
		t.Fatalf("stylesheet does not contain marketing CSS")
	}
}

func TestCheckDomainRendersScannerResult(t *testing.T) {
	t.Parallel()

	form := url.Values{"domain": {"example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewRouterWithScanner(fakeScanner{result: scanner.Result{
		Domain: "example.com",
		SPF:    scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass, Message: "SPF ok", Record: "v=spf1 -all"},
		DMARC:  scanner.CheckResult{Name: "DMARC", Status: scanner.StatusWarn, Message: "DMARC monitor", Record: "v=DMARC1; p=none"},
		BIMI:   scanner.CheckResult{Name: "BIMI", Status: scanner.StatusMissing, Message: "BIMI missing"},
		DKIM:   scanner.CheckResult{Name: "DKIM", Status: scanner.StatusMissing, Message: "DKIM missing"},
	}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"example.com", "SPF", "pass", "DMARC", "warn", "v=spf1 -all"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestCheckDomainRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	form := url.Values{"domain": {"localhost"}}
	req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}
