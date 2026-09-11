package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Store) CountPlannedSupervisorsByCalendarPeriod(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		CalendarPeriodID int64 `bun:"calendar_period_id"`
		Count            int   `bun:"count"`
	}
	stats, err := scanAllInto(ctx, db.NewSelect().Table("activities.supervisors").
		ColumnExpr("calendar_period_id, COUNT(*)::int AS count").Where("tenant_id = ?", tenantID).
		Where("calendar_period_id IS NOT NULL").GroupExpr("calendar_period_id"), &rows, "count planned supervisors by calendar period")
	if err != nil {
		return nil, stats, err
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.CalendarPeriodID] = row.Count
	}
	return counts, stats, nil
}
