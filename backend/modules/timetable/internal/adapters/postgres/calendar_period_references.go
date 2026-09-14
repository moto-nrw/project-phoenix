package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// CountCalendarPeriodReferences reads every Timetable-owned table that carries
// a calendar_period_id in one statement (#3124). Keeping the five tables in a
// single UNION ALL is what keeps the School Calendar usage read at one round
// trip per owner instead of one per table.
func (s *Store) CountCalendarPeriodReferences(ctx context.Context) (map[int64]domain.CalendarPeriodReferences, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		Source           string `bun:"source"`
		CalendarPeriodID int64  `bun:"calendar_period_id"`
		Count            int    `bun:"count"`
	}
	query := db.NewSelect().
		ColumnExpr("source, calendar_period_id, count").
		TableExpr(`(
			SELECT 'activity_group' AS source, g.calendar_period_id, COUNT(*)::int AS count
			FROM activities.groups AS g
			WHERE g.tenant_id = ? AND g.calendar_period_id IS NOT NULL
			GROUP BY g.calendar_period_id
			UNION ALL
			SELECT 'schedule' AS source, s.calendar_period_id, COUNT(*)::int AS count
			FROM activities.schedules AS s
			WHERE s.tenant_id = ? AND s.calendar_period_id IS NOT NULL
			GROUP BY s.calendar_period_id
			UNION ALL
			SELECT 'student_enrollment' AS source, se.calendar_period_id, COUNT(*)::int AS count
			FROM activities.student_enrollments AS se
			WHERE se.tenant_id = ? AND se.calendar_period_id IS NOT NULL
			GROUP BY se.calendar_period_id
			UNION ALL
			SELECT 'supervisor' AS source, sup.calendar_period_id, COUNT(*)::int AS count
			FROM activities.supervisors AS sup
			WHERE sup.tenant_id = ? AND sup.calendar_period_id IS NOT NULL
			GROUP BY sup.calendar_period_id
			UNION ALL
			SELECT 'activity_instance' AS source, ai.calendar_period_id, COUNT(*)::int AS count
			FROM schedule.activity_instances AS ai
			WHERE ai.tenant_id = ? AND ai.calendar_period_id IS NOT NULL
			GROUP BY ai.calendar_period_id
		) AS refs`, tenantID, tenantID, tenantID, tenantID, tenantID)
	stats, err := scanAllInto(ctx, query, &rows, "count calendar period references")
	if err != nil {
		return nil, stats, err
	}
	counts := make(map[int64]domain.CalendarPeriodReferences, len(rows))
	for _, row := range rows {
		entry := counts[row.CalendarPeriodID]
		switch row.Source {
		case "activity_group":
			entry.ActivityGroups = row.Count
		case "schedule":
			entry.Schedules = row.Count
		case "student_enrollment":
			entry.StudentEnrollments = row.Count
		case "supervisor":
			entry.Supervisors = row.Count
		case "activity_instance":
			entry.ActivityInstances = row.Count
		}
		counts[row.CalendarPeriodID] = entry
	}
	stats.Rows = int64(len(rows))
	return counts, stats, nil
}
