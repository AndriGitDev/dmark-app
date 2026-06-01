package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/ops"
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

func TestSyncSPFRouteWorkerRecordsSuccessfulOperationalEvent(t *testing.T) {
	ctx := context.Background()
	scheduled := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	events := &fakeOperationalEventStore{}
	syncer := &fakeSPFRouteSyncer{route: spfroutes.Route{ID: "route-1", LastSyncStatus: spfroutes.StatusHealthy}}
	worker := &SyncSPFRouteWorker{Syncer: syncer, Events: events}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1", Manual: true, ScheduledFor: scheduled}}

	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("Work returned error: %v", err)
	}
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(events.created))
	}
	event := events.created[0]
	if event.Kind != "spf_route.publish_succeeded" || event.Severity != ops.SeverityInfo {
		t.Fatalf("event kind/severity = %q/%q, want publish_succeeded/info", event.Kind, event.Severity)
	}
	if event.Actor != "worker:sync_spf_route" || event.Resource != "spf_route:route-1" {
		t.Fatalf("event actor/resource = %q/%q", event.Actor, event.Resource)
	}
	if event.Metadata["route_id"] != "route-1" || event.Metadata["status"] != string(spfroutes.StatusHealthy) || event.Metadata["manual"] != true || event.Metadata["scheduled_for"] != scheduled.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected metadata: %#v", event.Metadata)
	}
	if len(syncer.calls) != 1 || syncer.calls[0] != "route-1" {
		t.Fatalf("sync calls = %+v, want route-1", syncer.calls)
	}
}

func TestSyncSPFRouteWorkerRecordsSanitizedFailureOperationalEventAndReturnsOriginalError(t *testing.T) {
	expected := errors.New("provider token=secret failed at https://user:pass@example.test")
	events := &fakeOperationalEventStore{}
	worker := &SyncSPFRouteWorker{Syncer: &fakeSPFRouteSyncer{err: expected}, Events: events}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1", Manual: true}}

	if err := worker.Work(context.Background(), job); !errors.Is(err, expected) {
		t.Fatalf("Work error = %v, want %v", err, expected)
	}
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(events.created))
	}
	event := events.created[0]
	if event.Kind != "spf_route.publish_failed" || event.Severity != ops.SeverityWarn {
		t.Fatalf("event kind/severity = %q/%q, want publish_failed/warn", event.Kind, event.Severity)
	}
	if event.Metadata["error_class"] != "sync_error" {
		t.Fatalf("error_class = %#v, want sync_error; metadata=%#v", event.Metadata["error_class"], event.Metadata)
	}
	if _, ok := event.Metadata["error"]; ok {
		t.Fatalf("metadata must not include raw error: %#v", event.Metadata)
	}
}

func TestSyncSPFRouteWorkerOperationalEventFailureIsBestEffort(t *testing.T) {
	expected := errors.New("event store unavailable")
	events := &fakeOperationalEventStore{err: expected}
	worker := &SyncSPFRouteWorker{
		Syncer: &fakeSPFRouteSyncer{route: spfroutes.Route{ID: "route-1", LastSyncStatus: spfroutes.StatusHealthy}},
		Events: events,
	}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1"}}

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work returned error from best-effort event store: %v", err)
	}
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want attempted event despite store error", len(events.created))
	}
}

func TestSyncSPFRouteWorkerRecordsBlockedOperationalEvent(t *testing.T) {
	events := &fakeOperationalEventStore{}
	worker := &SyncSPFRouteWorker{
		Syncer: &fakeSPFRouteSyncer{route: spfroutes.Route{ID: "route-1", LastSyncStatus: spfroutes.StatusBlocked}},
		Events: events,
	}
	job := &river.Job[SyncSPFRouteArgs]{Args: SyncSPFRouteArgs{RouteID: "route-1"}}

	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work returned error: %v", err)
	}
	if len(events.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(events.created))
	}
	if events.created[0].Kind != "spf_route.publish_blocked" || events.created[0].Severity != ops.SeverityWarn {
		t.Fatalf("event kind/severity = %q/%q, want publish_blocked/warn", events.created[0].Kind, events.created[0].Severity)
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

type fakeOperationalEventStore struct {
	created []ops.Event
	err     error
}

func (s *fakeOperationalEventStore) CreateOperationalEvent(ctx context.Context, event ops.Event) (ops.Event, error) {
	s.created = append(s.created, event)
	return event, s.err
}
