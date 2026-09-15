package studentpresence

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

type VisitRetentionQuery interface {
	ListVisitRetentionCounts(context.Context) ([]VisitRetentionCount, error)
	CountExpiredVisits(context.Context) (int64, error)
	OldestExpiredVisitDate(context.Context) (*time.Time, error)
	ListExpiredVisitMonths(context.Context) ([]VisitMonthCount, error)
}

func (m *Module) ListVisitRetentionCounts(ctx context.Context) ([]VisitRetentionCount, error) {
	return m.engine.ListVisitRetentionCounts(ctx)
}
func (m *Module) CountExpiredVisits(ctx context.Context) (int64, error) {
	return m.engine.CountExpiredVisits(ctx)
}
func (m *Module) OldestExpiredVisitDate(ctx context.Context) (*time.Time, error) {
	return m.engine.OldestExpiredVisitDate(ctx)
}
func (m *Module) ListExpiredVisitMonths(ctx context.Context) ([]VisitMonthCount, error) {
	return m.engine.ListExpiredVisitMonths(ctx)
}
