package active

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StatisticsRepository implements active.StatisticsRepository (#2606).
// The aggregates are deliberately shaped for the Statistik page and do
// not fit the generic Repository[T] filter surface: they group across
// students / rooms and clamp visit durations to the report window.
type StatisticsRepository struct {
	db         *bun.DB
	statusDays interface {
		ListStatusDaySummaries(context.Context, string, string) ([]StatusDaySummary, error)
	}
}

type StatusDaySummary struct {
	StudentID int64
	Date      string
	Status    string
}

func (r *StatisticsRepository) BindCarePlan(query interface {
	ListStatusDaySummaries(context.Context, string, string) ([]StatusDaySummary, error)
}) {
	r.statusDays = query
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
		return nil, &modelBase.DatabaseError{Op: "statistics attendance days", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// StatusDays returns every uncleared status day in [from, to].
func (r *StatisticsRepository) StatusDays(ctx context.Context, from, to timezone.Date) ([]active.StatusDayRow, error) {
	if r.statusDays == nil {
		return nil, errors.New("statistics status days: care plan capability is required")
	}
	values, err := r.statusDays.ListStatusDaySummaries(ctx, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	rows := make([]active.StatusDayRow, 0, len(values))
	for _, value := range values {
		rows = append(rows, active.StatusDayRow{StudentID: value.StudentID, Date: timezone.Date(value.Date), Status: value.Status})
	}
	return rows, nil
}

// RoomUtilization aggregates visits overlapping [start, end) per room.
//
// Window semantics follow AggregateRoomSessions: a visit counts when it
// started before `end` and either is still open or ended after `start`.
// Visits are restricted to each student's effective retention and enrollment
// windows. Minutes are clamped to those windows; a visit whose clamped span
// collapses to nothing drops out entirely. The peak is a sweep over entry (+1)
// and exit (-1) events per room; at equal instants the exit sorts first so
// a back-to-back room change never counts a child twice.
//
// The visiting child must be enrolled — the same rule the child table applies
// through users.EnrolledOn, or the two tables would report different
// populations. The eligible_student CTE spells it in SQL: alumni and
// bound-less inactive rows drop out, and the effective lower bound is
// enrolled_from except for an active child whose enrollment starts later,
// where immediate activation pulls it forward to today.
func (r *StatisticsRepository) RoomUtilization(ctx context.Context, start, end time.Time, today timezone.Date, groupIDs []int64) ([]active.RoomUtilizationRow, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, fmt.Errorf("statistics room utilization requires a tenant context")
	}
	// Placeholders below are positional, so the args are appended in the order
	// the SQL reads: the eligible_student CTE first (active status, today
	// twice, alumnus, inactive, tenant, then the optional group filter), then
	// the clamped CTE (GREATEST start, LEAST end, entry_time < end,
	// exit_time > start, two tenant filters).
	args := []any{
		string(userModels.StudentStatusActive), today, today,
		string(userModels.StudentStatusAlumnus), string(userModels.StudentStatusInactive),
		tenantID,
	}
	groupClause := ""
	if len(groupIDs) > 0 {
		positiveGroupIDs := make([]int64, 0, len(groupIDs))
		includeNoGroup := false
		for _, groupID := range groupIDs {
			if groupID == 0 {
				includeNoGroup = true
				continue
			}
			positiveGroupIDs = append(positiveGroupIDs, groupID)
		}
		switch {
		case includeNoGroup && len(positiveGroupIDs) > 0:
			groupClause = " AND (s.group_id IN (?) OR s.group_id IS NULL)"
			args = append(args, bun.List(positiveGroupIDs))
		case includeNoGroup:
			groupClause = " AND s.group_id IS NULL"
		default:
			groupClause = " AND s.group_id IN (?)"
			args = append(args, bun.List(positiveGroupIDs))
		}
	}
	args = append(args, start, end, end, start, tenantID, tenantID)

	sql := `
WITH eligible_student AS (
	SELECT s.id,
	       CASE
	           WHEN s.status = ? AND s.enrolled_from > ?::date THEN ?::date
	           ELSE s.enrolled_from
	       END AS effective_from,
	       s.enrolled_until
	FROM users.students s
	WHERE s.status <> ?
	  AND NOT (s.enrolled_from IS NULL AND s.enrolled_until IS NULL AND s.status = ?)
	  AND s.tenant_id = ?` + groupClause + `
),
retention AS (
	SELECT "privacy_consent".student_id,
	       MIN("privacy_consent".data_retention_days) AS retention_days
	FROM users.privacy_consents AS "privacy_consent"
	WHERE "privacy_consent".accepted = TRUE
	GROUP BY "privacy_consent".student_id
),
clamped AS (
	SELECT ag.room_id,
	       v.student_id,
	       GREATEST(
		       v.entry_time,
		       ?,
		       COALESCE(es.effective_from::timestamp AT TIME ZONE 'Europe/Berlin', '-infinity'::timestamptz)
	       ) AS entry_at,
	       LEAST(
		       COALESCE(v.exit_time, NOW()),
		       ?,
		       COALESCE((es.enrolled_until + 1)::timestamp AT TIME ZONE 'Europe/Berlin', 'infinity'::timestamptz)
	       ) AS exit_at
	FROM active.visits v
	JOIN active.groups ag ON ag.id = v.active_group_id
	JOIN eligible_student es ON es.id = v.student_id
	JOIN retention r ON r.student_id = v.student_id
	WHERE v.entry_time < ?
	  AND (v.exit_time IS NULL OR v.exit_time > ?)
	  AND (es.effective_from IS NULL OR COALESCE(v.exit_time, NOW()) > es.effective_from::timestamp AT TIME ZONE 'Europe/Berlin')
	  AND (es.enrolled_until IS NULL OR v.entry_time < (es.enrolled_until + 1)::timestamp AT TIME ZONE 'Europe/Berlin')
	  AND v.created_at >= NOW() - make_interval(days => r.retention_days)
	  AND v.tenant_id = ? AND ag.tenant_id = ?
),
scoped AS (
	-- A clamp can leave nothing: care that ended before the window start, or
	-- an enrollment starting after its end, pushes exit_at to or before
	-- entry_at. Such a visit is no presence in the window, so it must not
	-- count a child, a day, or a peak — an exit sorted before its own entry
	-- would leave the sweep permanently one child too high.
	SELECT room_id, student_id, entry_at, exit_at
	FROM clamped
	WHERE exit_at > entry_at
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
),
room_days AS (
	SELECT s.room_id,
	       COUNT(DISTINCT d.used_date) AS days_used
	FROM scoped s
	CROSS JOIN LATERAL generate_series(
		(s.entry_at AT TIME ZONE 'Europe/Berlin')::date,
		((s.exit_at - INTERVAL '1 microsecond') AT TIME ZONE 'Europe/Berlin')::date,
		INTERVAL '1 day'
	) AS d(used_date)
	GROUP BY s.room_id
)
SELECT s.room_id,
	   COALESCE(d.days_used, 0)::int AS days_used,
       COUNT(DISTINCT s.student_id) AS distinct_students,
       COALESCE(SUM(EXTRACT(EPOCH FROM (s.exit_at - s.entry_at))) / 60, 0)::int AS student_minutes,
       COALESCE(p.peak_occupancy, 0) AS peak_occupancy
FROM scoped s
LEFT JOIN peak p ON p.room_id = s.room_id
LEFT JOIN room_days d ON d.room_id = s.room_id
GROUP BY s.room_id, d.days_used, p.peak_occupancy
ORDER BY s.room_id`
	var rows []active.RoomUtilizationRow
	if err := base.GetDB(ctx, r.db).NewRaw(sql, args...).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "statistics room utilization", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
