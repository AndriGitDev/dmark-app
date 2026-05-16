package attribution

import (
	"sort"
	"time"
)

type EnrichmentEntry struct {
	IP           string
	ReverseDNS   string
	ASN          string
	CountryCode  string
	ProviderHint string
	Confidence   Confidence
	Evidence     []string
	RefreshedAt  time.Time
	ExpiresAt    time.Time
}

type CachePolicy struct {
	TTL           time.Duration
	RefreshBefore time.Duration
}

func DefaultCachePolicy() CachePolicy {
	return CachePolicy{TTL: 7 * 24 * time.Hour, RefreshBefore: 24 * time.Hour}
}

func (p CachePolicy) Normalize() CachePolicy {
	if p.TTL <= 0 {
		p.TTL = 7 * 24 * time.Hour
	}
	if p.RefreshBefore < 0 {
		p.RefreshBefore = 0
	}
	if p.RefreshBefore > p.TTL {
		p.RefreshBefore = p.TTL
	}
	return p
}

func NewEnrichmentEntry(signal Signal, result Result, countryCode string, now time.Time, policy CachePolicy) EnrichmentEntry {
	policy = policy.Normalize()
	return EnrichmentEntry{
		IP:           signal.IP,
		ReverseDNS:   signal.ReverseDNS,
		ASN:          signal.ASN,
		CountryCode:  countryCode,
		ProviderHint: result.Provider,
		Confidence:   result.Confidence,
		Evidence:     append([]string(nil), result.Evidence...),
		RefreshedAt:  now.UTC(),
		ExpiresAt:    now.UTC().Add(policy.TTL),
	}
}

func (e EnrichmentEntry) Expired(now time.Time) bool {
	return !e.ExpiresAt.IsZero() && !now.UTC().Before(e.ExpiresAt)
}

func (e EnrichmentEntry) RefreshDue(now time.Time, policy CachePolicy) bool {
	policy = policy.Normalize()
	if e.RefreshedAt.IsZero() || e.ExpiresAt.IsZero() {
		return true
	}
	if e.Expired(now) {
		return true
	}
	return !now.UTC().Before(e.ExpiresAt.Add(-policy.RefreshBefore))
}

func RefreshDue(entries []EnrichmentEntry, now time.Time, policy CachePolicy) []EnrichmentEntry {
	due := []EnrichmentEntry{}
	for _, entry := range entries {
		if entry.RefreshDue(now, policy) {
			due = append(due, entry)
		}
	}
	sort.Slice(due, func(i int, j int) bool {
		if due[i].ExpiresAt.Equal(due[j].ExpiresAt) {
			return due[i].IP < due[j].IP
		}
		return due[i].ExpiresAt.Before(due[j].ExpiresAt)
	})
	return due
}
