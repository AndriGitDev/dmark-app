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

type AttentionReason string

const (
	ReasonStale   AttentionReason = "stale"
	ReasonFailed  AttentionReason = "failed"
	ReasonBlocked AttentionReason = "blocked"
)

type AuditResult struct {
	Total     int             `json:"total"`
	Enabled   int             `json:"enabled"`
	Healthy   int             `json:"healthy"`
	Failed    int             `json:"failed"`
	Blocked   int             `json:"blocked"`
	Pending   int             `json:"pending"`
	Stale     int             `json:"stale"`
	Attention []AttentionItem `json:"attention"`
}

type AttentionItem struct {
	RouteID      string           `json:"route_id"`
	Domain       string           `json:"domain"`
	Status       spfroutes.Status `json:"status"`
	Reason       AttentionReason  `json:"reason"`
	LastError    string           `json:"last_error,omitempty"`
	LastSyncedAt time.Time        `json:"last_synced_at,omitempty"`
	NextSyncAt   time.Time        `json:"next_sync_at,omitempty"`
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

func Audit(now time.Time, cadence time.Duration, routes []spfroutes.Route) AuditResult {
	if cadence <= 0 {
		cadence = DefaultCadence
	}
	result := AuditResult{Total: len(routes)}
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		result.Enabled++
		switch route.LastSyncStatus {
		case spfroutes.StatusHealthy:
			result.Healthy++
		case spfroutes.StatusFailed:
			result.Failed++
		case spfroutes.StatusBlocked:
			result.Blocked++
		default:
			result.Pending++
		}
		nextSyncAt := scheduler.NextSPFRouteSync(route.LastSyncedAt, cadence)
		isStale := !nextSyncAt.After(now)
		if isStale {
			result.Stale++
			result.Attention = append(result.Attention, attentionItem(route, ReasonStale, nextSyncAt))
		}
		switch route.LastSyncStatus {
		case spfroutes.StatusFailed:
			result.Attention = append(result.Attention, attentionItem(route, ReasonFailed, nextSyncAt))
		case spfroutes.StatusBlocked:
			result.Attention = append(result.Attention, attentionItem(route, ReasonBlocked, nextSyncAt))
		}
	}
	return result
}

func attentionItem(route spfroutes.Route, reason AttentionReason, nextSyncAt time.Time) AttentionItem {
	return AttentionItem{
		RouteID:      route.ID,
		Domain:       route.Domain,
		Status:       route.LastSyncStatus,
		Reason:       reason,
		LastError:    route.LastSyncError,
		LastSyncedAt: route.LastSyncedAt,
		NextSyncAt:   nextSyncAt,
	}
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
