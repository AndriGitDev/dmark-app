package spfroutescheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/AndriGitDev/meil/internal/jobs"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

func TestEnqueueDueQueuesOnlyDueRoutes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	store := &fakeRouteStore{routes: []spfroutes.Route{
		{ID: "due", Enabled: true, LastSyncedAt: now.Add(-13 * time.Hour)},
		{ID: "never", Enabled: true},
		{ID: "fresh", Enabled: true, LastSyncedAt: now.Add(-time.Hour)},
	}}
	enqueuer := &fakeEnqueuer{}

	result, err := EnqueueDue(context.Background(), store, enqueuer, now, 12*time.Hour)
	if err != nil {
		t.Fatalf("EnqueueDue returned error: %v", err)
	}
	if result.Checked != 3 || result.Due != 2 || result.Queued != 2 {
		t.Fatalf("result = %+v, want checked=3 due=2 queued=2", result)
	}
	if len(enqueuer.args) != 2 || enqueuer.args[0].RouteID != "due" || enqueuer.args[1].RouteID != "never" {
		t.Fatalf("queued args = %+v", enqueuer.args)
	}
	if enqueuer.args[0].Manual || enqueuer.args[1].Manual {
		t.Fatalf("scheduled jobs must not be marked manual: %+v", enqueuer.args)
	}
	if enqueuer.opts[0].Queue != jobs.ScanQueue || enqueuer.opts[1].Queue != jobs.ScanQueue {
		t.Fatalf("unexpected queue opts: %+v", enqueuer.opts)
	}
}

func TestEnqueueDueSurfacesStoreAndInsertErrors(t *testing.T) {
	t.Parallel()

	expected := errors.New("database unavailable")
	if _, err := EnqueueDue(context.Background(), &fakeRouteStore{err: expected}, &fakeEnqueuer{}, time.Now(), 0); !errors.Is(err, expected) {
		t.Fatalf("store error = %v, want %v", err, expected)
	}

	expected = errors.New("river unavailable")
	store := &fakeRouteStore{routes: []spfroutes.Route{{ID: "due", Enabled: true}}}
	_, err := EnqueueDue(context.Background(), store, &fakeEnqueuer{err: expected}, time.Now(), 0)
	if !errors.Is(err, expected) {
		t.Fatalf("insert error = %v, want %v", err, expected)
	}
}

func TestAuditClassifiesFailedBlockedAndStaleRoutes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)
	audit := Audit(now, 12*time.Hour, []spfroutes.Route{
		{ID: "healthy-fresh", Domain: "fresh.example", Enabled: true, LastSyncStatus: spfroutes.StatusHealthy, LastSyncedAt: now.Add(-time.Hour)},
		{ID: "healthy-stale", Domain: "stale.example", Enabled: true, LastSyncStatus: spfroutes.StatusHealthy, LastSyncedAt: now.Add(-13 * time.Hour)},
		{ID: "never-synced", Domain: "never.example", Enabled: true, LastSyncStatus: spfroutes.StatusPending},
		{ID: "failed", Domain: "failed.example", Enabled: true, LastSyncStatus: spfroutes.StatusFailed, LastSyncError: "Bunny DNS timeout", LastSyncedAt: now.Add(-10 * time.Minute)},
		{ID: "blocked", Domain: "blocked.example", Enabled: true, LastSyncStatus: spfroutes.StatusBlocked, LastSyncError: "lookup limit exceeded", LastSyncedAt: now.Add(-10 * time.Minute)},
		{ID: "disabled", Domain: "disabled.example", Enabled: false, LastSyncStatus: spfroutes.StatusFailed, LastSyncedAt: now.Add(-24 * time.Hour)},
	})

	if audit.Total != 6 || audit.Enabled != 5 || audit.Healthy != 2 || audit.Failed != 1 || audit.Blocked != 1 || audit.Pending != 1 || audit.Stale != 2 {
		t.Fatalf("audit counts = %+v", audit)
	}
	if len(audit.Attention) != 4 {
		t.Fatalf("attention length = %d, want 4: %+v", len(audit.Attention), audit.Attention)
	}
	if audit.Attention[0].RouteID != "healthy-stale" || audit.Attention[0].Reason != ReasonStale {
		t.Fatalf("first attention item = %+v, want stale route", audit.Attention[0])
	}
	if audit.Attention[1].RouteID != "never-synced" || audit.Attention[1].Reason != ReasonStale {
		t.Fatalf("second attention item = %+v, want never-synced stale route", audit.Attention[1])
	}
	if audit.Attention[2].RouteID != "failed" || audit.Attention[2].Reason != ReasonFailed || audit.Attention[2].LastError != "Bunny DNS timeout" {
		t.Fatalf("third attention item = %+v, want failed route with error", audit.Attention[2])
	}
	if audit.Attention[3].RouteID != "blocked" || audit.Attention[3].Reason != ReasonBlocked || audit.Attention[3].LastError != "lookup limit exceeded" {
		t.Fatalf("fourth attention item = %+v, want blocked route with error", audit.Attention[3])
	}
}

type fakeRouteStore struct {
	routes []spfroutes.Route
	err    error
}

func (s *fakeRouteStore) ListEnabledSPFRoutes(context.Context) ([]spfroutes.Route, error) {
	return s.routes, s.err
}

type fakeEnqueuer struct {
	args []jobs.SyncSPFRouteArgs
	opts []river.InsertOpts
	err  error
}

func (e *fakeEnqueuer) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	spfArgs := args.(jobs.SyncSPFRouteArgs)
	e.args = append(e.args, spfArgs)
	if opts != nil {
		e.opts = append(e.opts, *opts)
	}
	return &rivertype.JobInsertResult{}, e.err
}
