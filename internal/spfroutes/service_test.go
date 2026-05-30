package spfroutes

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/AndriGitDev/meil/internal/bunnydns"
	"github.com/AndriGitDev/meil/internal/spfflatten"
)

func TestSyncRoutePublishesGeneratedSPFAndPersistsEvidence(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{route: Route{
		ID:                  "route-1",
		DomainID:            "domain-1",
		Domain:              "example.com",
		RouteLabel:          "example-com",
		RouteFQDN:           "example-com.route.dmark.is",
		CustomerCNAMETarget: "example-com.route.dmark.is",
		Enabled:             true,
		MechanismPolicy:     PolicySoftfail,
		Sources: []Source{
			{Kind: SourceKindInclude, Value: "_spf.mailer.example", Label: "Mailer", Enabled: true},
			{Kind: SourceKindIP4, Value: "198.51.100.5", Label: "Static gateway", Enabled: true},
		},
	}}
	publisher := &fakePublisher{result: bunnydns.UpsertResult{RecordID: 42, Changed: true, Action: bunnydns.ActionCreated}}
	resolver := fakeResolver{txt: map[string][]string{
		"_spf.mailer.example": {"v=spf1 ip4:192.0.2.10 include:nested.example ~all"},
		"nested.example":      {"v=spf1 ip6:2001:db8::/32 ~all"},
	}}

	result, err := Service{Store: store, Resolver: resolver, Publisher: publisher, Config: Config{
		BunnyZoneID: 801471,
		DNSZone:     "dmark.is",
		RouteZone:   "route.dmark.is",
		TTL:         300,
	}}.SyncRoute(ctx, "route-1")
	if err != nil {
		t.Fatalf("SyncRoute returned error: %v", err)
	}
	if result.LastSyncStatus != StatusHealthy || result.LastGeneratedSPF == "" || result.LastPublishedSPF != result.LastGeneratedSPF || result.BunnyRecordID != 42 {
		t.Fatalf("result = %+v, want healthy published record 42", result)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("publisher calls = %d, want 1", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.zoneID != 801471 || call.name != "example-com.route" || call.ttl != 300 || call.value != "v=spf1 ip4:192.0.2.10 ip4:198.51.100.5 ip6:2001:db8::/32 ~all" {
		t.Fatalf("publisher call = %+v", call)
	}
	if got := store.saved.Status; got != StatusHealthy {
		t.Fatalf("saved status = %q, want healthy", got)
	}
	if len(store.saved.Evidence) < 3 {
		t.Fatalf("saved evidence length = %d, want resolver evidence", len(store.saved.Evidence))
	}
}

func TestSyncRouteSkipsBunnyWhenPublishedSPFIsAlreadyCurrent(t *testing.T) {
	ctx := context.Background()
	currentSPF := "v=spf1 ip4:192.0.2.10 ~all"
	store := &fakeStore{route: Route{
		ID: "route-1", RouteLabel: "example-com", RouteFQDN: "example-com.route.dmark.is", Enabled: true,
		MechanismPolicy: PolicySoftfail, LastPublishedSPF: currentSPF,
		Sources: []Source{{Kind: SourceKindIP4, Value: "192.0.2.10", Enabled: true}},
	}}
	publisher := &fakePublisher{}

	result, err := Service{Store: store, Publisher: publisher, Config: Config{BunnyZoneID: 801471, DNSZone: "dmark.is", RouteZone: "route.dmark.is", TTL: 300}}.SyncRoute(ctx, "route-1")
	if err != nil {
		t.Fatalf("SyncRoute returned error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("publisher was called for unchanged SPF: %+v", publisher.calls)
	}
	if result.LastSyncStatus != StatusHealthy || result.LastPublishedSPF != currentSPF || !reflect.DeepEqual(store.saved.PublishedSPF, currentSPF) {
		t.Fatalf("result/saved = %+v / %+v, want unchanged healthy", result, store.saved)
	}
}

func TestSyncRouteBlocksAndDoesNotPublishWhenResolutionFails(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{route: Route{
		ID: "route-1", RouteLabel: "example-com", Enabled: true, MechanismPolicy: PolicySoftfail, LastPublishedSPF: "v=spf1 ip4:203.0.113.10 ~all", BunnyRecordID: 42,
		Sources: []Source{{Kind: SourceKindInclude, Value: "broken.example", Enabled: true}},
	}}
	publisher := &fakePublisher{}
	resolver := fakeResolver{err: errors.New("dns timeout")}

	result, err := Service{Store: store, Resolver: resolver, Publisher: publisher, Config: Config{BunnyZoneID: 801471, DNSZone: "dmark.is", RouteZone: "route.dmark.is", TTL: 300}}.SyncRoute(ctx, "route-1")
	if err != nil {
		t.Fatalf("SyncRoute returned error: %v", err)
	}
	if result.LastSyncStatus != StatusBlocked || result.LastSyncError == "" {
		t.Fatalf("result = %+v, want blocked with error", result)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("publisher was called for blocked route: %+v", publisher.calls)
	}
	if store.saved.Status != StatusBlocked || store.saved.PublishedSPF != "v=spf1 ip4:203.0.113.10 ~all" || store.saved.BunnyRecordID != 42 || len(store.saved.Evidence) == 0 {
		t.Fatalf("saved sync = %+v, want blocked evidence with prior published SPF preserved", store.saved)
	}
}

type fakeStore struct {
	route Route
	saved SyncUpdate
}

func (s *fakeStore) GetSPFRoute(ctx context.Context, routeID string) (Route, error) {
	return s.route, nil
}

func (s *fakeStore) SaveSPFRouteSync(ctx context.Context, routeID string, update SyncUpdate) (Route, error) {
	s.saved = update
	s.route.LastSyncStatus = update.Status
	s.route.LastSyncError = update.Error
	s.route.LastGeneratedSPF = update.GeneratedSPF
	s.route.LastPublishedSPF = update.PublishedSPF
	s.route.BunnyRecordID = update.BunnyRecordID
	return s.route, nil
}

type fakePublisher struct {
	calls  []publishCall
	result bunnydns.UpsertResult
	err    error
}

type publishCall struct {
	zoneID int64
	name   string
	value  string
	ttl    int
}

func (p *fakePublisher) UpsertTXT(ctx context.Context, zoneID int64, name, value string, ttl int) (bunnydns.UpsertResult, error) {
	p.calls = append(p.calls, publishCall{zoneID: zoneID, name: name, value: value, ttl: ttl})
	return p.result, p.err
}

type fakeResolver struct {
	txt map[string][]string
	err error
}

func (r fakeResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.txt[domain], nil
}

var _ spfflatten.TXTResolver = fakeResolver{}
