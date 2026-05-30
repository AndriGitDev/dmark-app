package bunnydns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecordNameUsesParentZoneForRouteSubdomain(t *testing.T) {
	got, err := RecordName("customer-example", "route.dmark.is", "dmark.is")
	if err != nil {
		t.Fatalf("RecordName returned error: %v", err)
	}
	if got != "customer-example.route" {
		t.Fatalf("record name = %q, want %q", got, "customer-example.route")
	}
}

func TestRecordNameUsesChildZoneWhenDelegationWorks(t *testing.T) {
	got, err := RecordName("customer-example", "route.dmark.is", "route.dmark.is")
	if err != nil {
		t.Fatalf("RecordName returned error: %v", err)
	}
	if got != "customer-example" {
		t.Fatalf("record name = %q, want %q", got, "customer-example")
	}
}

func TestRecordNameRejectsUnsafeLabel(t *testing.T) {
	if _, err := RecordName("bad label", "route.dmark.is", "dmark.is"); err == nil {
		t.Fatal("RecordName accepted a label containing whitespace")
	}
}

func TestUpsertTXTCreatesMissingRouteRecord(t *testing.T) {
	var requests []string
	var accessKeys []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		accessKeys = append(accessKeys, r.Header.Get("AccessKey"))
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(zoneResponse{Records: []Record{{ID: 10, Type: RecordTypeTXT, Name: "other.route", Value: "v=spf1 -all"}}})
		case http.MethodPut:
			var payload recordPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			if payload.Type == nil || *payload.Type != RecordTypeTXT || payload.Name != "customer-example.route" || payload.Value != "v=spf1 ip4:192.0.2.10 ~all" || payload.TTL != 300 {
				t.Fatalf("unexpected create payload: %+v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(Record{ID: 42, Type: RecordTypeTXT, Name: payload.Name, Value: payload.Value, TTL: payload.TTL})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, AccessKey: "secret-key", HTTPClient: server.Client()}
	result, err := client.UpsertTXT(context.Background(), 123, "customer-example.route", "v=spf1 ip4:192.0.2.10 ~all", 300)
	if err != nil {
		t.Fatalf("UpsertTXT returned error: %v", err)
	}
	if !result.Changed || result.RecordID != 42 || result.Action != ActionCreated {
		t.Fatalf("result = %+v, want created record 42", result)
	}
	if got := strings.Join(requests, ","); got != "GET /dnszone/123,PUT /dnszone/123/records" {
		t.Fatalf("requests = %s", got)
	}
	for _, key := range accessKeys {
		if key != "secret-key" {
			t.Fatalf("AccessKey header = %q", key)
		}
	}
}

func TestUpsertTXTDoesNothingWhenRouteRecordAlreadyMatches(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(zoneResponse{Records: []Record{{ID: 42, Type: RecordTypeTXT, Name: "customer-example.route", Value: "v=spf1 ip4:192.0.2.10 ~all", TTL: 300}}})
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, AccessKey: "secret-key", HTTPClient: server.Client()}
	result, err := client.UpsertTXT(context.Background(), 123, "customer-example.route", "v=spf1 ip4:192.0.2.10 ~all", 300)
	if err != nil {
		t.Fatalf("UpsertTXT returned error: %v", err)
	}
	if result.Changed || result.RecordID != 42 || result.Action != ActionUnchanged {
		t.Fatalf("result = %+v, want unchanged record 42", result)
	}
	if got := strings.Join(requests, ","); got != "GET /dnszone/123" {
		t.Fatalf("requests = %s", got)
	}
}

func TestUpsertTXTUpdatesExistingRouteRecordWhenValueDiffers(t *testing.T) {
	var updatePayload recordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(zoneResponse{Records: []Record{{ID: 42, Type: RecordTypeTXT, Name: "customer-example.route", Value: "v=spf1 ip4:198.51.100.2 ~all", TTL: 300}}})
		case http.MethodPost:
			if r.URL.Path != "/dnszone/123/records/42" {
				t.Fatalf("update path = %s", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
				t.Fatalf("decode update payload: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, AccessKey: "secret-key", HTTPClient: server.Client()}
	result, err := client.UpsertTXT(context.Background(), 123, "customer-example.route", "v=spf1 ip4:192.0.2.10 ~all", 300)
	if err != nil {
		t.Fatalf("UpsertTXT returned error: %v", err)
	}
	if !result.Changed || result.RecordID != 42 || result.Action != ActionUpdated {
		t.Fatalf("result = %+v, want updated record 42", result)
	}
	if updatePayload.Name != "customer-example.route" || updatePayload.Value != "v=spf1 ip4:192.0.2.10 ~all" || updatePayload.TTL != 300 {
		t.Fatalf("unexpected update payload: %+v", updatePayload)
	}
}

func TestUpsertTXTErrorRedactsAccessKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "backend mentions secret-key", http.StatusBadGateway)
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, AccessKey: "secret-key", HTTPClient: server.Client()}
	_, err := client.UpsertTXT(context.Background(), 123, "customer-example.route", "v=spf1 ~all", 300)
	if err == nil {
		t.Fatal("UpsertTXT returned nil error")
	}
	if strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("error leaked access key: %v", err)
	}
}
