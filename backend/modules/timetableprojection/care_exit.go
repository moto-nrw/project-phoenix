package timetableprojection

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// CountPlannedRosterAfter counts live and restorable roster rows against the
// pre-exit baseline. A row already restored manually is counted only once.
func CountPlannedRosterAfter(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, after timezone.Date, removals string) (map[int64]int, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	err := db.NewRaw(plannedRosterQuery,
		bun.List(studentIDs), after, tenantID,
		removals, tenantID, bun.List(studentIDs), after,
	).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: count planned roster: %w", err)
	}
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, nil
}

const plannedRosterQuery = `
		SELECT student_id, COUNT(*)::int AS total FROM (
			SELECT s.student_id AS student_id
			FROM schedule.instance_students AS s
			JOIN schedule.activity_instances AS ai
			  ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
			WHERE s.student_id IN (?)
	  AND s.checked_in_at IS NULL
	  AND s.checked_out_at IS NULL
	  AND ai.date > ?
	  AND ai.status NOT IN ('completed', 'cancelled')
	  AND s.tenant_id = ?
			UNION ALL
			SELECT rm.student_id AS student_id
			FROM jsonb_to_recordset(?::jsonb) AS rm(
			     tenant_id bigint, student_id bigint, kind text, instance_id bigint
			)
			JOIN schedule.activity_instances AS ai
			  ON ai.id = rm.instance_id AND ai.tenant_id = rm.tenant_id
			WHERE rm.kind = 'roster'
			  AND rm.tenant_id = ?
			  AND rm.student_id IN (?)
			  AND ai.date > ?
			  AND ai.status NOT IN ('completed', 'cancelled')
			  -- Somebody may have put the child back on that block by hand
			  -- since. Then the live branch above already counted it, and the
			  -- restore will skip it.
			  AND NOT EXISTS (
			        SELECT 1 FROM schedule.instance_students AS live
			         WHERE live.instance_id = rm.instance_id
			           AND live.student_id = rm.student_id
			           AND live.tenant_id = rm.tenant_id
			      )
		) AS baseline
		GROUP BY student_id`

// LatestRosterAttendanceDate ignores planned and manually marked rows without
// an observed check-in. The instance date, not the check-in instant, is the day.
func LatestRosterAttendanceDate(ctx context.Context, db bun.IDB, tenantID, studentID int64) (*timezone.Date, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	var day *timezone.Date
	err := db.NewRaw(`SELECT MAX(instance.date)
  FROM schedule.instance_students AS roster
  JOIN schedule.activity_instances AS instance
    ON instance.tenant_id = roster.tenant_id AND instance.id = roster.instance_id
  WHERE roster.tenant_id = ? AND roster.student_id = ?
    AND roster.checked_in_at IS NOT NULL`, tenantID, studentID).Scan(ctx, &day)
	if err != nil {
		return nil, fmt.Errorf("timetable projection: latest roster attendance: %w", err)
	}
	return day, nil
}
