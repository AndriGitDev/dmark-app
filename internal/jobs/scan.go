package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
)

const (
	ScanDomainKind = "scan_domain"
	ScanQueue      = "default"
)

type ScanDomainArgs struct {
	UserID       string    `json:"user_id" river:"unique"`
	DomainID     string    `json:"domain_id" river:"unique"`
	Domain       string    `json:"domain"`
	ScheduledFor time.Time `json:"scheduled_for" river:"unique"`
	Manual       bool      `json:"manual" river:"unique"`
}

func (ScanDomainArgs) Kind() string {
	return ScanDomainKind
}

func (args ScanDomainArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: DefaultRetryPolicy().MaxAttempts,
		Queue:       ScanQueue,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByQueue: true,
		},
	}
}

func (args ScanDomainArgs) IdempotencyKey() string {
	slot := args.ScheduledFor.UTC().Truncate(time.Minute).Format(time.RFC3339)
	if args.Manual {
		slot = "manual:" + slot
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s", args.UserID, args.DomainID, args.Domain, slot)))
	return hex.EncodeToString(sum[:])
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 5,
		Backoff:     30 * time.Second,
	}
}

func (policy RetryPolicy) Delay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	if policy.Backoff <= 0 {
		policy.Backoff = 30 * time.Second
	}
	delay := policy.Backoff
	for i := 2; i < attempt; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		return 15 * time.Minute
	}
	return delay
}

type ScanDomainStore interface {
	GetDomain(ctx context.Context, userID string, domainID string) (domains.Domain, error)
	CreateCheck(ctx context.Context, domainID string, result scanner.Result) error
}

type ScanDomainScanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

type ScanDomainWorker struct {
	river.WorkerDefaults[ScanDomainArgs]

	Store   ScanDomainStore
	Scanner ScanDomainScanner
	Policy  RetryPolicy
}

func (w *ScanDomainWorker) Work(ctx context.Context, job *river.Job[ScanDomainArgs]) error {
	if w.Store == nil {
		return fmt.Errorf("scan domain worker store is not configured")
	}
	if w.Scanner == nil {
		return fmt.Errorf("scan domain worker scanner is not configured")
	}
	saved, err := w.Store.GetDomain(ctx, job.Args.UserID, job.Args.DomainID)
	if err != nil {
		return err
	}
	result, err := w.Scanner.Scan(ctx, saved.Domain)
	if err != nil {
		return err
	}
	return w.Store.CreateCheck(ctx, saved.ID, result)
}

func (w *ScanDomainWorker) NextRetry(job *river.Job[ScanDomainArgs]) time.Time {
	policy := w.Policy
	if policy.MaxAttempts == 0 && policy.Backoff == 0 {
		policy = DefaultRetryPolicy()
	}
	delay := policy.Delay(job.Attempt + 1)
	if delay == 0 {
		return time.Time{}
	}
	return time.Now().UTC().Add(delay)
}

func (w *ScanDomainWorker) Timeout(*river.Job[ScanDomainArgs]) time.Duration {
	return 2 * time.Minute
}

func NewWorkers(store interface {
	ScanDomainStore
	EnrichmentCacheStore
}, emailScanner ScanDomainScanner, refresher EnrichmentRefresher, spfSyncers ...SPFRouteSyncer,
) (*river.Workers, error) {
	workers := river.NewWorkers()
	err := river.AddWorkerSafely(workers, &ScanDomainWorker{
		Store:   store,
		Scanner: emailScanner,
		Policy:  DefaultRetryPolicy(),
	})
	if err != nil {
		return nil, err
	}
	err = river.AddWorkerSafely(workers, &RefreshEnrichmentWorker{
		Store:     store,
		Refresher: refresher,
		Policy:    DefaultRetryPolicy(),
	})
	if err != nil {
		return nil, err
	}
	if len(spfSyncers) > 0 && spfSyncers[0] != nil {
		var events SPFRouteOperationalEventStore
		if operationalEvents, ok := store.(SPFRouteOperationalEventStore); ok {
			events = operationalEvents
		}
		err = river.AddWorkerSafely(workers, &SyncSPFRouteWorker{
			Syncer: spfSyncers[0],
			Events: events,
			Policy: DefaultRetryPolicy(),
		})
		if err != nil {
			return nil, err
		}
	}
	return workers, nil
}
