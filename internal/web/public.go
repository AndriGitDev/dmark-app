package web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AndriGitDev/meil/internal/ops"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type publicRateLimiter struct {
	mu      sync.Mutex
	limit   int
	buckets map[string]publicRateBucket
}

type publicRateBucket struct {
	window time.Time
	count  int
}

func newPublicRateLimiter(limit int) *publicRateLimiter {
	if limit <= 0 {
		limit = 12
	}
	return &publicRateLimiter{
		limit:   limit,
		buckets: map[string]publicRateBucket{},
	}
}

func (l *publicRateLimiter) allow(key string, now time.Time) bool {
	if l == nil {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	window := now.UTC().Truncate(time.Minute)
	l.mu.Lock()
	defer l.mu.Unlock()
	for existingKey, existingBucket := range l.buckets {
		if existingBucket.window.Before(window.Add(-time.Minute)) {
			delete(l.buckets, existingKey)
		}
	}
	bucket := l.buckets[key]
	if bucket.window.IsZero() || !bucket.window.Equal(window) {
		l.buckets[key] = publicRateBucket{window: window, count: 1}
		return true
	}
	if bucket.count >= l.limit {
		return false
	}
	bucket.count++
	l.buckets[key] = bucket
	return true
}

func publicRateKey(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		if first = strings.TrimSpace(first); first != "" && net.ParseIP(first) != nil {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func publicScanDomain(r *http.Request) (string, error) {
	if r.Method == http.MethodGet {
		return scanner.NormalizeDomain(r.URL.Query().Get("domain"))
	}
	var payload struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("invalid JSON body")
	}
	return scanner.NormalizeDomain(payload.Domain)
}

func recordPublicScanOperationalEvent(r *http.Request, store OperationalEventStore, domain string, status int, completed bool) {
	if store == nil {
		return
	}
	kind := "public_scan.failed"
	severity := ops.SeverityWarn
	message := "public scan failed"
	metadata := map[string]any{
		"domain":     domain,
		"method":     r.Method,
		"path":       r.URL.Path,
		"status":     status,
		"request_id": RequestID(r.Context()),
	}
	if completed {
		kind = "public_scan.completed"
		severity = ops.SeverityInfo
		message = "public scan completed"
	} else {
		metadata["error_class"] = "scanner_error"
	}
	event, err := ops.NewEvent(kind, severity, message, time.Now().UTC())
	if err != nil {
		return
	}
	event.Actor = "public"
	event.Resource = "domain:" + domain
	event.Metadata = metadata
	_, _ = store.CreateOperationalEvent(r.Context(), event)
}

func normalizePublicEmail(raw string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(raw))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", fmt.Errorf("email is invalid")
	}
	return email, nil
}

func publicEmailDomain(email string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(strings.ToLower(email)), "@")
	if !ok {
		return ""
	}
	normalized, err := scanner.NormalizeDomain(domain)
	if err != nil {
		return ""
	}
	return normalized
}

func recordPublicAuditClaimOperationalEvent(r *http.Request, store OperationalEventStore, domain string, emailDomain string, status int, kind string, severity ops.Severity, message string, extra map[string]any) {
	if store == nil {
		return
	}
	metadata := map[string]any{
		"domain":       domain,
		"email_domain": emailDomain,
		"method":       r.Method,
		"path":         r.URL.Path,
		"status":       status,
		"request_id":   RequestID(r.Context()),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	event, err := ops.NewEvent(kind, severity, message, time.Now().UTC())
	if err != nil {
		return
	}
	event.Actor = "public"
	event.Resource = "domain:" + domain
	event.Metadata = metadata
	_, _ = store.CreateOperationalEvent(r.Context(), event)
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, origin := range allowedOrigins {
		origin = normalizeOrigin(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := normalizeOrigin(r.Header.Get("Origin"))
			if origin != "" {
				appendVary(w, "Origin")
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}
			if r.Method == http.MethodOptions {
				appendVary(w, "Access-Control-Request-Method")
				appendVary(w, "Access-Control-Request-Headers")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func normalizeOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return (&url.URL{Scheme: strings.ToLower(parsed.Scheme), Host: strings.ToLower(parsed.Host)}).String()
}

func appendVary(w http.ResponseWriter, value string) {
	existing := w.Header().Get("Vary")
	for _, part := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	if existing == "" {
		w.Header().Set("Vary", value)
		return
	}
	w.Header().Set("Vary", existing+", "+value)
}
