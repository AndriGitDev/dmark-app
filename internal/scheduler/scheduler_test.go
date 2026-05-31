package scheduler

import (
	"testing"
	"time"
)

func TestNextCheck(t *testing.T) {
	t.Parallel()

	last := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	if got := NextCheck(last, time.Hour); !got.Equal(last.Add(time.Hour)) {
		t.Fatalf("NextCheck = %v, want %v", got, last.Add(time.Hour))
	}
	if got := NextCheck(last, 0); !got.Equal(last.Add(24 * time.Hour)) {
		t.Fatalf("NextCheck default = %v, want 24h cadence", got)
	}
	if got := NextCheck(time.Time{}, time.Hour); !got.IsZero() {
		t.Fatalf("NextCheck zero = %v, want zero", got)
	}
}

func TestDue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	due := Due(now, []DomainSchedule{
		{UserID: "user-1", DomainID: "domain-1", Domain: "due.example", LastCheckedAt: now.Add(-2 * time.Hour), Cadence: time.Hour, Enabled: true},
		{UserID: "user-1", DomainID: "domain-2", Domain: "future.example", LastCheckedAt: now.Add(-30 * time.Minute), Cadence: time.Hour, Enabled: true},
		{UserID: "user-1", DomainID: "domain-3", Domain: "disabled.example", LastCheckedAt: now.Add(-2 * time.Hour), Cadence: time.Hour, Enabled: false},
		{UserID: "user-1", DomainID: "domain-4", Domain: "never.example", Enabled: true},
	})

	if len(due) != 2 {
		t.Fatalf("len(due) = %d, want 2: %#v", len(due), due)
	}
	if due[0].Domain != "due.example" || due[0].Args.IdempotencyKey() == "" {
		t.Fatalf("unexpected first due scan: %+v", due[0])
	}
	if due[1].Domain != "never.example" || !due[1].ScheduledFor.Equal(now) {
		t.Fatalf("unexpected never-scanned due scan: %+v", due[1])
	}
}

func TestDueSPFRouteSyncs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	due := DueSPFRouteSyncs(now, []SPFRouteSchedule{
		{RouteID: "route-due", LastSyncedAt: now.Add(-13 * time.Hour), Cadence: 12 * time.Hour, Enabled: true},
		{RouteID: "route-default-cadence", LastSyncedAt: now.Add(-13 * time.Hour), Enabled: true},
		{RouteID: "route-future", LastSyncedAt: now.Add(-6 * time.Hour), Cadence: 12 * time.Hour, Enabled: true},
		{RouteID: "route-disabled", LastSyncedAt: now.Add(-13 * time.Hour), Cadence: 12 * time.Hour, Enabled: false},
		{RouteID: "route-never", Enabled: true},
		{RouteID: "", LastSyncedAt: now.Add(-13 * time.Hour), Cadence: 12 * time.Hour, Enabled: true},
	})

	if len(due) != 3 {
		t.Fatalf("len(due) = %d, want 3: %#v", len(due), due)
	}
	if due[0].RouteID != "route-due" || due[0].Args.RouteID != "route-due" || due[0].Args.Manual {
		t.Fatalf("unexpected first SPF sync: %+v", due[0])
	}
	if !due[0].ScheduledFor.Equal(now.Add(-time.Hour)) {
		t.Fatalf("first SPF scheduled_for = %v, want %v", due[0].ScheduledFor, now.Add(-time.Hour))
	}
	if due[1].RouteID != "route-default-cadence" || !due[1].ScheduledFor.Equal(now.Add(-time.Hour)) {
		t.Fatalf("unexpected default-cadence SPF sync: %+v", due[1])
	}
	if due[2].RouteID != "route-never" || !due[2].ScheduledFor.Equal(now) || due[2].Args.IdempotencyKey() == "" {
		t.Fatalf("unexpected never-synced SPF sync: %+v", due[2])
	}
}
