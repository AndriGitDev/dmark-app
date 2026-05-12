package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
