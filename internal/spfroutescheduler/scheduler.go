package spfroutescheduler

import (
	"context"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/AndriGitDev/meil/internal/scheduler"
	"github.com/AndriGitDev/meil/internal/spfroutes"
)

const DefaultCadence = 12 * time.Hour

type RouteStore interface {
	ListEnabledSPFRoutes(ctx context.Context) ([]spfroutes.Route, error)
}

type Enqueuer interface {
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

type Result struct {
	Checked int
	Due     int
	Queued  int
}

func SchedulesFromRoutes(routes []spfroutes.Route, cadence time.Duration) []scheduler.SPFRouteSchedule {
	if cadence <= 0 {
		cadence = DefaultCadence
	}
	schedules := make([]scheduler.SPFRouteSchedule, 0, len(routes))
	for _, route := range routes {
		schedules = append(schedules, scheduler.SPFRouteSchedule{
			RouteID:      route.ID,
			LastSyncedAt: route.LastSyncedAt,
			Cadence:      cadence,
			Enabled:      route.Enabled,
		})
	}
	return schedules
}

func EnqueueDue(ctx context.Context, store RouteStore, enqueuer Enqueuer, now time.Time, cadence time.Duration) (Result, error) {
	routes, err := store.ListEnabledSPFRoutes(ctx)
	if err != nil {
		return Result{}, err
	}
	due := scheduler.DueSPFRouteSyncs(now, SchedulesFromRoutes(routes, cadence))
	result := Result{Checked: len(routes), Due: len(due)}
	for _, item := range due {
		args := item.Args
		if _, err := enqueuer.Insert(ctx, args, ptr(args.InsertOpts())); err != nil {
			return result, err
		}
		result.Queued++
	}
	return result, nil
}

func ptr[T any](value T) *T {
	return &value
}
