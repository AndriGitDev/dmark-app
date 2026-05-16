package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/attribution"
)

const RefreshEnrichmentKind = "refresh_enrichment"

type RefreshEnrichmentArgs struct {
	IP                  string    `json:"ip" river:"unique"`
	DomainID            string    `json:"domain_id,omitempty" river:"unique"`
	AuthenticatedDomain string    `json:"authenticated_domain,omitempty" river:"unique"`
	Reason              string    `json:"reason" river:"unique"`
	RequestedFor        time.Time `json:"requested_for" river:"unique"`
}

func (RefreshEnrichmentArgs) Kind() string {
	return RefreshEnrichmentKind
}

func (args RefreshEnrichmentArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: DefaultRetryPolicy().MaxAttempts,
		Queue:       ScanQueue,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByQueue: true,
		},
	}
}

func (args RefreshEnrichmentArgs) IdempotencyKey() string {
	slot := args.RequestedFor.UTC().Truncate(time.Hour).Format(time.RFC3339)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", args.IP, args.DomainID, args.AuthenticatedDomain, args.Reason, slot)))
	return hex.EncodeToString(sum[:])
}

type EnrichmentRefresher interface {
	Refresh(ctx context.Context, ip string, authenticatedDomain string, now time.Time) (attribution.EnrichmentEntry, error)
}

type EnrichmentCacheStore interface {
	StoreEnrichmentCache(ctx context.Context, entry attribution.EnrichmentEntry) error
}

type RefreshEnrichmentWorker struct {
	river.WorkerDefaults[RefreshEnrichmentArgs]

	Refresher EnrichmentRefresher
	Store     EnrichmentCacheStore
	Policy    RetryPolicy
}

func (w *RefreshEnrichmentWorker) Work(ctx context.Context, job *river.Job[RefreshEnrichmentArgs]) error {
	if w.Refresher == nil {
		return fmt.Errorf("enrichment refresher is not configured")
	}
	if w.Store == nil {
		return fmt.Errorf("enrichment cache store is not configured")
	}
	now := job.Args.RequestedFor
	if now.IsZero() {
		now = time.Now()
	}
	entry, err := w.Refresher.Refresh(ctx, job.Args.IP, job.Args.AuthenticatedDomain, now)
	if err != nil {
		return err
	}
	return w.Store.StoreEnrichmentCache(ctx, entry)
}

func (w *RefreshEnrichmentWorker) NextRetry(job *river.Job[RefreshEnrichmentArgs]) time.Time {
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

func (w *RefreshEnrichmentWorker) Timeout(*river.Job[RefreshEnrichmentArgs]) time.Duration {
	return 30 * time.Second
}
