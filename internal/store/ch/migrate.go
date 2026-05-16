package ch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MigrationConfig struct {
	URL      string
	Database string
	User     string
	Password string
	Dir      string
}

type MigrationStatus struct {
	Database string
	Tables   []string
}

func RunMigrations(ctx context.Context, cfg MigrationConfig) error {
	statements, err := LoadMigrationStatements(cfg.Dir)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 30 * time.Second}
	for _, statement := range statements {
		if err := execute(ctx, client, cfg, statement); err != nil {
			return err
		}
	}
	return nil
}

func Status(ctx context.Context, cfg MigrationConfig) (MigrationStatus, error) {
	client := http.Client{Timeout: 10 * time.Second}
	body, err := query(ctx, client, cfg, "SELECT name FROM system.tables WHERE database = currentDatabase() ORDER BY name FORMAT TSV")
	if err != nil {
		return MigrationStatus{}, err
	}
	tables := []string{}
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tables = append(tables, line)
		}
	}
	return MigrationStatus{Database: cfg.Database, Tables: tables}, nil
}

func LoadMigrationStatements(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join("migrations", "clickhouse")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading ClickHouse migrations: %w", err)
	}
	statements := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// #nosec G304 -- ClickHouse migrations are operator-controlled files from the configured migration directory.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading ClickHouse migration %s: %w", path, err)
		}
		statements = append(statements, splitStatements(string(data))...)
	}
	if len(statements) == 0 {
		return nil, fmt.Errorf("no ClickHouse migration statements found in %s", dir)
	}
	return statements, nil
}

func splitStatements(sql string) []string {
	statements := []string{}
	var current strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			statement := strings.TrimSpace(current.String())
			statement = strings.TrimSuffix(statement, ";")
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}
	statement := strings.TrimSpace(current.String())
	if statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func execute(ctx context.Context, client http.Client, cfg MigrationConfig, statement string) error {
	if _, err := query(ctx, client, cfg, statement); err != nil {
		return fmt.Errorf("executing ClickHouse migration: %w", err)
	}
	return nil
}

func query(ctx context.Context, client http.Client, cfg MigrationConfig, statement string) (string, error) {
	endpoint, err := clickhouseEndpoint(cfg)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(statement))
	if err != nil {
		return "", fmt.Errorf("creating ClickHouse request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting ClickHouse: %s", clickhouseRequestError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ClickHouse response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ClickHouse returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func clickhouseRequestError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

func clickhouseEndpoint(cfg MigrationConfig) (string, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		rawURL = "http://localhost:8123"
	}
	endpoint, err := url.Parse(rawURL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return "", fmt.Errorf("CLICKHOUSE_URL is invalid")
	}
	query := endpoint.Query()
	if strings.TrimSpace(cfg.Database) != "" {
		query.Set("database", cfg.Database)
	}
	if strings.TrimSpace(cfg.User) != "" {
		query.Set("user", cfg.User)
	}
	if strings.TrimSpace(cfg.Password) != "" {
		query.Set("password", cfg.Password)
	}
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}
