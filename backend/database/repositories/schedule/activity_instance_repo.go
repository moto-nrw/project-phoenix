package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableActivityInstances   = "schedule.activity_instances"
	aliasActivityInstance    = "activity_instance"
	modelTblActivityInstance = `schedule.activity_instances AS "activity_instance"`
)

var errActivityInstanceNil = fmt.Errorf("activity instance cannot be nil")

// ActivityInstanceRepository implements schedule.ActivityInstanceRepository.
type ActivityInstanceRepository struct {
	*base.Repository[*schedule.ActivityInstance]
	db *bun.DB
}

// NewActivityInstanceRepository creates a new ActivityInstanceRepository.
func NewActivityInstanceRepository(db *bun.DB) schedule.ActivityInstanceRepository {
	repo := base.NewRepository[*schedule.ActivityInstance](db, tableActivityInstances, "ActivityInstance")
	repo.TenantScoped = true
	return &ActivityInstanceRepository{
		Repository: repo,
		db:         db,
	}
}

// CreateTemplateBackedIfAbsent inserts a template-backed activity instance,
// absorbing the duplicate-template-slot race at the database layer. Catching a
// 23505 after a plain INSERT would leave the surrounding PostgreSQL
// transaction aborted, so materialization must use ON CONFLICT DO NOTHING.
func (r *ActivityInstanceRepository) CreateTemplateBackedIfAbsent(ctx context.Context, i *schedule.ActivityInstance) (bool, error) {
	if i == nil {
		return false, errActivityInstanceNil
	}
	if i.ActivityGroupID == nil {
		return false, fmt.Errorf("activity_group_id is required for template-backed insert")
	}
	if err := i.Validate(); err != nil {
		return false, err
	}

	if i.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			i.SetTenantID(tid)
		}
	}

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(i).
		ModelTableExpr(tableActivityInstances).
		On("CONFLICT (tenant_id, date, activity_group_id, start_time) WHERE activity_group_id IS NOT NULL DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create template-backed if absent",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "create template-backed if absent rows affected",
			Err: err,
		}
	}
	return affected > 0, nil
}

// MarkCompleted updates only lifecycle columns. It intentionally avoids the
// generic full-row Update path because SQL TIME columns on activity instances
// are unsafe to write back after a DB decode round-trip.
func (r *ActivityInstanceRepository) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	inst := &schedule.ActivityInstance{
		Status:      schedule.InstanceStatusCompleted,
		CompletedAt: &completedAt,
	}
	inst.ID = instanceID

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model(inst).
		ModelTableExpr(modelTblActivityInstance).
		Column("status", "completed_at").
		Where(`"activity_instance".id = ?`, instanceID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		q = q.Where(`"activity_instance".tenant_id = ?`, tenantID)
	}

	result, err := q.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark completed", Err: err}
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return &modelBase.DatabaseError{Op: "mark completed", Err: sql.ErrNoRows}
	}
	return nil
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *ActivityInstanceRepository) FindByID(ctx context.Context, id any) (*schedule.ActivityInstance, error) {
	var instance schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instance).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}
	return &instance, nil
}

// List retrieves activity instances matching the provided query options.
func (r *ActivityInstanceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ActivityInstance, error) {
	return r.ListWithOptions(ctx, options)
}

// FindByTenantAndDate returns instances for the current tenant on a given date.
func (r *ActivityInstanceRepository) FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant and date",
			Err: err,
		}
	}
	return instances, nil
}

// FindPlannedTemplateBackedFrom returns planned instances dated on or after
// `from` for the current tenant that were MATERIALIZED from a template. Feeds
// the grade-transition revert reconciliation (re-adding restored students the
// materializer skipped).
//
// "Materialized" is narrower than "has an activity_group_id". A planner can
// create a one-off block by hand and link it to a template for its metadata
// (CreateInstanceInput.ActivityGroupID); its roster is then whatever student
// IDs that person submitted, NOT a copy of the template's enrollments. Filling
// such an instance from enrollments would add children the planner never put
// there. Only the materializer writes calendar_period_id together with
// is_spontaneous = false — the manual create path leaves the period NULL — so
// that pair is the marker for "this roster came from enrollments" (#405
// review).
//
// The date bound is inclusive so an instance materialized during the alumnus
// window and dated today is reconciled too: it carries no archive row (the
// materializer never created the alumnus's row), so nothing else can repair it
// afterwards. Instances that already started or finished today drop out via the
// planned-status filter (#405 review).
func (r *ActivityInstanceRepository) FindPlannedTemplateBackedFrom(ctx context.Context, from timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusPlanned).
		Where(`"activity_instance".activity_group_id IS NOT NULL`).
		Where(`"activity_instance".calendar_period_id IS NOT NULL`).
		Where(`"activity_instance".is_spontaneous = FALSE`).
		Order("date ASC", "start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find planned template-backed instances from date",
			Err: err,
		}
	}
	return instances, nil
}

// MaxID returns the highest instance id visible to the current tenant, or 0 if
// the tenant has none. See the interface doc for why the grade transition uses
// an id high-water mark instead of created_at (#405 review).
func (r *ActivityInstanceRepository) MaxID(ctx context.Context) (int64, error) {
	var maxID sql.NullInt64
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(modelTblActivityInstance).
		ColumnExpr(`MAX("activity_instance".id)`)

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	if err := query.Scan(ctx, &maxID); err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get max activity instance id",
			Err: err,
		}
	}
	return maxID.Int64, nil
}

// FindByTenantAndDateRange returns instances within an inclusive date range.
func (r *ActivityInstanceRepository) FindByTenantAndDateRange(ctx context.Context, from, to timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		Order("date ASC", "start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tenant and date range",
			Err: err,
		}
	}
	return instances, nil
}

// FindByIDs returns the instances matching ids in one tenant-scoped IN query.
// Custom method (backend-conventions Rule 2): bulk IN lookup with the
// empty-slice short-circuit callers rely on to skip the round trip entirely,
// mirroring InstanceStaffRepository.FindByInstanceIDs.
func (r *ActivityInstanceRepository) FindByIDs(ctx context.Context, ids []int64) ([]*schedule.ActivityInstance, error) {
	instances := make([]*schedule.ActivityInstance, 0, len(ids))
	if len(ids) == 0 {
		return instances, nil
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".id IN (?)`, bun.List(ids)).
		Order("date ASC", "start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by ids",
			Err: err,
		}
	}
	return instances, nil
}

// FindByActivityGroupAndDate returns instances for a template on a given date.
func (r *ActivityInstanceRepository) FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date timezone.Date) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".activity_group_id = ?`, activityGroupID).
		Where(`"activity_instance".date = ?`, date).
		Order("start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group and date",
			Err: err,
		}
	}
	return instances, nil
}

// FindByActivityGroupAndDateRange returns one template's instances within an
// inclusive range. This custom read is necessary because the generic exact-
// filter shape cannot express inclusive date bounds; the group predicate is
// applied in SQL so a long-running probe never loads every tenant block.
func (r *ActivityInstanceRepository) FindByActivityGroupAndDateRange(
	ctx context.Context,
	activityGroupID int64,
	from, to timezone.Date,
) ([]*schedule.ActivityInstance, error) {
	var instances []*schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instances).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".activity_group_id = ?`, activityGroupID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		Order("date ASC", "start_time ASC")

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by activity group and date range",
			Err: err,
		}
	}
	return instances, nil
}

// FindByActiveGroupID returns the instance bridged to the given active.group,
// or nil if none exists. The bridge is 1:1 — enforced by a UNIQUE partial
// index on schedule.activity_instances(active_group_id) WHERE IS NOT NULL.
// The Limit(1) is belt-and-suspenders for windows where the index is not yet
// present (e.g. a test DB restored from a pre-fix dump).
func (r *ActivityInstanceRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*schedule.ActivityInstance, error) {
	var instance schedule.ActivityInstance
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&instance).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".active_group_id = ?`, activeGroupID)

	query = base.WithTenantFilter(ctx, query, aliasActivityInstance)

	err := query.Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group id",
			Err: err,
		}
	}
	return &instance, nil
}

// CompleteActiveByActiveGroupIDs marks every still-active instance bridged to
// one of the given active.groups as completed and returns the number of rows
// changed. Custom method (backend-conventions Rule 2): lifecycle bulk update
// keyed on the active-group bridge, paired with MarkCompleted's
// column-restricted shape. Used by the scheduler's daily session-end bridge
// and by kiosk/session end. Recovery state is deliberately omitted: those
// completions are the end of the live session, not a planner undo.
func (r *ActivityInstanceRepository) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(modelTblActivityInstance).
		Set(`status = ?`, schedule.InstanceStatusCompleted).
		Set(`completed_at = ?`, completedAt).
		Set(`updated_at = ?`, completedAt).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusActive).
		Where(`"activity_instance".active_group_id IN (?)`, bun.List(activeGroupIDs))

	q = base.WithTenantFilter(ctx, q, aliasActivityInstance)

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "complete active instances by active group ids",
			Err: err,
		}
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "complete active instances by active group ids",
			Err: err,
		}
	}
	return rows, nil
}

// DeletePlannedNonSpontaneousInWindow removes every still-planned,
// template-backed instance whose date falls in the inclusive [from, to]
// window and returns the number of rows deleted. A nil `to` makes the window
// open-ended (date >= from, no upper bound) — used by the template split so
// the old template's planned instances are purged from the effective date
// onward regardless of the materialization horizon. A non-nil
// activityGroupID narrows the delete to that template's instances (template
// split, WP-B3); nil deletes across all templates (ReplanWeek over the whole
// grid). CASCADE removes the instance_staff / instance_students children
// (declared at DDL level). Custom method (backend-conventions Rule 2):
// multi-predicate lifecycle delete for the ReplanWeek admin action.
//
// #1840: `preserveDeviations` controls whether instances carrying a
// Vertretungsplan deviation are kept. Only re-plan (ReplanWeek) passes true:
// re-plan regenerates the week from the base plan but must NOT wipe the admin's
// manual overrides, so a deviated row (an acknowledged shortfall, or any
// instance_staff row marked absent / substitute / bearing an absence reason) is
// preserved and skipped by the insert-only re-materialization.
//
// Template split and template end pass false: both are the destructive
// "this and all following" series operation. Preserving a deviated old-template
// row there is wrong — for END it would leave the "ended" series partly alive,
// and for SPLIT the surviving old row (activity_group_id = old template) plus
// the freshly materialized successor row (activity_group_id = new template)
// dedupe apart and the user sees a duplicate block. So the series operation
// hard-deletes every still-planned non-spontaneous row in the window,
// deviations included; started/completed/cancelled/spontaneous rows always
// survive regardless of this flag.
func (r *ActivityInstanceRepository) DeletePlannedNonSpontaneousInWindow(ctx context.Context, from timezone.Date, to *timezone.Date, activityGroupID *int64, preserveDeviations bool) (int64, error) {
	q := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusPlanned).
		Where(`"activity_instance".is_spontaneous = ?`, false)

	if preserveDeviations {
		// Keep deviated instances (#1840, re-plan only). RLS already scopes
		// instance_staff to the tenant; the explicit tenant match keeps the
		// correlation safe even under a superuser connection.
		q = q.
			Where(`"activity_instance".understaffed_ack = ?`, false).
			Where(`NOT EXISTS (
				SELECT 1 FROM schedule.instance_staff AS "dev_is"
				WHERE "dev_is".instance_id = "activity_instance".id
				  AND "dev_is".tenant_id = "activity_instance".tenant_id
				  AND ("dev_is".is_absent OR "dev_is".is_substitute OR "dev_is".absence_reason IS NOT NULL)
			)`)
	}

	if to != nil {
		q = q.Where(`"activity_instance".date <= ?`, *to)
	}

	if activityGroupID != nil {
		q = q.Where(`"activity_instance".activity_group_id = ?`, *activityGroupID)
	}

	q = base.WithTenantFilter(ctx, q, aliasActivityInstance)

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete planned non-spontaneous in window",
			Err: err,
		}
	}
	deleted, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
	return deleted, nil
}

// DeletePlannedMaterializedWeekendInstances removes future materialized rows
// for weekdays removed from a legacy template. It deliberately leaves manual,
// active, cancelled, and historical rows untouched.
func (r *ActivityInstanceRepository) DeletePlannedMaterializedWeekendInstances(ctx context.Context, activityGroupID int64, weekdays []int) (int64, error) {
	if len(weekdays) == 0 {
		return 0, nil
	}
	q := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(modelTblActivityInstance).
		Where(`"activity_instance".activity_group_id = ?`, activityGroupID).
		Where(`"activity_instance".calendar_period_id IS NOT NULL`).
		Where(`"activity_instance".date > ?`, timezone.TodayDate()).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusPlanned).
		Where(`"activity_instance".is_spontaneous = ?`, false).
		Where(`EXTRACT(ISODOW FROM "activity_instance".date)::int IN (?)`, bun.List(weekdays))
	q = base.WithTenantFilter(ctx, q, aliasActivityInstance)
	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete removed legacy weekend instances", Err: err}
	}
	deleted, _ := res.RowsAffected()
	return deleted, nil
}

// PropagateListKindToFutureInstances re-classifies the future template-backed
// planned instances of one template whose list_kind still matches the series'
// previous value, returning the number of rows changed. It closes the gap where
// editing a series' Listenart (activities.groups.list_kind) left already
// materialized future occurrences carrying the stale value the classified daily
// lists filter on, so the series stayed absent from its list until a manual
// re-plan (#1565 review).
//
// Only rows a series edit is allowed to touch are updated:
//   - date strictly after `after` (today): the materialization/re-plan invariant
//     that the current and past days are never rewritten is preserved, so a list
//     already in use today is not reshuffled underneath its readers;
//   - status = 'planned' and is_spontaneous = false: started/completed/cancelled
//     and spontaneous rows are never rewritten by a series edit;
//   - list_kind still equal to `previousKind` (NULL and empty treated alike): an
//     occurrence individually re-classified via the instance PUT diverges from
//     the series value and is a single-occurrence edit (EditedChangeListKind that
//     ReplanWeek reports as lost) — it is left untouched.
//
// Custom method (backend-conventions Rule 2): a multi-predicate series-scoped
// column rewrite the generic filter API cannot express.
func (r *ActivityInstanceRepository) PropagateListKindToFutureInstances(
	ctx context.Context,
	activityGroupID int64,
	previousKind *string,
	newKind *string,
	after timezone.Date,
) (int64, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.ActivityInstance)(nil)).
		ModelTableExpr(modelTblActivityInstance).
		Set(`list_kind = ?`, newKind).
		Set(`updated_at = ?`, time.Now()).
		Where(`"activity_instance".activity_group_id = ?`, activityGroupID).
		Where(`"activity_instance".date > ?`, after).
		Where(`"activity_instance".status = ?`, schedule.InstanceStatusPlanned).
		Where(`"activity_instance".is_spontaneous = ?`, false).
		Where(`COALESCE("activity_instance".list_kind, '') = COALESCE(?, '')`, previousKind)

	q = base.WithTenantFilter(ctx, q, aliasActivityInstance)

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "propagate list kind to future instances",
			Err: err,
		}
	}
	updated, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
	return updated, nil
}
