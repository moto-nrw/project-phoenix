package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableInstanceStaff    = "schedule.instance_staff"
	aliasInstanceStaff    = "instance_staff"
	modelTblInstanceStaff = `schedule.instance_staff AS "instance_staff"`
)

var errInstanceStaffNil = fmt.Errorf("instance staff assignment cannot be nil")

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

// Create inserts a new instance staff row after running model-level validation.
func (r *InstanceStaffRepository) Create(ctx context.Context, s *schedule.InstanceStaff) error {
	if s == nil {
		return errInstanceStaffNil
	}
	if err := s.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, s)
}

// Update writes the given instance staff row back to the database.
func (r *InstanceStaffRepository) Update(ctx context.Context, s *schedule.InstanceStaff) error {
	if s == nil {
		return errInstanceStaffNil
	}
	if err := s.Validate(); err != nil {
		return err
	}
	return r.Repository.Update(ctx, s)
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *InstanceStaffRepository) FindByID(ctx context.Context, id any) (*schedule.InstanceStaff, error) {
	var row schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStaff); ok {
		query = query.Where(where, val)
	}

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
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStaff); ok {
		query = query.Where(where, val)
	}

	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}
	return rows, nil
}

// FindByInstanceID returns all staff assignments for an instance.
func (r *InstanceStaffRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedule.InstanceStaff, error) {
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Order(orderCreatedAtASC)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStaff); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance id",
			Err: err,
		}
	}
	return rows, nil
}

// FindByStaffAndDate returns all staff assignments for a staff member across
// all instances on the given date.
func (r *InstanceStaffRepository) FindByStaffAndDate(ctx context.Context, staffID int64, date time.Time) ([]*schedule.InstanceStaff, error) {
	var rows []*schedule.InstanceStaff
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStaff).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_staff".instance_id`).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where(`"activity_instance".date = ?`, date).
		Order(`"activity_instance".start_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStaff); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff and date",
			Err: err,
		}
	}
	return rows, nil
}

// DeleteByInstanceID removes all staff assignments for an instance. Used when
// re-materializing an instance without deleting the parent row.
func (r *InstanceStaffRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.InstanceStaff)(nil)).
		ModelTableExpr(modelTblInstanceStaff).
		Where(`"instance_staff".instance_id = ?`, instanceID)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStaff); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by instance id",
			Err: err,
		}
	}
	return nil
}
