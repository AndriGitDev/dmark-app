package ch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMigrationStatements(t *testing.T) {
	t.Parallel()

	statements, err := LoadMigrationStatements(filepath.Join("..", "..", "..", "migrations", "clickhouse"))
	if err != nil {
		t.Fatalf("LoadMigrationStatements returned error: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("len(statements) = %d, want 2", len(statements))
	}
	if !strings.Contains(statements[0], "CREATE TABLE IF NOT EXISTS dmarc_aggregate_rows") {
		t.Fatalf("first statement missing aggregate table: %s", statements[0])
	}
	if strings.Contains(statements[0], ";") {
		t.Fatalf("statement should not keep trailing semicolon: %s", statements[0])
	}
}

func TestRunMigrationsPostsStatements(t *testing.T) {
	t.Parallel()

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Query().Get("database") != "meil" || r.URL.Query().Get("user") != "meil" || r.URL.Query().Get("password") != "secret" {
			t.Fatalf("query = %s, want database/user/password", r.URL.RawQuery)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		requests = append(requests, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := RunMigrations(context.Background(), MigrationConfig{
		URL:      server.URL,
		Database: "meil",
		User:     "meil",
		Password: "secret",
		Dir:      filepath.Join("..", "..", "..", "migrations", "clickhouse"),
	})
	if err != nil {
		t.Fatalf("RunMigrations returned error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("len(requests) = %d, want 2", len(requests))
	}
	if !strings.Contains(requests[1], "dmarc_report_imports") {
		t.Fatalf("second migration missing imports table: %s", requests[1])
	}
}

func TestStatusQueriesTables(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statement := r.URL.Query().Get("query")
		if statement != "" {
			t.Fatalf("query parameter should not carry statement: %s", statement)
		}
		_, _ = w.Write([]byte("dmarc_aggregate_rows\ndmarc_report_imports\n"))
	}))
	defer server.Close()

	status, err := Status(context.Background(), MigrationConfig{URL: server.URL, Database: "meil"})
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if len(status.Tables) != 2 || status.Tables[0] != "dmarc_aggregate_rows" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestClickHouseEndpointValidation(t *testing.T) {
	t.Parallel()

	endpoint, err := clickhouseEndpoint(MigrationConfig{URL: "http://clickhouse:8123", Database: "meil", User: "u", Password: "p"})
	if err != nil {
		t.Fatalf("clickhouseEndpoint returned error: %v", err)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("endpoint did not parse: %v", err)
	}
	if parsed.Query().Get("database") != "meil" || parsed.Query().Get("user") != "u" || parsed.Query().Get("password") != "p" {
		t.Fatalf("endpoint query = %s", parsed.RawQuery)
	}
	if _, err := clickhouseEndpoint(MigrationConfig{URL: "://bad"}); err == nil {
		t.Fatalf("clickhouseEndpoint returned nil error for invalid URL")
	}
}

func TestClickHouseRequestErrorOmitsURLSecrets(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  "Post",
		URL: "http://localhost:8123?database=meil&password=secret&user=meil",
		Err: errors.New("connection refused"),
	}
	message := clickhouseRequestError(err)
	if strings.Contains(message, "secret") || strings.Contains(message, "password=") {
		t.Fatalf("request error leaked secret material: %s", message)
	}
	if message != "connection refused" {
		t.Fatalf("message = %q, want connection refused", message)
	}
}
