package studentpresence

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

// VisitRetentionQuery reads completed visits past their retention window: the
// shortest accepted consent window of the child, otherwise defaultDays, the
// tenant's gdpr.privacy_consent_retention_days (#3835).
type VisitRetentionQuery interface {
	ListVisitRetentionCounts(ctx context.Context, defaultDays int) ([]VisitRetentionCount, error)
	CountExpiredVisits(ctx context.Context, defaultDays int) (int64, error)
	OldestExpiredVisitDate(ctx context.Context, defaultDays int) (*time.Time, error)
	ListExpiredVisitMonths(ctx context.Context, defaultDays int) ([]VisitMonthCount, error)
}
