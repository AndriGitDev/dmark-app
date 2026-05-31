package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

func TestSyncSPFRouteArgsAreUniquePerRouteAndScheduleSlot(t *testing.T) {
	scheduled := time.Date(2026, 5, 31, 2, 10, 59, 0, time.UTC)
	args := SyncSPFRouteArgs{RouteID: "route-1", ScheduledFor: scheduled}

	if got := args.Kind(); got != SyncSPFRouteKind {
		t.Fatalf("kind = %q, want %q", got, SyncSPFRouteKind)
	}
	if got := args.InsertOpts().Queue; got != ScanQueue {
		t.Fatalf("queue = %q, want %q", got, ScanQueue)
	}
	if args.IdempotencyKey() == (SyncSPFRouteArgs{RouteID: "route-1", ScheduledFor: scheduled.Add(time.Minute)}).IdempotencyKey() {
		t.Fatal("idempotency key did not change for a different schedule minute")
	}
	if args.IdempotencyKey() == (SyncSPFRouteArgs{RouteID: "route-2", ScheduledFor: scheduled}).IdempotencyKey() {
		t.Fatal("idempotency key did not change for a different route")
	}
	if args.IdempotencyKey() != (SyncSPFRouteArgs{RouteID: "route-1", ScheduledFor: scheduled.Truncate(time.Minute)}).IdempotencyKey() {
		t.Fatal("idempotency key should be stable within the same minute slot")
	}
}

func TestSyncSPFRouteWorkerCallsRouteService(t *testing.T) {
	ctx := context.Background()
	syncer := &fakeSPFRouteSyncer{route: spfroutes.Route{ID: "route-1", LastSyncStatus: spfroutes.StatusHealthy}}
	worker := &SyncSPFRouteWorker{Syncer: syncer, Policy: RetryPolicy{MaxAttempts: 3, Backoff: time.Second}}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1", Manual: true}}

	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("Work returned error: %v", err)
	}
	if len(syncer.calls) != 1 || syncer.calls[0] != "route-1" {
		t.Fatalf("sync calls = %+v, want route-1", syncer.calls)
	}
}

func TestSyncSPFRouteWorkerReturnsSyncFailuresForRiverRetry(t *testing.T) {
	expected := errors.New("bunny temporarily unavailable")
	worker := &SyncSPFRouteWorker{Syncer: &fakeSPFRouteSyncer{err: expected}}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1"}}

	if err := worker.Work(context.Background(), job); !errors.Is(err, expected) {
		t.Fatalf("Work error = %v, want %v", err, expected)
	}
}

func TestNewWorkersRegistersSPFRouteWorkerWhenSyncerProvided(t *testing.T) {
	refresher := attribution.NewService(attribution.StaticEnricher{}, attribution.DefaultClassifier(), attribution.DefaultCachePolicy())
	workers, err := NewWorkers(&fakeScanStore{}, &fakeJobScanner{}, refresher, &fakeSPFRouteSyncer{})
	if err != nil {
		t.Fatalf("NewWorkers returned error: %v", err)
	}
	if workers == nil {
		t.Fatal("workers is nil")
	}
}

type fakeSPFRouteSyncer struct {
	calls []string
	route spfroutes.Route
	err   error
}

func (s *fakeSPFRouteSyncer) SyncRoute(ctx context.Context, routeID string) (spfroutes.Route, error) {
	s.calls = append(s.calls, routeID)
	return s.route, s.err
}
