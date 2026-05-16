package scheduler

import (
	"time"

	"github.com/AndriGitDev/meil/internal/jobs"
)

type DomainSchedule struct {
	UserID        string
	DomainID      string
	Domain        string
	LastCheckedAt time.Time
	Cadence       time.Duration
	Enabled       bool
}

type DueScan struct {
	DomainID     string
	Domain       string
	ScheduledFor time.Time
	Args         jobs.ScanDomainArgs
}

func NextCheck(lastCheckedAt time.Time, cadence time.Duration) time.Time {
	if cadence <= 0 {
		cadence = 24 * time.Hour
	}
	if lastCheckedAt.IsZero() {
		return time.Time{}
	}
	return lastCheckedAt.UTC().Add(cadence)
}

func Due(now time.Time, schedules []DomainSchedule) []DueScan {
	now = now.UTC()
	due := []DueScan{}
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		scheduledFor := NextCheck(schedule.LastCheckedAt, schedule.Cadence)
		if scheduledFor.IsZero() {
			scheduledFor = now
		}
		if scheduledFor.After(now) {
			continue
		}
		args := jobs.ScanDomainArgs{
			UserID:       schedule.UserID,
			DomainID:     schedule.DomainID,
			Domain:       schedule.Domain,
			ScheduledFor: scheduledFor,
		}
		due = append(due, DueScan{
			DomainID:     schedule.DomainID,
			Domain:       schedule.Domain,
			ScheduledFor: scheduledFor,
			Args:         args,
		})
	}
	return due
}
