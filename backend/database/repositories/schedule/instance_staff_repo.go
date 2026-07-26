package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableInstanceStaff    = "schedule.instance_staff"
	aliasInstanceStaff    = "instance_staff"
	modelTblInstanceStaff = `schedule.instance_staff AS "instance_staff"`
)

// InstanceStaffRepository implements schedule.InstanceStaffRepository.
type InstanceStaffRepository struct {
	*base.Repository[*schedule.InstanceStaff]
	db *bun.DB
}

// NewInstanceStaffRepository creates a new InstanceStaffRepository.
func NewInstanceStaffRepository(db *bun.DB) schedule.InstanceStaffRepository {
	repo := base.NewRepository[*schedule.InstanceStaff](db, tableInstanceStaff, "InstanceStaff")
	repo.TenantScoped = true
	return &InstanceStaffRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *InstanceStaffRepository) FindByID(ctx context.Context, id any) (*schedule.InstanceStaff, error) {
	var row schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}
	return &row, nil
}

// List retrieves instance staff rows matching the provided query options.
func (r *InstanceStaffRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.InstanceStaff, error) {
	return r.ListWithOptions(ctx, options)
}

// FindByInstanceID returns all staff assignments for an instance.
func (r *InstanceStaffRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedule.InstanceStaff, error) {
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		OrderExpr(`"instance_staff".created_at ASC, "instance_staff".id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance id",
			Err: err,
		}
	}
	return rows, nil
}

// FindByInstanceIDs returns every instance_staff row for any of the given
// instance IDs. Tenant-scoped. Empty input returns an empty slice without
// hitting the DB, matching the sibling bulk helpers (see
// FindExpectedByInstanceIDs in instance_student_repo). Custom method
// (backend-conventions Rule 2): bulk IN lookup the generic List(filters)
// shape cannot express without per-call map allocation games; required by the
// planned-conflict probe to stay O(1) queries for N overlapping instances.
func (r *InstanceStaffRepository) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedule.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*schedule.InstanceStaff{}, nil
	}
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".instance_id IN (?)`, bun.List(instanceIDs)).
		OrderExpr(`"instance_staff".instance_id ASC, "instance_staff".created_at ASC, "instance_staff".id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance ids",
			Err: err,
		}
	}
	return rows, nil
}

// FindByStaffAndDate returns all staff assignments for a staff member across
// all instances on the given date.
func (r *InstanceStaffRepository) FindByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*schedule.InstanceStaff, error) {
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_staff".instance_id`).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"activity_instance".date = ?`, date).
		OrderExpr(`"activity_instance".start_time ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff and date",
			Err: err,
		}
	}
	return rows, nil
}

// FindByStaffAndDateRange returns the staff member's assignments across all
// instances dated within [from, to] inclusive, ordered by instance date then
// start time. Custom method (backend-conventions Rule 2): the date predicate
// lives on the joined activity_instances table, which the generic filter
// shape cannot express. Keeps the self-service assignment read (#1844)
// proportional to one staff member's plan instead of the whole tenant window.
func (r *InstanceStaffRepository) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*schedule.InstanceStaff, error) {
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_staff".instance_id`).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC, "instance_staff".id ASC`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff and date range",
			Err: err,
		}
	}
	return rows, nil
}

// DeleteUpcomingByStaffID removes the staff member's assignments on instances
// dated strictly after the given date, plus same-day instances that are still
// 'planned' — those would otherwise be copied into active.group_supervisors
// when the instance starts. Same-day instances that already ran (or run right
// now) keep their rows as history. Used by staff offboarding, where the staff
// row is only soft-deleted and the RESTRICT FK no longer applies.
func (r *InstanceStaffRepository) DeleteUpcomingByStaffID(ctx context.Context, staffID int64, after timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.InstanceStaff)(nil)).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"instance_staff".instance_id IN (
			SELECT id FROM schedule.activity_instances WHERE date > ? OR (date = ? AND status = ?)
		)`, after, after, schedule.InstanceStatusPlanned)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete upcoming by staff id",
			Err: err,
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// CountNonAbsentByInstanceIDs groups instance_staff by instance_id and returns
// the count of rows with is_absent=false per instance. One query with GROUP BY.
// Instances with zero non-absent rows do not appear in the returned map —
// callers must treat missing keys as zero.
func (r *InstanceStaffRepository) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	if len(instanceIDs) == 0 {
		return map[int64]int{}, nil
	}

	var rows []struct {
		InstanceID int64 `bun:"instance_id"`
		Cnt        int   `bun:"cnt"`
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(modelTblInstanceStaff).
		ColumnExpr(`"instance_staff".instance_id AS instance_id`).
		ColumnExpr(`COUNT(*)::int AS cnt`).
		Where(`"instance_staff".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_staff".is_absent = ?`, false).
		GroupExpr(`"instance_staff".instance_id`)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count non-absent by instance ids",
			Err: err,
		}
	}

	out := make(map[int64]int, len(rows))
	for _, row := range rows {
		out[row.InstanceID] = row.Cnt
	}
	return out, nil
}

// DeleteByInstanceID removes all staff assignments for an instance. Used when
// re-materializing an instance without deleting the parent row.
func (r *InstanceStaffRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.InstanceStaff)(nil)).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".instance_id = ?`, instanceID)

	query = base.WithTenantFilter(ctx, query, aliasInstanceStaff)

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by instance id",
			Err: err,
		}
	}
	return nil
}
