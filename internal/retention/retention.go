package retention

import "time"

type Policy struct {
	RawReportDays       int
	AggregateRowDays    int
	AuditLogDays        int
	NotificationLogDays int
}

func FreePolicy() Policy {
	return Policy{RawReportDays: 0, AggregateRowDays: 0, AuditLogDays: 365, NotificationLogDays: 30}
}

func ReporterPolicy() Policy {
	return Policy{RawReportDays: 90, AggregateRowDays: 365, AuditLogDays: 730, NotificationLogDays: 180}
}

func BusinessPolicy() Policy {
	return Policy{RawReportDays: 365, AggregateRowDays: 730, AuditLogDays: 2555, NotificationLogDays: 365}
}

func Cutoff(now time.Time, days int) time.Time {
	if days <= 0 {
		return now.UTC()
	}
	return now.UTC().AddDate(0, 0, -days)
}

func ShouldDelete(createdAt time.Time, now time.Time, days int) bool {
	if days <= 0 {
		return true
	}
	return createdAt.Before(Cutoff(now, days))
}
