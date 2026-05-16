package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/attribution"
)

func TestRefreshEnrichmentArgsKind(t *testing.T) {
	t.Parallel()

	if kind := (RefreshEnrichmentArgs{}).Kind(); kind != RefreshEnrichmentKind {
		t.Fatalf("Kind() = %q, want %q", kind, RefreshEnrichmentKind)
	}
}

func TestRefreshEnrichmentArgsIdempotencyKey(t *testing.T) {
	t.Parallel()

	args := RefreshEnrichmentArgs{
		IP:           "203.0.113.10",
		DomainID:     "domain-1",
		Reason:       "stale_cache",
		RequestedFor: time.Date(2026, 5, 16, 12, 34, 0, 0, time.UTC),
	}
	sameHour := args
	sameHour.RequestedFor = time.Date(2026, 5, 16, 12, 59, 0, 0, time.UTC)
	nextHour := args
	nextHour.RequestedFor = time.Date(2026, 5, 16, 13, 0, 0, 0, time.UTC)
	differentReason := args
	differentReason.Reason = "missing_cache"

	if args.IdempotencyKey() != sameHour.IdempotencyKey() {
		t.Fatalf("same refresh hour should share idempotency key")
	}
	if args.IdempotencyKey() == nextHour.IdempotencyKey() {
		t.Fatalf("different refresh hour should produce a different key")
	}
	if args.IdempotencyKey() == differentReason.IdempotencyKey() {
		t.Fatalf("different refresh reason should produce a different key")
	}
}

func TestRefreshEnrichmentArgsInsertOpts(t *testing.T) {
	t.Parallel()

	opts := RefreshEnrichmentArgs{}.InsertOpts()
	if opts.Queue != ScanQueue {
		t.Fatalf("Queue = %q, want %q", opts.Queue, ScanQueue)
	}
	if opts.MaxAttempts != DefaultRetryPolicy().MaxAttempts {
		t.Fatalf("MaxAttempts = %d, want %d", opts.MaxAttempts, DefaultRetryPolicy().MaxAttempts)
	}
	if !opts.UniqueOpts.ByArgs || !opts.UniqueOpts.ByQueue {
		t.Fatalf("unique opts = %+v, want by args and queue", opts.UniqueOpts)
	}
}

func TestRefreshEnrichmentWorkerStoresRefreshedEntry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	store := &fakeEnrichmentStore{}
	refresher := attribution.NewService(attribution.StaticEnricher{
		"203.0.113.10": {
			IP:          "203.0.113.10",
			ReverseDNS:  "mail.google.com",
			ASN:         "AS15169",
			CountryCode: "us",
		},
	}, attribution.DefaultClassifier(), attribution.DefaultCachePolicy())
	worker := &RefreshEnrichmentWorker{Store: store, Refresher: refresher}

	err := worker.Work(context.Background(), &river.Job[RefreshEnrichmentArgs]{
		Args: RefreshEnrichmentArgs{
			IP:                  "203.0.113.10",
			AuthenticatedDomain: "example.com",
			RequestedFor:        now,
		},
	})
	if err != nil {
		t.Fatalf("Work returned error: %v", err)
	}
	if store.entry.IP != "203.0.113.10" || store.entry.ProviderHint != "Google Workspace" {
		t.Fatalf("stored entry = %+v", store.entry)
	}
	if !store.entry.RefreshedAt.Equal(now) {
		t.Fatalf("RefreshedAt = %v, want %v", store.entry.RefreshedAt, now)
	}
}

type fakeEnrichmentStore struct {
	entry attribution.EnrichmentEntry
}

func (s *fakeEnrichmentStore) StoreEnrichmentCache(ctx context.Context, entry attribution.EnrichmentEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.entry = entry
	return nil
}
