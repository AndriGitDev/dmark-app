package attribution

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type EnrichmentLookup struct {
	IP          string
	ReverseDNS  string
	ASN         string
	CountryCode string
}

type Enricher interface {
	Lookup(ctx context.Context, ip string) (EnrichmentLookup, error)
}

type Service struct {
	enricher   Enricher
	classifier Classifier
	policy     CachePolicy
}

func NewService(enricher Enricher, classifier Classifier, policy CachePolicy) Service {
	if classifier.rules == nil {
		classifier = DefaultClassifier()
	}
	return Service{enricher: enricher, classifier: classifier, policy: policy.Normalize()}
}

func (s Service) Refresh(ctx context.Context, ip string, authenticatedDomain string, now time.Time) (EnrichmentEntry, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return EnrichmentEntry{}, fmt.Errorf("source IP is required")
	}
	if s.enricher == nil {
		return EnrichmentEntry{}, fmt.Errorf("enricher is not configured")
	}
	lookup, err := s.enricher.Lookup(ctx, ip)
	if err != nil {
		return EnrichmentEntry{}, err
	}
	if lookup.IP == "" {
		lookup.IP = ip
	}
	signal := Signal{
		IP:                  lookup.IP,
		ReverseDNS:          lookup.ReverseDNS,
		ASN:                 lookup.ASN,
		AuthenticatedDomain: strings.TrimSpace(authenticatedDomain),
	}
	result := s.classifier.Classify(signal)
	return NewEnrichmentEntry(signal, result, strings.ToUpper(strings.TrimSpace(lookup.CountryCode)), now, s.policy), nil
}

type StaticEnricher map[string]EnrichmentLookup

func (e StaticEnricher) Lookup(ctx context.Context, ip string) (EnrichmentLookup, error) {
	if err := ctx.Err(); err != nil {
		return EnrichmentLookup{}, err
	}
	lookup, ok := e[strings.TrimSpace(ip)]
	if !ok {
		return EnrichmentLookup{IP: strings.TrimSpace(ip)}, nil
	}
	return lookup, nil
}
