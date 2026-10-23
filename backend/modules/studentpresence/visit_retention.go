package studentpresence

import (
	"context"
	"time"
)

// VisitRetentionPolicy selects rolling consent windows or a completed-day cutoff.
type VisitRetentionPolicy struct {
	DefaultDays int
	// CompletedBefore selects only visits closed before this instant, ignoring consent windows.
	CompletedBefore *time.Time
}

// VisitRetentionCount carries the exact policy used to select a student's expired visits.
type VisitRetentionCount struct {
	CompletedBefore *time.Time
	StudentID       int64
	Count           int
	RetentionDays   int
}
type VisitMonthCount struct {
	Month string
	Count int64
}

// VisitRetentionQuery reads completed visits past their retention window: the
// shortest accepted consent window, otherwise DefaultDays. CompletedBefore
// overrides both for schools whose attendance log is disabled.
type VisitRetentionQuery interface {
	ListVisitRetentionCounts(ctx context.Context, policy VisitRetentionPolicy) ([]VisitRetentionCount, error)
	CountExpiredVisits(ctx context.Context, policy VisitRetentionPolicy) (int64, error)
	OldestExpiredVisitDate(ctx context.Context, policy VisitRetentionPolicy) (*time.Time, error)
	ListExpiredVisitMonths(ctx context.Context, policy VisitRetentionPolicy) ([]VisitMonthCount, error)
}
