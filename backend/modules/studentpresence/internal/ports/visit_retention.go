package ports

import (
	"context"
	"time"
)

type VisitRetentionCount struct {
	StudentID int64
	Count     int
}
type VisitMonthCount struct {
	Month string
	Count int64
}

type VisitRetentionStore interface {
	ListVisitRetentionCounts(context.Context) ([]VisitRetentionCount, Stats, error)
	CountExpiredVisits(context.Context) (int64, Stats, error)
	OldestExpiredVisitDate(context.Context) (*time.Time, Stats, error)
	ListExpiredVisitMonths(context.Context) ([]VisitMonthCount, Stats, error)
}
