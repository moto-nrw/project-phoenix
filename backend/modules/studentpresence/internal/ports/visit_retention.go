package ports

import (
	"context"
	"time"
)

// VisitRetentionCount is one student's expired visits and the retention
// window that expired them.
type VisitRetentionCount struct {
	StudentID     int64
	Count         int
	RetentionDays int
}
type VisitMonthCount struct {
	Month string
	Count int64
}

// VisitRetentionStore reads completed visits past their retention window. A
// child's window is the shortest of its accepted consents; every other child
// uses defaultDays, the tenant's gdpr.privacy_consent_retention_days (#3835).
type VisitRetentionStore interface {
	ListVisitRetentionCounts(ctx context.Context, defaultDays int) ([]VisitRetentionCount, Stats, error)
	CountExpiredVisits(ctx context.Context, defaultDays int) (int64, Stats, error)
	OldestExpiredVisitDate(ctx context.Context, defaultDays int) (*time.Time, Stats, error)
	ListExpiredVisitMonths(ctx context.Context, defaultDays int) ([]VisitMonthCount, Stats, error)
}
