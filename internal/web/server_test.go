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
}
