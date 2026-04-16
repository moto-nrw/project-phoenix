package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

const (
	tableInstanceStudents   = "schedule.instance_students"
	aliasInstanceStudent    = "instance_student"
	modelTblInstanceStudent = `schedule.instance_students AS "instance_student"`
)

var errInstanceStudentNil = fmt.Errorf("instance student row cannot be nil")

// InstanceStudentRepository implements schedule.InstanceStudentRepository.
type InstanceStudentRepository struct {
	*base.Repository[*schedule.InstanceStudent]
	db *bun.DB
}

// NewInstanceStudentRepository creates a new InstanceStudentRepository.
func NewInstanceStudentRepository(db *bun.DB) schedule.InstanceStudentRepository {
	repo := base.NewRepository[*schedule.InstanceStudent](db, tableInstanceStudents, "InstanceStudent")
	repo.TenantScoped = true
	return &InstanceStudentRepository{
		Repository: repo,
		db:         db,
	}
}

// Create inserts a new instance student row after running model-level validation.
func (r *InstanceStudentRepository) Create(ctx context.Context, s *schedule.InstanceStudent) error {
	if s == nil {
		return errInstanceStudentNil
	}
	if err := s.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, s)
}

// Update writes the given instance student row back to the database.
func (r *InstanceStudentRepository) Update(ctx context.Context, s *schedule.InstanceStudent) error {
	if s == nil {
		return errInstanceStudentNil
	}
	if err := s.Validate(); err != nil {
		return err
	}
	return r.Repository.Update(ctx, s)
}

// FindByID overrides the base method to ensure schema-qualified queries.
func (r *InstanceStudentRepository) FindByID(ctx context.Context, id any) (*schedule.InstanceStudent, error) {
	var row schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
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

// List retrieves instance student rows matching the provided query options.
func (r *InstanceStudentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
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

// FindByInstanceID returns all attendance rows for an instance.
func (r *InstanceStudentRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Order(orderCreatedAtASC)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
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

// FindByStudentAndDateRange returns attendance rows for a student across all
// instances whose date falls within the inclusive range.
func (r *InstanceStudentRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to time.Time) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		Order(`"activity_instance".date ASC`, `"activity_instance".start_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date range",
			Err: err,
		}
	}
	return rows, nil
}

// FindByInstanceAndStudent returns a single attendance row, or nil if the
// student is not expected at the instance.
func (r *InstanceStudentRepository) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*schedule.InstanceStudent, error) {
	var row schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&row).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by instance and student",
			Err: err,
		}
	}
	return &row, nil
}

// DeleteByInstanceID removes all attendance rows for an instance.
func (r *InstanceStudentRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id = ?`, instanceID)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
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
