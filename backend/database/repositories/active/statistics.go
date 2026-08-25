package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StatisticsRepository implements active.StatisticsRepository (#2606).
// The aggregates are deliberately shaped for the Statistik page and do
// not fit the generic Repository[T] filter surface: they group across
// students / rooms and clamp visit durations to the report window.
type StatisticsRepository struct {
	db *bun.DB
}

// NewStatisticsRepository creates a statistics repository.
func NewStatisticsRepository(db *bun.DB) active.StatisticsRepository {
	return &StatisticsRepository{db: db}
}

// AttendanceDays returns distinct (student, date) pairs with an attendance
// row in [from, to]. Tenant scope: RLS plus the explicit filter.
func (r *StatisticsRepository) AttendanceDays(ctx context.Context, from, to timezone.Date) ([]active.AttendanceDayRow, error) {
	var rows []active.AttendanceDayRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.attendance AS "attendance"`).
		ColumnExpr(`"attendance".student_id`).
		ColumnExpr(`"attendance".date`).
		Where(`"attendance".date >= ? AND "attendance".date <= ?`, from, to).
		GroupExpr(`"attendance".student_id, "attendance".date`)
	query = base.WithTenantFilter(ctx, query, "attendance")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics attendance days", Err: err}
	}
	return rows, nil
}

// StatusDays returns every uncleared status day in [from, to].
func (r *StatisticsRepository) StatusDays(ctx context.Context, from, to timezone.Date) ([]active.StatusDayRow, error) {
	var rows []active.StatusDayRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.student_status_days AS "status_day"`).
		ColumnExpr(`"status_day".student_id`).
		ColumnExpr(`"status_day".date`).
		ColumnExpr(`"status_day".status`).
		Where(`"status_day".date >= ? AND "status_day".date <= ?`, from, to).
		Where(`"status_day".cleared_at IS NULL`)
	query = base.WithTenantFilter(ctx, query, "status_day")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics status days", Err: err}
	}
	return rows, nil
}

// RoomUtilization aggregates visits overlapping [start, end) per room.
//
// Window semantics follow AggregateRoomSessions: a visit counts when it
// started before `end` and either is still open or ended after `start`.
// Minutes are clamped to the window. The peak is a sweep over entry (+1)
// and exit (-1) events per room; at equal instants the exit sorts first so
// a back-to-back room change never counts a child twice.
func (r *StatisticsRepository) RoomUtilization(ctx context.Context, start, end time.Time, groupIDs []int64) ([]active.RoomUtilizationRow, error) {
	tenantID := tenant.FromContext(ctx)
	// Placeholder order in the SQL below: GREATEST(start), LEAST(end),
	// entry_time < end, exit_time > start, then the three tenant filters.
	tenantClause := ""
	groupClause := ""
	args := []any{start, end, end, start}
	if tenantID > 0 {
		tenantClause = " AND v.tenant_id = ? AND ag.tenant_id = ? AND s.tenant_id = ?"
		args = append(args, tenantID, tenantID, tenantID)
	}
	if len(groupIDs) > 0 {
		groupClause = " AND s.group_id IN (?)"
		args = append(args, bun.In(groupIDs))
	}

	sql := `
WITH scoped AS (
	SELECT ag.room_id,
	       v.student_id,
	       GREATEST(v.entry_time, ?) AS entry_at,
	       LEAST(COALESCE(v.exit_time, NOW()), ?) AS exit_at
	FROM active.visits v
	JOIN active.groups ag ON ag.id = v.active_group_id
	JOIN users.students s ON s.id = v.student_id
	WHERE v.entry_time < ?
	  AND (v.exit_time IS NULL OR v.exit_time > ?)` + tenantClause + groupClause + `
),
events AS (
	SELECT room_id, entry_at AS at, 1 AS delta FROM scoped
	UNION ALL
	SELECT room_id, exit_at AS at, -1 AS delta FROM scoped
),
running AS (
	SELECT room_id, SUM(delta) OVER (PARTITION BY room_id ORDER BY at, delta) AS occupancy
	FROM events
),
peak AS (
	SELECT room_id, MAX(occupancy) AS peak_occupancy FROM running GROUP BY room_id
)
SELECT s.room_id,
       COUNT(DISTINCT (s.entry_at AT TIME ZONE 'Europe/Berlin')::date) AS days_used,
       COUNT(DISTINCT s.student_id) AS distinct_students,
       COALESCE(SUM(GREATEST(EXTRACT(EPOCH FROM (s.exit_at - s.entry_at)), 0)) / 60, 0)::int AS student_minutes,
       COALESCE(p.peak_occupancy, 0) AS peak_occupancy
FROM scoped s
LEFT JOIN peak p ON p.room_id = s.room_id
GROUP BY s.room_id, p.peak_occupancy
ORDER BY s.room_id`
	var rows []active.RoomUtilizationRow
	if err := base.GetDB(ctx, r.db).NewRaw(sql, args...).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics room utilization", Err: err}
	}
	return rows, nil
}
