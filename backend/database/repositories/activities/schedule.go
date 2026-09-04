// backend/database/repositories/activities/schedule.go
package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesSchedules          = "activities.schedules"
	tableExprActivitiesSchedulesAsSch = `activities.schedules AS "schedule"`
)

// ScheduleRepository implements activities.ScheduleRepository interface
type ScheduleRepository struct {
	*base.Repository[*activities.Schedule]
	db *bun.DB
}

// NewScheduleRepository creates a new ScheduleRepository
func NewScheduleRepository(db *bun.DB) activities.ScheduleRepository {
	repo := base.NewRepository[*activities.Schedule](db, tableActivitiesSchedules, "Schedule")
	repo.TenantScoped = true
	return &ScheduleRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByGroupID finds all schedules for a specific group
func (r *ScheduleRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		// Removed Timeframe relation since it's not properly defined in the model
		Where("activity_group_id = ?", groupID)

	query = base.WithTenantFilter(ctx, query, "schedule")

	err := query.
		Order("weekday").
		Order("timeframe_id").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return schedules, nil
}

// FindByWeekday finds all schedules for a specific weekday
func (r *ScheduleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*activities.Schedule, error) {
	var schedules []*activities.Schedule
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		// Note: ActivityGroup relation is commented out in model, so we can't use Relation()
		// The caller should load ActivityGroup separately if needed
		Where("weekday = ?", weekday)

	query = base.WithTenantFilter(ctx, query, "schedule")

	err := query.
		Order("timeframe_id").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by weekday",
			Err: base.TranslateNotFound(err),
		}
	}

	return schedules, nil
}

// FindTemplateStartTimesByGroupIDs returns (activity_group_id, weekday,
// timeframe.start_time) tuples for the given group IDs. Joins
// activities.schedules to schedule.timeframes and skips rows without a
// linked timeframe. Both tables are tenant-scoped in the WHERE clause as
// defense-in-depth alongside RLS. Returns an empty slice on empty input.
//
// The caller (WP-B13 conflict-warning handler) indexes the result by
// (ActivityGroupID, Weekday) and flags ambiguity when a template has more
// than one schedule on the same weekday.
func (r *ScheduleRepository) FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activities.TemplateStartTime, error) {
	if len(groupIDs) == 0 {
		return []*activities.TemplateStartTime{}, nil
	}

	// TemplateStartTime is a plain DTO and Bun's ORM layer struggles to
	// build a valid FROM clause without a registered model, so this query
	// uses a raw SQL statement and manual row scanning. The belt-and-
	// suspenders tenant filter (RLS + explicit WHERE) is kept intact.
	const query = `
		SELECT s.activity_group_id, s.weekday, TO_CHAR(t.start_time, 'HH24:MI:SS') AS start_time
		FROM activities.schedules AS s
		INNER JOIN schedule.timeframes AS t ON t.id = s.timeframe_id
		WHERE s.activity_group_id IN (?)
		  AND s.timeframe_id IS NOT NULL
		  AND s.tenant_id = ? AND t.tenant_id = ?
		ORDER BY s.activity_group_id ASC, s.weekday ASC, t.start_time ASC`
	tenantID := tenant.FromContext(ctx)

	rows, err := base.GetDB(ctx, r.db).QueryContext(ctx, query, bun.List(groupIDs), tenantID, tenantID)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find template start times by group ids",
			Err: base.TranslateNotFound(err),
		}
	}
	defer func() { _ = rows.Close() }()

	out := make([]*activities.TemplateStartTime, 0)
	for rows.Next() {
		var gid int64
		var wd int
		var raw string
		if err := rows.Scan(&gid, &wd, &raw); err != nil {
			return nil, &modelBase.DatabaseError{
				Op:  "find template start times by group ids: scan",
				Err: base.TranslateNotFound(err),
			}
		}
		startTime, err := parseSQLClock(raw)
		if err != nil {
			return nil, &modelBase.DatabaseError{
				Op:  "find template start times by group ids: parse clock",
				Err: base.TranslateNotFound(err),
			}
		}
		out = append(out, &activities.TemplateStartTime{
			ActivityGroupID: gid,
			Weekday:         wd,
			StartTime:       startTime,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find template start times by group ids: rows",
			Err: base.TranslateNotFound(err),
		}
	}
	return out, nil
}

func parseSQLClock(raw string) (time.Time, error) {
	parsed, err := time.Parse("15:04:05", raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(1, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), time.UTC), nil
}

// CapValidUntil ends the recurrence of every schedule of the given template
// at validUntil (exclusive): open-ended rows (valid_until IS NULL) and rows
// ending later than validUntil are capped to validUntil. Returns the number
// of rows changed. Custom method (backend-conventions Rule 2): multi-row,
// multi-predicate bulk update for the template split (WP-B3) — not
// expressible via the generic per-entity Update.
func (r *ScheduleRepository) CapValidUntil(ctx context.Context, activityGroupID int64, validUntil timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.Schedule)(nil)).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		Set("valid_until = ?", validUntil).
		Where(`"schedule".activity_group_id = ?`, activityGroupID).
		Where(`("schedule".valid_until IS NULL OR "schedule".valid_until > ?)`, validUntil)

	query = base.WithTenantFilter(ctx, query, "schedule")

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "cap schedule valid_until",
			Err: base.TranslateNotFound(err),
		}
	}
	rows, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
	return rows, nil
}

// Update overrides the base Update method to handle validation
func (r *ScheduleRepository) Update(ctx context.Context, schedule *activities.Schedule) error {
	if schedule == nil {
		return fmt.Errorf("schedule cannot be nil")
	}

	// Validate schedule
	if err := schedule.Validate(); err != nil {
		return err
	}

	// Get the query builder - GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(schedule).
		Where(whereIDEquals, schedule.ID).
		ModelTableExpr(tableActivitiesSchedules)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Execute the query
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update schedule")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *ScheduleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Schedule, error) {
	return r.ListWithOptions(ctx, options)
}

// DeleteByGroupID removes all schedules of an activity group (issue #584:
// moved verbatim from api/timetable's template update). Custom method
// (Rule 2): the generic Delete works on a single primary key — a bulk
// delete by foreign key plus tenant filter is not expressible through it.
func (r *ScheduleRepository) DeleteByGroupID(ctx context.Context, groupID int64) error {
	tenantID := tenant.FromContext(ctx)
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Table("activities.schedules").
		Where("tenant_id = ?", tenantID).
		Where("activity_group_id = ?", groupID).
		Exec(ctx)
	return err
}
