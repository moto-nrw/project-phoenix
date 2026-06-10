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

// FindExpectedByInstanceIDs returns every instance_students row with
// status='expected' for any of the given instance IDs. Tenant-scoped.
// Empty input returns an empty slice without hitting the DB, matching the
// sibling bulk helpers (see CountNonAbsentByInstanceIDs in instance_staff_repo).
func (r *InstanceStudentRepository) FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*schedule.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*schedule.InstanceStudent{}, nil
	}
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".instance_id IN (?)`, bun.List(instanceIDs)).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusExpected).
		OrderExpr(`"instance_student".instance_id ASC, "instance_student".student_id ASC`)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find expected by instance ids",
			Err: err,
		}
	}
	return rows, nil
}

// FindByStudentAndDateRange returns attendance rows for a student across all
// instances whose date falls within the inclusive range.
func (r *InstanceStudentRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*schedule.InstanceStudent, error) {
	var rows []*schedule.InstanceStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(modelTblInstanceStudent).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"activity_instance".date >= ?`, from).
		Where(`"activity_instance".date <= ?`, to).
		OrderExpr(`"activity_instance".date ASC, "activity_instance".start_time ASC`)

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

// UpdateAttendanceFromCheckin is the mirror-path write (WP-B10). It flips
// status 'expected' → 'present' and stamps checked_in_at, but ONLY when the
// current row is still 'expected'. The status='expected' predicate in the
// WHERE clause enforces monotonicity: a row that has been marked present
// (by a prior check-in or a staff PATCH) is never overwritten here.
//
// Returns (updated=true) when exactly one row was modified. A zero-rows
// result means either (a) the row doesn't exist in this tenant, or
// (b) it already moved out of 'expected' between the caller's read and
// this UPDATE — a benign race the mirror service handles by preserving
// the existing state.
func (r *InstanceStudentRepository) UpdateAttendanceFromCheckin(
	ctx context.Context, instanceID, studentID int64, checkedInAt time.Time,
) (bool, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, schedule.AttendanceStatusPresent).
		Set(`checked_in_at = ?`, checkedInAt).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".status = ?`, schedule.AttendanceStatusExpected)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		q = q.Where(where, val)
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "update attendance from checkin",
			Err: err,
		}
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateAttendanceFields writes only the fields the patch carries. Pointer-nil
// (and the *Clear bools unset) means "do not touch that column". Substatus and
// Note support explicit NULL via the *Clear companion bools, since both
// columns are nullable in the schema.
//
// This is the PATCH-endpoint write — it overwrites whatever is there. The
// handler is responsible for cross-field validation (e.g. substatus-with-
// expected-status) before calling.
func (r *InstanceStudentRepository) UpdateAttendanceFields(
	ctx context.Context, id int64, patch schedule.AttendanceFieldPatch,
) error {
	if !patch.HasChanges() {
		// Defensive: the handler should reject this at 400. A repo no-op
		// here is safer than issuing an empty UPDATE and bumping updated_at.
		return nil
	}

	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Where(`"instance_student".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		q = q.Where(where, val)
	}

	if patch.Status != nil {
		q = q.Set(`status = ?`, *patch.Status)
	}
	switch {
	case patch.SubstatusClear:
		q = q.Set(`substatus = NULL`)
	case patch.Substatus != nil:
		q = q.Set(`substatus = ?`, *patch.Substatus)
	}
	switch {
	case patch.NoteClear:
		q = q.Set(`note = NULL`)
	case patch.Note != nil:
		q = q.Set(`note = ?`, *patch.Note)
	}
	q = q.Set(`updated_at = ?`, time.Now().UTC())

	if _, err := q.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update attendance fields",
			Err: err,
		}
	}
	return nil
}

// BulkUpdateStatus flips every attendance row for instanceID whose current
// status equals fromStatus to toStatus. Returns the number of rows changed.
//
// Used by instance Complete() to mark remaining expected students as absent.
// The fromStatus predicate keeps the update idempotent and free of clobber:
// rows already moved past 'expected' (present via checkin, absent via PATCH)
// stay put.
func (r *InstanceStudentRepository) BulkUpdateStatus(
	ctx context.Context, instanceID int64, fromStatus, toStatus string,
) (int, error) {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*schedule.InstanceStudent)(nil)).
		ModelTableExpr(modelTblInstanceStudent).
		Set(`status = ?`, toStatus).
		Set(`updated_at = ?`, time.Now().UTC()).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Where(`"instance_student".status = ?`, fromStatus)

	if where, val, ok := base.TenantWhere(ctx, aliasInstanceStudent); ok {
		q = q.Where(where, val)
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "bulk update status",
			Err: err,
		}
	}
	n, _ := res.RowsAffected()
	return int(n), nil
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
