package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/spfroutes"
)

const SyncSPFRouteKind = "sync_spf_route"

type SyncSPFRouteArgs struct {
	RouteID      string    `json:"route_id" river:"unique"`
	ScheduledFor time.Time `json:"scheduled_for" river:"unique"`
	Manual       bool      `json:"manual" river:"unique"`
}

func (SyncSPFRouteArgs) Kind() string {
	return SyncSPFRouteKind
}

func (args SyncSPFRouteArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: DefaultRetryPolicy().MaxAttempts,
		Queue:       ScanQueue,
		UniqueOpts: river.UniqueOpts{
			ByArgs:  true,
			ByQueue: true,
		},
	}
}

func (args SyncSPFRouteArgs) IdempotencyKey() string {
	slot := args.ScheduledFor.UTC().Truncate(time.Minute).Format(time.RFC3339)
	if args.Manual {
		slot = "manual:" + slot
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s", args.RouteID, slot)))
	return hex.EncodeToString(sum[:])
}

type SPFRouteSyncer interface {
	SyncRoute(ctx context.Context, routeID string) (spfroutes.Route, error)
}

type SyncSPFRouteWorker struct {
	river.WorkerDefaults[SyncSPFRouteArgs]

	Syncer SPFRouteSyncer
	Policy RetryPolicy
}

func (w *SyncSPFRouteWorker) Work(ctx context.Context, job *river.Job[SyncSPFRouteArgs]) error {
	if w.Syncer == nil {
		return fmt.Errorf("SPF route syncer is not configured")
	}
	if job.Args.RouteID == "" {
		return fmt.Errorf("SPF route ID is required")
	}
	_, err := w.Syncer.SyncRoute(ctx, job.Args.RouteID)
	return err
}

func (w *SyncSPFRouteWorker) NextRetry(job *river.Job[SyncSPFRouteArgs]) time.Time {
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

func (w *SyncSPFRouteWorker) Timeout(*river.Job[SyncSPFRouteArgs]) time.Duration {
	return 2 * time.Minute
}
