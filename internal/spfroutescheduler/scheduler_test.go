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
