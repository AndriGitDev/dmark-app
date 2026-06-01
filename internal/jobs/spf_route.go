package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/ops"
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

type SPFRouteOperationalEventStore interface {
	CreateOperationalEvent(ctx context.Context, event ops.Event) (ops.Event, error)
}

type SyncSPFRouteWorker struct {
	river.WorkerDefaults[SyncSPFRouteArgs]

	Syncer SPFRouteSyncer
	Events SPFRouteOperationalEventStore
	Policy RetryPolicy
}

func (w *SyncSPFRouteWorker) Work(ctx context.Context, job *river.Job[SyncSPFRouteArgs]) error {
	if job.Args.RouteID == "" {
		return fmt.Errorf("SPF route ID is required")
	}
	if w.Syncer == nil {
		err := fmt.Errorf("SPF route syncer is not configured")
		w.recordOperationalEvent(ctx, job.Args, spfroutes.Route{}, err)
		return err
	}
	route, err := w.Syncer.SyncRoute(ctx, job.Args.RouteID)
	w.recordOperationalEvent(ctx, job.Args, route, err)
	return err
}

func (w *SyncSPFRouteWorker) recordOperationalEvent(ctx context.Context, args SyncSPFRouteArgs, route spfroutes.Route, syncErr error) {
	if w.Events == nil || args.RouteID == "" {
		return
	}
	kind, severity, message, ok := spfRouteOperationalEventShape(route.LastSyncStatus, syncErr)
	if !ok {
		return
	}
	event, err := ops.NewEvent(kind, severity, message, time.Now().UTC())
	if err != nil {
		return
	}
	event.Actor = "worker:" + SyncSPFRouteKind
	event.Resource = "spf_route:" + args.RouteID
	metadata := map[string]any{
		"route_id": args.RouteID,
		"manual":   args.Manual,
	}
	if !args.ScheduledFor.IsZero() {
		metadata["scheduled_for"] = args.ScheduledFor.UTC().Format(time.RFC3339)
	}
	if route.LastSyncStatus != "" {
		metadata["status"] = string(route.LastSyncStatus)
	}
	if route.ID != "" && route.ID != args.RouteID {
		metadata["result_route_id"] = route.ID
	}
	if route.OrganizationID != "" {
		metadata["organization_id"] = route.OrganizationID
	}
	if route.DomainID != "" {
		metadata["domain_id"] = route.DomainID
	}
	if syncErr != nil {
		metadata["error_class"] = "sync_error"
	}
	event.Metadata = ops.SanitizeMetadata(metadata)
	_, _ = w.Events.CreateOperationalEvent(ctx, event)
}

func spfRouteOperationalEventShape(status spfroutes.Status, syncErr error) (string, ops.Severity, string, bool) {
	if syncErr != nil {
		return "spf_route.publish_failed", ops.SeverityWarn, "SPF route publish failed", true
	}
	switch status {
	case spfroutes.StatusHealthy:
		return "spf_route.publish_succeeded", ops.SeverityInfo, "SPF route publish succeeded", true
	case spfroutes.StatusBlocked:
		return "spf_route.publish_blocked", ops.SeverityWarn, "SPF route publish blocked", true
	case spfroutes.StatusFailed:
		return "spf_route.publish_failed", ops.SeverityWarn, "SPF route publish failed", true
	default:
		return "", "", "", false
	}
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
