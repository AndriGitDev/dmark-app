package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	SyntheticStatusPass    = "pass"
	SyntheticStatusFail    = "fail"
	SyntheticStatusWarn    = "warn"
	SyntheticStatusSkipped = "skipped"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupCNAME(ctx context.Context, name string) (string, error)
}

type SyntheticConfig struct {
	FrontendURL    string
	BackendURL     string
	ScanDomain     string
	MaxScanAge     time.Duration
	RouteTXTName   string
	RouteTXTWanted string
	RouteCNAMEName string
	RouteCNAMEWant string
	Timeout        time.Duration
}

type SyntheticReport struct {
	Failed    bool                   `json:"failed"`
	CheckedAt time.Time              `json:"checked_at"`
	Checks    []SyntheticCheckResult `json:"checks"`
}

type SyntheticCheckResult struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	Target       string `json:"target,omitempty"`
	LatencyMS    int64  `json:"latency_ms,omitempty"`
	ErrorClass   string `json:"error_class,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type SyntheticRunner struct {
	HTTPClient HTTPDoer
	Resolver   DNSResolver
	Now        func() time.Time
}

func (r SyntheticRunner) Run(ctx context.Context, cfg SyntheticConfig) SyntheticReport {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	report := SyntheticReport{CheckedAt: now().UTC()}
	report.Checks = append(report.Checks, r.checkFrontend(ctx, cfg.FrontendURL))
	report.Checks = append(report.Checks, r.checkHealthz(ctx, cfg.BackendURL))
	report.Checks = append(report.Checks, r.checkPublicScan(ctx, cfg.BackendURL, cfg.ScanDomain, cfg.MaxScanAge, now()))
	if cfg.RouteTXTName != "" {
		report.Checks = append(report.Checks, r.checkTXT(ctx, cfg.RouteTXTName, cfg.RouteTXTWanted))
	}
	if cfg.RouteCNAMEName != "" {
		report.Checks = append(report.Checks, r.checkCNAME(ctx, cfg.RouteCNAMEName, cfg.RouteCNAMEWant))
	}
	for _, check := range report.Checks {
		if check.Status == SyntheticStatusFail {
			report.Failed = true
			break
		}
	}
	return report
}

func (r SyntheticRunner) httpClient() HTTPDoer {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}

func (r SyntheticRunner) resolver() DNSResolver {
	if r.Resolver != nil {
		return r.Resolver
	}
	return net.DefaultResolver
}

func (r SyntheticRunner) checkFrontend(ctx context.Context, target string) SyntheticCheckResult {
	result, body := r.get(ctx, "frontend_homepage", target)
	_ = body
	if result.Status == SyntheticStatusFail {
		return result
	}
	return result
}

func (r SyntheticRunner) checkHealthz(ctx context.Context, backend string) SyntheticCheckResult {
	result, body := r.get(ctx, "backend_healthz", joinURL(backend, "/healthz"))
	if result.Status == SyntheticStatusFail {
		return result
	}
	if !strings.Contains(strings.ToLower(string(body)), "ok") {
		return failResult(result.Name, result.Target, result.LatencyMS, "schema", "healthz body did not contain ok")
	}
	return result
}

func (r SyntheticRunner) checkPublicScan(ctx context.Context, backend, domain string, maxAge time.Duration, now time.Time) SyntheticCheckResult {
	if domain == "" {
		domain = "kastro.is"
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	target := joinURL(backend, "/api/public/scan")
	u, err := url.Parse(target)
	if err == nil {
		q := u.Query()
		q.Set("domain", domain)
		u.RawQuery = q.Encode()
		target = u.String()
	}
	result, body := r.get(ctx, "public_scan", target)
	if result.Status == SyntheticStatusFail {
		return result
	}
	var payload struct {
		Domain    string `json:"domain"`
		CheckedAt string `json:"checked_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return failResult(result.Name, result.Target, result.LatencyMS, "schema", "scan response was not valid JSON")
	}
	if payload.Domain != domain || strings.TrimSpace(payload.CheckedAt) == "" {
		return failResult(result.Name, result.Target, result.LatencyMS, "schema", "scan response missing domain or checked_at")
	}
	checkedAt, err := time.Parse(time.RFC3339, payload.CheckedAt)
	if err != nil {
		return failResult(result.Name, result.Target, result.LatencyMS, "schema", "scan checked_at was not RFC3339")
	}
	if age := now.UTC().Sub(checkedAt.UTC()); age > maxAge || age < -1*time.Minute {
		return failResult(result.Name, result.Target, result.LatencyMS, "freshness", "scan checked_at was outside the freshness window")
	}
	return result
}

func (r SyntheticRunner) get(ctx context.Context, name, target string) (SyntheticCheckResult, []byte) {
	start := time.Now()
	result := SyntheticCheckResult{Name: name, Target: target}
	if strings.TrimSpace(target) == "" {
		return failResult(name, target, 0, "config", "target URL is required"), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return failResult(name, target, 0, "config", err.Error()), nil
	}
	resp, err := r.httpClient().Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return failResult(name, target, latency, classifyError(err), err.Error()), nil
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	result.LatencyMS = latency
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return failResult(name, target, latency, "http_status", fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)), body
	}
	result.Status = SyntheticStatusPass
	return result, body
}

func (r SyntheticRunner) checkTXT(ctx context.Context, name, want string) SyntheticCheckResult {
	start := time.Now()
	records, err := r.resolver().LookupTXT(ctx, name)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return failResult("route_txt", name, latency, classifyError(err), err.Error())
	}
	if want != "" {
		for _, record := range records {
			if strings.Contains(record, want) {
				return SyntheticCheckResult{Name: "route_txt", Status: SyntheticStatusPass, Target: name, LatencyMS: latency}
			}
		}
		return failResult("route_txt", name, latency, "dns_mismatch", "TXT record did not contain expected value")
	}
	if len(records) == 0 {
		return failResult("route_txt", name, latency, "dns_mismatch", "TXT record was not found")
	}
	return SyntheticCheckResult{Name: "route_txt", Status: SyntheticStatusPass, Target: name, LatencyMS: latency}
}

func (r SyntheticRunner) checkCNAME(ctx context.Context, name, want string) SyntheticCheckResult {
	start := time.Now()
	cname, err := r.resolver().LookupCNAME(ctx, name)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return failResult("route_cname", name, latency, classifyError(err), err.Error())
	}
	if want != "" && !strings.EqualFold(strings.TrimSuffix(cname, "."), strings.TrimSuffix(want, ".")) {
		return failResult("route_cname", name, latency, "dns_mismatch", "CNAME record did not match expected value")
	}
	return SyntheticCheckResult{Name: "route_cname", Status: SyntheticStatusPass, Target: name, LatencyMS: latency}
}

func failResult(name, target string, latency int64, class, message string) SyntheticCheckResult {
	return SyntheticCheckResult{Name: name, Status: SyntheticStatusFail, Target: target, LatencyMS: latency, ErrorClass: class, ErrorMessage: sanitizeErrorMessage(message)}
}

func classifyError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_lookup"
	}
	return "network"
}

func sanitizeErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "check failed"
	}
	for _, marker := range []string{"token=", "api_key=", "apikey=", "access_key=", "password=", "secret="} {
		for {
			lower := strings.ToLower(message)
			idx := strings.Index(lower, marker)
			if idx < 0 {
				break
			}
			end := idx + len(marker)
			for end < len(message) && message[end] != ' ' && message[end] != '&' && message[end] != '\n' && message[end] != '	' {
				end++
			}
			message = message[:idx] + "[REDACTED]" + message[end:]
		}
	}
	return message
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return path
	}
	return base + path
}
