package alerts

import (
	"context"
	"sort"
	"time"

	"github.com/AndriGitDev/meil/internal/drift"
)

type EventState struct {
	ID              string
	Domain          string
	Event           drift.Event
	CreatedAt       time.Time
	AcknowledgedAt  time.Time
	MutedUntil      time.Time
	SuppressedUntil time.Time
}

type Store interface {
	AcknowledgeEvent(ctx context.Context, userID string, eventID string, at time.Time) error
	MuteEvent(ctx context.Context, userID string, eventID string, until time.Time) error
}

func Acknowledge(state EventState, at time.Time) EventState {
	state.AcknowledgedAt = at.UTC()
	return state
}

func Mute(state EventState, until time.Time) EventState {
	state.MutedUntil = until.UTC()
	return state
}

func Suppress(state EventState, until time.Time) EventState {
	state.SuppressedUntil = until.UTC()
	return state
}

func ShouldNotifyState(state EventState, rule Rule, now time.Time) Decision {
	if !state.AcknowledgedAt.IsZero() {
		return Decision{Reason: "event acknowledged"}
	}
	if !state.SuppressedUntil.IsZero() && now.Before(state.SuppressedUntil) {
		return Decision{Reason: "event suppressed"}
	}
	if !state.MutedUntil.IsZero() && now.Before(state.MutedUntil) {
		return Decision{Reason: "event muted"}
	}
	return ShouldNotify(state.Event, rule, now)
}

type TimelineItem struct {
	ID           string
	Domain       string
	CheckName    string
	Kind         drift.Kind
	Severity     drift.Severity
	Message      string
	CreatedAt    time.Time
	Acknowledged bool
	Muted        bool
	Suppressed   bool
	BeforeRecord string
	AfterRecord  string
}

func BuildTimeline(states []EventState, now time.Time) []TimelineItem {
	items := make([]TimelineItem, 0, len(states))
	for _, state := range states {
		items = append(items, TimelineItem{
			ID:           state.ID,
			Domain:       state.Domain,
			CheckName:    state.Event.CheckName,
			Kind:         state.Event.Kind,
			Severity:     state.Event.Severity,
			Message:      state.Event.Message,
			CreatedAt:    state.CreatedAt,
			Acknowledged: !state.AcknowledgedAt.IsZero(),
			Muted:        !state.MutedUntil.IsZero() && now.Before(state.MutedUntil),
			Suppressed:   !state.SuppressedUntil.IsZero() && now.Before(state.SuppressedUntil),
			BeforeRecord: state.Event.BeforeRecord,
			AfterRecord:  state.Event.AfterRecord,
		})
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			if severityRank(items[i].Severity) == severityRank(items[j].Severity) {
				return items[i].ID < items[j].ID
			}
			return severityRank(items[i].Severity) > severityRank(items[j].Severity)
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}
