package ops

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type Event struct {
	ID         string
	Kind       string
	Severity   Severity
	Message    string
	Actor      string
	Resource   string
	Metadata   map[string]any
	OccurredAt time.Time
}

func NewEvent(kind string, severity Severity, message string, occurredAt time.Time) (Event, error) {
	kind = strings.TrimSpace(kind)
	message = strings.TrimSpace(message)
	if kind == "" {
		return Event{}, fmt.Errorf("operational event kind is required")
	}
	if message == "" {
		return Event{}, fmt.Errorf("operational event message is required")
	}
	if !validSeverity(severity) {
		return Event{}, fmt.Errorf("operational event severity is invalid")
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{Kind: kind, Severity: severity, Message: message, OccurredAt: occurredAt.UTC(), Metadata: map[string]any{}}, nil
}

func SanitizeMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if secretLikeKey(key) {
			sanitized[key] = "[REDACTED]"
			continue
		}
		sanitized[key] = sanitizeMetadataValue(value)
	}
	return sanitized
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return SanitizeMetadata(typed)
	case []any:
		sanitized := make([]any, len(typed))
		for i, item := range typed {
			sanitized[i] = sanitizeMetadataValue(item)
		}
		return sanitized
	case string:
		if suspiciousSecretValue(typed) {
			return "[REDACTED]"
		}
		return typed
	default:
		return value
	}
}

func validSeverity(severity Severity) bool {
	switch severity {
	case SeverityInfo, SeverityWarn, SeverityError:
		return true
	default:
		return false
	}
}

func secretLikeKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	for _, marker := range []string{"token", "secret", "password", "api_key", "apikey", "dsn", "connection_string"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func suspiciousSecretValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	normalized := strings.ToLower(trimmed)
	for _, marker := range []string{"password=", "passwd=", "pwd=", "token=", "secret=", "api_key=", "apikey="} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" && (parsed.User != nil || parsed.Host != "") {
		return true
	}
	return false
}
