package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	duration time.Duration
	status   int
	err      error
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("load", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	baseURL := flags.String("base-url", "http://127.0.0.1:8080", "target base URL")
	scenario := flags.String("scenario", "checker", "scenario: checker, api-get, api-post, reporter-get")
	domain := flags.String("domain", "example.com", "domain used by checker scenario")
	path := flags.String("path", "/healthz", "GET path used by api-get and reporter-get scenarios")
	body := flags.String("body", "", "request body used by api-post scenario")
	contentType := flags.String("content-type", "application/json", "request content type used by api-post scenario")
	requests := flags.Int("requests", 50, "total requests")
	concurrency := flags.Int("concurrency", 5, "parallel workers")
	timeout := flags.Duration("timeout", 5*time.Second, "per-request timeout")
	token := flags.String("api-token", "", "optional bearer token for API scenarios")
	expectStatus := flags.String("expect-status", "", "comma-separated acceptable HTTP statuses; defaults to any 2xx-4xx response")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *requests <= 0 {
		return fmt.Errorf("requests must be positive")
	}
	if *concurrency <= 0 {
		return fmt.Errorf("concurrency must be positive")
	}
	expectedStatuses, err := parseExpectedStatuses(*expectStatus)
	if err != nil {
		return err
	}
	target, err := url.Parse(strings.TrimRight(*baseURL, "/"))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("base-url is invalid")
	}
	client := http.Client{Timeout: *timeout}
	results := runScenario(ctx, client, target, scenarioConfig{
		scenario:    *scenario,
		domain:      *domain,
		path:        *path,
		body:        *body,
		contentType: *contentType,
		requests:    *requests,
		concurrency: *concurrency,
		token:       *token,
	})
	printSummary(os.Stdout, *scenario, results, expectedStatuses)
	if failures := failed(results, expectedStatuses); failures > 0 {
		return fmt.Errorf("load scenario completed with %d failures", failures)
	}
	return nil
}

type scenarioConfig struct {
	scenario    string
	domain      string
	path        string
	body        string
	contentType string
	requests    int
	concurrency int
	token       string
}

func runScenario(ctx context.Context, client http.Client, base *url.URL, cfg scenarioConfig) []result {
	results := make([]result, cfg.requests)
	var next atomic.Int64
	var wg sync.WaitGroup
	for worker := 0; worker < cfg.concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1)) - 1
				if index >= cfg.requests {
					return
				}
				results[index] = oneRequest(ctx, client, base, cfg)
			}
		}()
	}
	wg.Wait()
	return results
}

func oneRequest(ctx context.Context, client http.Client, base *url.URL, cfg scenarioConfig) result {
	req, err := newScenarioRequest(ctx, base, cfg)
	if err != nil {
		return result{err: err}
	}
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return result{duration: duration, err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return result{duration: duration, status: resp.StatusCode}
}

func newScenarioRequest(ctx context.Context, base *url.URL, cfg scenarioConfig) (*http.Request, error) {
	switch cfg.scenario {
	case "checker":
		target := *base
		target.Path = "/check"
		body := url.Values{"domain": []string{cfg.domain}}.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewBufferString(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	case "api-get", "reporter-get":
		target := *base
		target.Path = cfg.path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cfg.token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.token))
		}
		return req, nil
	case "api-post":
		target := *base
		target.Path = cfg.path
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewBufferString(cfg.body))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cfg.contentType) != "" {
			req.Header.Set("Content-Type", strings.TrimSpace(cfg.contentType))
		}
		if strings.TrimSpace(cfg.token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.token))
		}
		return req, nil
	default:
		return nil, fmt.Errorf("unknown scenario %q", cfg.scenario)
	}
}

func printSummary(w io.Writer, scenario string, results []result, expectedStatuses map[int]bool) {
	durations := make([]time.Duration, 0, len(results))
	statuses := map[int]int{}
	for _, result := range results {
		if result.err == nil {
			durations = append(durations, result.duration)
			statuses[result.status]++
		}
	}
	sort.Slice(durations, func(i int, j int) bool { return durations[i] < durations[j] })
	_, _ = fmt.Fprintf(w, "scenario=%s requests=%d failures=%d\n", scenario, len(results), failed(results, expectedStatuses))
	_, _ = fmt.Fprintf(w, "statuses=%v\n", statuses)
	if len(durations) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "p50=%s p95=%s max=%s\n", percentile(durations, 0.50), percentile(durations, 0.95), durations[len(durations)-1])
}

func failed(results []result, expectedStatuses map[int]bool) int {
	failures := 0
	for _, result := range results {
		if failedResult(result, expectedStatuses) {
			failures++
		}
	}
	return failures
}

func failedResult(result result, expectedStatuses map[int]bool) bool {
	if result.err != nil {
		return true
	}
	if len(expectedStatuses) > 0 {
		return !expectedStatuses[result.status]
	}
	return result.status < 200 || result.status >= 500
}

func parseExpectedStatuses(raw string) (map[int]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	statuses := map[int]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		status, err := strconv.Atoi(part)
		if err != nil || status < 100 || status > 599 {
			return nil, fmt.Errorf("invalid expected status %q", part)
		}
		statuses[status] = true
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("expect-status did not include any statuses")
	}
	return statuses, nil
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}
