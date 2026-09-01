package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const tableCalendarPeriods = "schedule.calendar_periods"

var errCalendarPeriodNil = fmt.Errorf("calendar period cannot be nil")

// CalendarPeriodRepository implements schedule.CalendarPeriodRepository
type CalendarPeriodRepository struct {
	*base.Repository[*schedule.CalendarPeriod]
	db *bun.DB
}

// NewCalendarPeriodRepository creates a new CalendarPeriodRepository
func NewCalendarPeriodRepository(db *bun.DB) schedule.CalendarPeriodRepository {
	repo := base.NewRepository[*schedule.CalendarPeriod](db, tableCalendarPeriods, "CalendarPeriod")
	repo.TenantScoped = true
	return &CalendarPeriodRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByTenantID finds all calendar periods for the current tenant
func (r *CalendarPeriodRepository) FindByTenantID(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant id",
			Err: base.TranslateNotFound(err),
		}
	}

	return periods, nil
}

// FindActiveByTenantID finds all active calendar periods for the current tenant
func (r *CalendarPeriodRepository) FindActiveByTenantID(ctx context.Context) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".is_active = ?`, true).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by tenant id",
			Err: base.TranslateNotFound(err),
		}
	}

	return periods, nil
}

// FindByName finds a calendar period by name within the current tenant
func (r *CalendarPeriodRepository) FindByName(ctx context.Context, name string) (*schedule.CalendarPeriod, error) {
	var period schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&period).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".name = ?`, name)

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: base.TranslateNotFound(err),
		}
	}

	return &period, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *CalendarPeriodRepository) FindByID(ctx context.Context, id any) (*schedule.CalendarPeriod, error) {
	var period schedule.CalendarPeriod

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&period).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: base.TranslateNotFound(err),
		}
	}

	return &period, nil
}

// CreateIfAbsent inserts the period unless a row with the same
// (tenant_id, name) already exists (unique_calendar_period_name, migration
// 1.15.33). It returns true when a new row was inserted and false when a
// period with that name was already present.
//
// Why a custom method instead of the generic Create: the school-year
// bootstrap (WP-B1) must be idempotent under concurrent requests. A
// read-then-insert sequence can race (two parallel bootstraps both see
// "no period" and both insert — one fails with a unique violation that
// would also abort the request's tenant transaction). ON CONFLICT
// DO NOTHING is the only race-free shape: exactly one insert wins, the
// loser is a clean no-op, and the transaction stays usable. Mirrors
// base.Create's validate + tenant-autoset + ModelTableExpr insert,
// adding only the conflict clause.
func (r *CalendarPeriodRepository) CreateIfAbsent(ctx context.Context, p *schedule.CalendarPeriod) (bool, error) {
	if p == nil {
		return false, errCalendarPeriodNil
	}
	if err := p.Validate(); err != nil {
		return false, err
	}
	base.EnsureTenantID(ctx, p)

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(p).
		ModelTableExpr(tableCalendarPeriods).
		On("CONFLICT (tenant_id, name) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create if absent",
			Err: base.TranslateNotFound(err),
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create if absent (rows affected)",
			Err: base.TranslateNotFound(err),
		}
	}

	return affected == 1, nil
}

// FindActiveOverlapping returns all active periods of the current tenant
// whose [start_date, end_date] range overlaps [start, end] (inclusive on
// both ends), excluding the period with excludeID. Adjacent ranges that
// merely touch outside the window (end_date < start or start_date > end)
// do not count as overlapping.
func (r *CalendarPeriodRepository) FindActiveOverlapping(ctx context.Context, start, end timezone.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".is_active = ?`, true).
		Where(`"calendar_period".start_date <= ?`, end).
		Where(`"calendar_period".end_date >= ?`, start).
		Where(`"calendar_period".id != ?`, excludeID).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active overlapping",
			Err: base.TranslateNotFound(err),
		}
	}

	return periods, nil
}

// FindActiveOverlappingByType behaves like FindActiveOverlapping but only
// considers active periods of the given period_type. It backs the hard
// same-type overlap rejection (#1837); the untyped variant stays advisory.
func (r *CalendarPeriodRepository) FindActiveOverlappingByType(ctx context.Context, periodType string, start, end timezone.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	var periods []*schedule.CalendarPeriod
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&periods).
		ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`).
		Where(`"calendar_period".is_active = ?`, true).
		Where(`"calendar_period".period_type = ?`, periodType).
		Where(`"calendar_period".start_date <= ?`, end).
		Where(`"calendar_period".end_date >= ?`, start).
		Where(`"calendar_period".id != ?`, excludeID).
		Order("start_date ASC")

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active overlapping by type",
			Err: base.TranslateNotFound(err),
		}
	}

	return periods, nil
}

// UsageCounts returns, per calendar period of the current tenant, how many
// rows reference it through nullable calendar_period_id FKs. Correlated
// subqueries (pattern: facilities/room.go occupancyColumns) keep this a
// single round-trip; explicit tenant predicates avoid cross-tenant counts
// even when the caller is a superuser test connection. Periods without
// references are omitted from the map.
func (r *CalendarPeriodRepository) UsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	var rows []struct {
		ID                     int64 `bun:"id"`
		PhaseCount             int   `bun:"phase_count"`
		ActivityGroupCount     int   `bun:"activity_group_count"`
		ScheduleCount          int   `bun:"schedule_count"`
		StudentEnrollmentCount int   `bun:"student_enrollment_count"`
		SupervisorCount        int   `bun:"supervisor_count"`
		ActivityInstanceCount  int   `bun:"activity_instance_count"`
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`schedule.calendar_periods AS "calendar_period"`).
		ColumnExpr(`"calendar_period".id AS id`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM enrollment.phases p
			WHERE p.calendar_period_id = "calendar_period".id
			  AND p.tenant_id = "calendar_period".tenant_id
		)::int AS phase_count`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM activities.groups g
			WHERE g.calendar_period_id = "calendar_period".id
			  AND g.tenant_id = "calendar_period".tenant_id
		)::int AS activity_group_count`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM activities.schedules s
			WHERE s.calendar_period_id = "calendar_period".id
			  AND s.tenant_id = "calendar_period".tenant_id
		)::int AS schedule_count`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM activities.student_enrollments se
			WHERE se.calendar_period_id = "calendar_period".id
			  AND se.tenant_id = "calendar_period".tenant_id
		)::int AS student_enrollment_count`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM activities.supervisors sp
			WHERE sp.calendar_period_id = "calendar_period".id
			  AND sp.tenant_id = "calendar_period".tenant_id
		)::int AS supervisor_count`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM schedule.activity_instances ai
			WHERE ai.calendar_period_id = "calendar_period".id
			  AND ai.tenant_id = "calendar_period".tenant_id
		)::int AS activity_instance_count`)

	query = base.WithTenantFilter(ctx, query, "calendar_period")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "usage counts",
			Err: base.TranslateNotFound(err),
		}
	}

	usage := make(map[int64]schedule.CalendarPeriodUsage, len(rows))
	for _, row := range rows {
		if row.PhaseCount == 0 &&
			row.ActivityGroupCount == 0 &&
			row.ScheduleCount == 0 &&
			row.StudentEnrollmentCount == 0 &&
			row.SupervisorCount == 0 &&
			row.ActivityInstanceCount == 0 {
			continue
		}
		usage[row.ID] = schedule.CalendarPeriodUsage{
			EnrollmentPhases:   row.PhaseCount,
			ActivityGroups:     row.ActivityGroupCount,
			Schedules:          row.ScheduleCount,
			StudentEnrollments: row.StudentEnrollmentCount,
			Supervisors:        row.SupervisorCount,
			ActivityInstances:  row.ActivityInstanceCount,
		}
	}
	return usage, nil
}

// List retrieves calendar periods matching the provided query options
func (r *CalendarPeriodRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.CalendarPeriod, error) {
	return r.ListWithOptions(ctx, options)
}
