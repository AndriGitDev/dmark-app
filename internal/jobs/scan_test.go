package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
)

func TestScanDomainArgsKind(t *testing.T) {
	t.Parallel()

	if kind := (ScanDomainArgs{}).Kind(); kind != ScanDomainKind {
		t.Fatalf("Kind() = %q, want %q", kind, ScanDomainKind)
	}
}

func TestScanDomainArgsIdempotencyKey(t *testing.T) {
	t.Parallel()

	args := ScanDomainArgs{
		UserID:       "user-1",
		DomainID:     "domain-1",
		Domain:       "example.com",
		ScheduledFor: time.Date(2026, 5, 16, 12, 34, 45, 0, time.UTC),
	}
	sameSlot := args
	sameSlot.ScheduledFor = time.Date(2026, 5, 16, 12, 34, 5, 0, time.UTC)
	nextSlot := args
	nextSlot.ScheduledFor = time.Date(2026, 5, 16, 12, 35, 0, 0, time.UTC)
	manual := args
	manual.Manual = true

	if args.IdempotencyKey() != sameSlot.IdempotencyKey() {
		t.Fatalf("same minute schedule slot should produce the same idempotency key")
	}
	if args.IdempotencyKey() == nextSlot.IdempotencyKey() {
		t.Fatalf("different schedule slot should produce a different idempotency key")
	}
	if args.IdempotencyKey() == manual.IdempotencyKey() {
		t.Fatalf("manual and scheduled scans should not share idempotency keys")
	}
}

func TestScanDomainArgsInsertOpts(t *testing.T) {
	t.Parallel()

	opts := ScanDomainArgs{}.InsertOpts()
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

func TestRetryPolicyDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{Backoff: time.Second}
	if delay := policy.Delay(1); delay != 0 {
		t.Fatalf("first attempt delay = %v, want 0", delay)
	}
	if delay := policy.Delay(4); delay != 4*time.Second {
		t.Fatalf("fourth attempt delay = %v, want 4s", delay)
	}
	if delay := policy.Delay(30); delay != 15*time.Minute {
		t.Fatalf("large attempt delay = %v, want 15m cap", delay)
	}
}

func TestScanDomainWorkerScansAndStoresResult(t *testing.T) {
	t.Parallel()

	store := &fakeScanStore{domain: domains.Domain{ID: "domain-1", Domain: "example.com"}}
	emailScanner := &fakeJobScanner{result: scanner.Result{Domain: "example.com", CheckedAt: time.Now()}}
	worker := &ScanDomainWorker{Store: store, Scanner: emailScanner}

	err := worker.Work(context.Background(), &river.Job[ScanDomainArgs]{
		Args: ScanDomainArgs{UserID: "user-1", DomainID: "domain-1"},
	})
	if err != nil {
		t.Fatalf("Work returned error: %v", err)
	}
	if emailScanner.scanned != "example.com" {
		t.Fatalf("scanner got %q, want example.com", emailScanner.scanned)
	}
	if store.checkDomainID != "domain-1" || store.result.Domain != "example.com" {
		t.Fatalf("stored check = %q/%+v", store.checkDomainID, store.result)
	}
}

func TestNewWorkersRegistersScanDomain(t *testing.T) {
	t.Parallel()

	refresher := attribution.NewService(attribution.StaticEnricher{}, attribution.DefaultClassifier(), attribution.DefaultCachePolicy())
	if _, err := NewWorkers(&fakeScanStore{}, &fakeJobScanner{}, refresher); err != nil {
		t.Fatalf("NewWorkers returned error: %v", err)
	}
}

type fakeScanStore struct {
	domain        domains.Domain
	checkDomainID string
	result        scanner.Result
}

func (s *fakeScanStore) GetDomain(ctx context.Context, userID string, domainID string) (domains.Domain, error) {
	if err := ctx.Err(); err != nil {
		return domains.Domain{}, err
	}
	if s.domain.ID == "" {
		s.domain = domains.Domain{ID: domainID, Domain: "example.com"}
	}
	return s.domain, nil
}

func (s *fakeScanStore) CreateCheck(ctx context.Context, domainID string, result scanner.Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.checkDomainID = domainID
	s.result = result
	return nil
}

func (s *fakeScanStore) StoreEnrichmentCache(ctx context.Context, entry attribution.EnrichmentEntry) error {
	return ctx.Err()
}

type fakeJobScanner struct {
	scanned string
	result  scanner.Result
}

func (s *fakeJobScanner) Scan(ctx context.Context, domain string) (scanner.Result, error) {
	if err := ctx.Err(); err != nil {
		return scanner.Result{}, err
	}
	s.scanned = domain
	if s.result.Domain == "" {
		s.result.Domain = domain
	}
	return s.result, nil
}
