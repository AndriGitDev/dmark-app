package bunnydns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.bunny.net"
	RecordTypeTXT  = 3
)

type Action string

const (
	ActionCreated   Action = "created"
	ActionUpdated   Action = "updated"
	ActionUnchanged Action = "unchanged"
)

type Client struct {
	BaseURL    string
	AccessKey  string
	HTTPClient *http.Client
}

type Record struct {
	ID    int64  `json:"Id"`
	Type  int    `json:"Type"`
	Name  string `json:"Name"`
	Value string `json:"Value"`
	TTL   int    `json:"Ttl"`
}

type UpsertResult struct {
	RecordID int64
	Changed  bool
	Action   Action
}

type zoneResponse struct {
	Records []Record `json:"Records"`
}

type recordPayload struct {
	Type  *int   `json:"Type,omitempty"`
	TTL   int    `json:"Ttl"`
	Value string `json:"Value"`
	Name  string `json:"Name"`
}

// RecordName returns the Bunny-relative DNS record name for a route label.
// When routeZone is a child of dnsZone, Bunny expects the child suffix in Name
// (for example customer.route in the dmark.is zone). When both zones are the
// same, only the customer label is used.
func RecordName(label, routeZone, dnsZone string) (string, error) {
	label = normalizeNamePart(label)
	routeZone = normalizeZone(routeZone)
	dnsZone = normalizeZone(dnsZone)
	if err := validateRecordNamePart(label); err != nil {
		return "", fmt.Errorf("route label: %w", err)
	}
	if routeZone == "" {
		return "", fmt.Errorf("route zone is required")
	}
	if dnsZone == "" {
		return "", fmt.Errorf("DNS zone is required")
	}
	if routeZone == dnsZone {
		return label, nil
	}
	suffix := "." + dnsZone
	if !strings.HasSuffix(routeZone, suffix) {
		return "", fmt.Errorf("route zone %q is not inside DNS zone %q", routeZone, dnsZone)
	}
	relativeRoute := strings.TrimSuffix(routeZone, suffix)
	if err := validateRecordNamePart(relativeRoute); err != nil {
		return "", fmt.Errorf("relative route zone: %w", err)
	}
	return label + "." + relativeRoute, nil
}

func (c Client) UpsertTXT(ctx context.Context, zoneID int64, name, value string, ttl int) (UpsertResult, error) {
	if zoneID <= 0 {
		return UpsertResult{}, fmt.Errorf("Bunny DNS zone ID is required")
	}
	name = normalizeNamePart(name)
	if err := validateRecordNamePart(name); err != nil {
		return UpsertResult{}, fmt.Errorf("record name: %w", err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return UpsertResult{}, fmt.Errorf("TXT value is required")
	}
	if ttl <= 0 {
		return UpsertResult{}, fmt.Errorf("TTL must be greater than zero")
	}

	records, err := c.ListRecords(ctx, zoneID)
	if err != nil {
		return UpsertResult{}, err
	}
	for _, record := range records {
		if record.Type != RecordTypeTXT || normalizeNamePart(record.Name) != name {
			continue
		}
		if strings.TrimSpace(record.Value) == value && record.TTL == ttl {
			return UpsertResult{RecordID: record.ID, Action: ActionUnchanged}, nil
		}
		updated, err := c.updateTXT(ctx, zoneID, record.ID, name, value, ttl)
		if err != nil {
			return UpsertResult{}, err
		}
		updatedID := updated.ID
		if updatedID == 0 {
			updatedID = record.ID
		}
		return UpsertResult{RecordID: updatedID, Changed: true, Action: ActionUpdated}, nil
	}

	created, err := c.createTXT(ctx, zoneID, name, value, ttl)
	if err != nil {
		return UpsertResult{}, err
	}
	return UpsertResult{RecordID: created.ID, Changed: true, Action: ActionCreated}, nil
}

func (c Client) ListRecords(ctx context.Context, zoneID int64) ([]Record, error) {
	var zone zoneResponse
	if err := c.do(ctx, http.MethodGet, "/dnszone/"+strconv.FormatInt(zoneID, 10), nil, &zone, http.StatusOK); err != nil {
		return nil, err
	}
	return zone.Records, nil
}

func (c Client) createTXT(ctx context.Context, zoneID int64, name, value string, ttl int) (Record, error) {
	typ := RecordTypeTXT
	payload := recordPayload{Type: &typ, Name: name, Value: value, TTL: ttl}
	var record Record
	err := c.do(ctx, http.MethodPut, "/dnszone/"+strconv.FormatInt(zoneID, 10)+"/records", payload, &record, http.StatusCreated)
	return record, err
}

func (c Client) updateTXT(ctx context.Context, zoneID, recordID int64, name, value string, ttl int) (Record, error) {
	typ := RecordTypeTXT
	payload := recordPayload{Type: &typ, Name: name, Value: value, TTL: ttl}
	var record Record
	err := c.do(ctx, http.MethodPost, "/dnszone/"+strconv.FormatInt(zoneID, 10)+"/records/"+strconv.FormatInt(recordID, 10), payload, &record, http.StatusNoContent, http.StatusOK)
	return record, err
}

func (c Client) do(ctx context.Context, method, path string, body any, out any, wantStatuses ...int) error {
	if strings.TrimSpace(c.AccessKey) == "" {
		return fmt.Errorf("Bunny API access key is required")
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Bunny DNS request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build Bunny DNS request: %w", err)
	}
	request.Header.Set("AccessKey", c.AccessKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return redact(c.AccessKey, fmt.Errorf("call Bunny DNS API: %w", err))
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Bunny DNS response: %w", err)
	}
	if !acceptedStatus(response.StatusCode, wantStatuses) {
		return redact(c.AccessKey, fmt.Errorf("Bunny DNS API %s %s returned %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(data))))
	}
	if out != nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode Bunny DNS response: %w", err)
		}
	}
	return nil
}

func acceptedStatus(got int, want []int) bool {
	for _, status := range want {
		if got == status {
			return true
		}
	}
	return false
}

func normalizeZone(zone string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(zone), "."))
}

func normalizeNamePart(name string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
}

func validateRecordNamePart(name string) error {
	if name == "" {
		return fmt.Errorf("is required")
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return fmt.Errorf("must not contain whitespace")
	}
	for _, part := range strings.Split(name, ".") {
		if part == "" {
			return fmt.Errorf("must not contain empty labels")
		}
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("label %q must not start or end with hyphen", part)
		}
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				continue
			}
			return fmt.Errorf("label %q contains invalid character %q", part, r)
		}
	}
	return nil
}

func redact(secret string, err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if secret != "" {
		text = strings.ReplaceAll(text, secret, "[REDACTED]")
	}
	return fmt.Errorf("%s", text)
}
