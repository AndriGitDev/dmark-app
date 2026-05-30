package spfroutes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/bunnydns"
	"github.com/AndriGitDev/meil/internal/spfflatten"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusHealthy Status = "healthy"
	StatusFailed  Status = "failed"
	StatusBlocked Status = "blocked"
)

type Policy string

const (
	PolicySoftfail Policy = "softfail"
	PolicyFail     Policy = "fail"
	PolicyNeutral  Policy = "neutral"
)

type SourceKind string

const (
	SourceKindInclude SourceKind = "include"
	SourceKindIP4     SourceKind = "ip4"
	SourceKindIP6     SourceKind = "ip6"
)

type Route struct {
	ID                  string
	OrganizationID      string
	DomainID            string
	Domain              string
	RouteLabel          string
	RouteFQDN           string
	CustomerCNAMEName   string
	CustomerCNAMETarget string
	Enabled             bool
	MechanismPolicy     Policy
	BunnyRecordID       int64
	LastGeneratedSPF    string
	LastPublishedSPF    string
	LastSyncStatus      Status
	LastSyncError       string
	LastSyncedAt        time.Time
	Sources             []Source
	Evidence            []Evidence
}

type Source struct {
	ID      string
	Kind    SourceKind
	Value   string
	Label   string
	Enabled bool
}

type Evidence struct {
	SourceValue    string
	ResolvedDomain string
	Mechanism      string
	Kind           string
	Message        string
}

type SyncUpdate struct {
	GeneratedSPF  string
	PublishedSPF  string
	Status        Status
	Error         string
	BunnyRecordID int64
	Evidence      []Evidence
}

type Store interface {
	GetSPFRoute(ctx context.Context, routeID string) (Route, error)
	SaveSPFRouteSync(ctx context.Context, routeID string, update SyncUpdate) (Route, error)
}

type Publisher interface {
	UpsertTXT(ctx context.Context, zoneID int64, name, value string, ttl int) (bunnydns.UpsertResult, error)
}

type Config struct {
	BunnyZoneID int64
	DNSZone     string
	RouteZone   string
	TTL         int
	MaxLength   int
}

type Service struct {
	Store     Store
	Resolver  spfflatten.TXTResolver
	Publisher Publisher
	Config    Config
}

func (s Service) SyncRoute(ctx context.Context, routeID string) (Route, error) {
	if s.Store == nil {
		return Route{}, fmt.Errorf("SPF route store is required")
	}
	route, err := s.Store.GetSPFRoute(ctx, routeID)
	if err != nil {
		return Route{}, err
	}
	if !route.Enabled {
		return route, nil
	}

	resolution := spfflatten.Resolver{DNS: s.Resolver}.Resolve(ctx, enabledSources(route.Sources))
	evidence := evidenceFromResolution(resolution)
	if resolution.Blocked() {
		return s.save(ctx, route, SyncUpdate{Status: StatusBlocked, Error: joinResolutionErrors(resolution.Errors), PublishedSPF: route.LastPublishedSPF, BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}

	generated := spfflatten.Generate(resolution.Mechanisms, spfflatten.GenerateOptions{Terminal: terminalForPolicy(route.MechanismPolicy), MaxLength: s.Config.MaxLength})
	if generated.Blocked {
		evidence = append(evidence, Evidence{Kind: "error", Message: generated.Error})
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusBlocked, Error: generated.Error, BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}

	if strings.TrimSpace(route.LastPublishedSPF) == generated.SPF {
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusHealthy, BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}

	if s.Publisher == nil {
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusFailed, Error: "SPF route publisher is required", BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}
	if s.Config.BunnyZoneID <= 0 {
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusFailed, Error: "Bunny DNS zone ID is required", BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}

	recordName, err := bunnydns.RecordName(route.RouteLabel, s.Config.RouteZone, s.Config.DNSZone)
	if err != nil {
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusFailed, Error: err.Error(), BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}
	ttl := s.Config.TTL
	if ttl == 0 {
		ttl = 300
	}
	published, err := s.Publisher.UpsertTXT(ctx, s.Config.BunnyZoneID, recordName, generated.SPF, ttl)
	if err != nil {
		return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: route.LastPublishedSPF, Status: StatusFailed, Error: err.Error(), BunnyRecordID: route.BunnyRecordID, Evidence: evidence})
	}
	recordID := published.RecordID
	if recordID == 0 {
		recordID = route.BunnyRecordID
	}
	return s.save(ctx, route, SyncUpdate{GeneratedSPF: generated.SPF, PublishedSPF: generated.SPF, Status: StatusHealthy, BunnyRecordID: recordID, Evidence: evidence})
}

func (s Service) save(ctx context.Context, route Route, update SyncUpdate) (Route, error) {
	if update.BunnyRecordID == 0 {
		update.BunnyRecordID = route.BunnyRecordID
	}
	return s.Store.SaveSPFRouteSync(ctx, route.ID, update)
}

func enabledSources(sources []Source) []spfflatten.Source {
	result := make([]spfflatten.Source, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		kind := spfflatten.SourceKind(source.Kind)
		result = append(result, spfflatten.Source{Kind: kind, Value: source.Value, Label: source.Label})
	}
	return result
}

func evidenceFromResolution(result spfflatten.Result) []Evidence {
	evidence := make([]Evidence, 0, len(result.Evidence)+len(result.Errors))
	for _, item := range result.Evidence {
		evidence = append(evidence, Evidence{SourceValue: item.SourceValue, ResolvedDomain: item.ResolvedDomain, Mechanism: item.Mechanism, Kind: item.Kind, Message: item.Message})
	}
	for _, item := range result.Errors {
		evidence = append(evidence, Evidence{ResolvedDomain: item.Domain, Kind: "error", Message: item.Message})
	}
	return evidence
}

func joinResolutionErrors(errors []spfflatten.ResolutionError) string {
	parts := make([]string, 0, len(errors))
	for _, item := range errors {
		if item.Domain == "" {
			parts = append(parts, item.Message)
			continue
		}
		parts = append(parts, item.Domain+": "+item.Message)
	}
	return strings.Join(parts, "; ")
}

func terminalForPolicy(policy Policy) string {
	switch policy {
	case PolicyFail:
		return "-all"
	case PolicyNeutral:
		return "?all"
	default:
		return "~all"
	}
}
