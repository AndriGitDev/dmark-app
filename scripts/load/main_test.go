package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewScenarioRequestAPIGetBearerToken(t *testing.T) {
	t.Parallel()

	base, err := url.Parse("https://meil.example")
	if err != nil {
		t.Fatalf("parsing base URL: %v", err)
	}
	req, err := newScenarioRequest(context.Background(), base, scenarioConfig{scenario: "api-get", path: "/api/v1/domains", token: "token-1"})
	if err != nil {
		t.Fatalf("newScenarioRequest returned error: %v", err)
	}
	if req.Method != http.MethodGet || req.URL.Path != "/api/v1/domains" {
		t.Fatalf("request = %s %s", req.Method, req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer token-1" {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

func TestNewScenarioRequestAPIPostBearerToken(t *testing.T) {
	t.Parallel()

	base, err := url.Parse("https://meil.example")
	if err != nil {
		t.Fatalf("parsing base URL: %v", err)
	}
	req, err := newScenarioRequest(context.Background(), base, scenarioConfig{
		scenario:    "api-post",
		path:        "/api/v1/domains/domain-1/check",
		body:        `{"force":true}`,
		contentType: "application/json",
		token:       "token-1",
	})
	if err != nil {
		t.Fatalf("newScenarioRequest returned error: %v", err)
	}
	if req.Method != http.MethodPost || req.URL.Path != "/api/v1/domains/domain-1/check" {
		t.Fatalf("request = %s %s", req.Method, req.URL.Path)
	}
	if req.Header.Get("Authorization") != "Bearer token-1" {
		t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", req.Header.Get("Content-Type"))
	}
	if body := readRequestBody(t, req); body != `{"force":true}` {
		t.Fatalf("body = %q", body)
	}
}

func TestRunScenarioCountsRequests(t *testing.T) {
	t.Parallel()

	base, err := url.Parse("https://meil.example")
	if err != nil {
		t.Fatalf("parsing base URL: %v", err)
	}
	var count atomic.Int64
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		count.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     http.Header{},
			Request:    req,
		}, nil
	}), Timeout: time.Second}
	results := runScenario(context.Background(), client, base, scenarioConfig{scenario: "api-get", path: "/healthz", requests: 3, concurrency: 2})
	if len(results) != 3 || count.Load() != 3 {
		t.Fatalf("results/count = %d/%d, want 3/3", len(results), count.Load())
	}
	if failed(results, nil) != 0 {
		t.Fatalf("failed(results) = %d, want 0", failed(results, nil))
	}
}

func TestParseExpectedStatuses(t *testing.T) {
	t.Parallel()

	statuses, err := parseExpectedStatuses("200, 202")
	if err != nil {
		t.Fatalf("parseExpectedStatuses returned error: %v", err)
	}
	if !statuses[http.StatusOK] || !statuses[http.StatusAccepted] {
		t.Fatalf("statuses = %#v, want 200 and 202", statuses)
	}
	if failed([]result{{status: http.StatusUnauthorized}}, statuses) != 1 {
		t.Fatalf("401 should fail when explicit statuses are configured")
	}
	if _, err := parseExpectedStatuses("200x"); err == nil {
		t.Fatalf("parseExpectedStatuses should reject malformed status")
	}
}

func readRequestBody(t *testing.T, req *http.Request) string {
	t.Helper()
	data, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(data)
}
