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

// Table names for pickup schedule repositories.
const (
	tablePickupSchedules  = "schedule.student_pickup_schedules"
	tablePickupExceptions = "schedule.student_pickup_exceptions"
)

// errScheduleNil is returned when a nil schedule is passed to a repository method.
var errScheduleNil = fmt.Errorf("schedule cannot be nil")

// StudentPickupScheduleRepository implements schedule.StudentPickupScheduleRepository interface
type StudentPickupScheduleRepository struct {
	*base.Repository[*schedule.StudentPickupSchedule]
	db *bun.DB
}

// NewStudentPickupScheduleRepository creates a new StudentPickupScheduleRepository
func NewStudentPickupScheduleRepository(db *bun.DB) schedule.StudentPickupScheduleRepository {
	return &StudentPickupScheduleRepository{
		Repository: base.NewRepository[*schedule.StudentPickupSchedule](db, tablePickupSchedules, "StudentPickupSchedule"),
		db:         db,
	}
}

// FindByStudentID finds all pickup schedules for a student
func (r *StudentPickupScheduleRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentPickupSchedule, error) {
	var schedules []*schedule.StudentPickupSchedule
	err := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).
		Where(`"student_pickup_schedule".student_id = ?`, studentID).
		Order("weekday ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id",
			Err: err,
		}
	}

	return schedules, nil
}

// FindByStudentIDAndWeekday finds a pickup schedule for a specific student and weekday
func (r *StudentPickupScheduleRepository) FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentPickupSchedule, error) {
	var pickupSchedule schedule.StudentPickupSchedule
	err := r.db.NewSelect().
		Model(&pickupSchedule).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).
		Where(`"student_pickup_schedule".student_id = ?`, studentID).
		Where(`"student_pickup_schedule".weekday = ?`, weekday).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and weekday",
			Err: err,
		}
	}

	return &pickupSchedule, nil
}

// FindByStudentIDsAndWeekday finds pickup schedules for multiple students and a specific weekday (bulk query)
func (r *StudentPickupScheduleRepository) FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedule.StudentPickupSchedule, error) {
	if len(studentIDs) == 0 {
		return []*schedule.StudentPickupSchedule{}, nil
	}

	var schedules []*schedule.StudentPickupSchedule
	err := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).
		Where(`"student_pickup_schedule".student_id IN (?)`, bun.In(studentIDs)).
		Where(`"student_pickup_schedule".weekday = ?`, weekday).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and weekday",
			Err: err,
		}
	}

	return schedules, nil
}

// UpsertSchedule creates or updates a pickup schedule for a student and weekday
func (r *StudentPickupScheduleRepository) UpsertSchedule(ctx context.Context, s *schedule.StudentPickupSchedule) error {
	if s == nil {
		return errScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(s).
		ModelTableExpr(tablePickupSchedules).
		On("CONFLICT (student_id, weekday) DO UPDATE").
		Set("pickup_time = EXCLUDED.pickup_time").
		Set("notes = EXCLUDED.notes").
		Set("updated_at = NOW()").
		Returning("id").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "upsert schedule",
			Err: err,
		}
	}

	return nil
}

// DeleteByStudentID deletes all pickup schedules for a student
func (r *StudentPickupScheduleRepository) DeleteByStudentID(ctx context.Context, studentID int64) error {
	_, err := r.db.NewDelete().
		Model((*schedule.StudentPickupSchedule)(nil)).
		ModelTableExpr(tablePickupSchedules).
		Where("student_id = ?", studentID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by student id",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *StudentPickupScheduleRepository) Create(ctx context.Context, s *schedule.StudentPickupSchedule) error {
	if s == nil {
		return errScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, s)
}

// Update overrides the base Update method to handle validation
func (r *StudentPickupScheduleRepository) Update(ctx context.Context, s *schedule.StudentPickupSchedule) error {
	if s == nil {
		return errScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, s)
}

// List retrieves pickup schedules matching the provided query options
func (r *StudentPickupScheduleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StudentPickupSchedule, error) {
	var schedules []*schedule.StudentPickupSchedule
	query := r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`)

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

	return schedules, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *StudentPickupScheduleRepository) FindByID(ctx context.Context, id any) (*schedule.StudentPickupSchedule, error) {
	var pickupSchedule schedule.StudentPickupSchedule

	err := r.db.NewSelect().
		Model(&pickupSchedule).
		ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).
		Where(`"student_pickup_schedule".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return &pickupSchedule, nil
}

// StudentPickupExceptionRepository implements schedule.StudentPickupExceptionRepository interface
type StudentPickupExceptionRepository struct {
	*base.Repository[*schedule.StudentPickupException]
	db *bun.DB
}

// NewStudentPickupExceptionRepository creates a new StudentPickupExceptionRepository
func NewStudentPickupExceptionRepository(db *bun.DB) schedule.StudentPickupExceptionRepository {
	return &StudentPickupExceptionRepository{
		Repository: base.NewRepository[*schedule.StudentPickupException](db, tablePickupExceptions, "StudentPickupException"),
		db:         db,
	}
}

// FindByStudentID finds all pickup exceptions for a student
func (r *StudentPickupExceptionRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error) {
	var exceptions []*schedule.StudentPickupException
	err := r.db.NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where(`"student_pickup_exception".student_id = ?`, studentID).
		Order("exception_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id",
			Err: err,
		}
	}

	return exceptions, nil
}

// FindUpcomingByStudentID finds upcoming pickup exceptions for a student (from today onwards)
func (r *StudentPickupExceptionRepository) FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error) {
	var exceptions []*schedule.StudentPickupException
	today := timezone.Today()

	err := r.db.NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where(`"student_pickup_exception".student_id = ?`, studentID).
		Where(`"student_pickup_exception".exception_date >= ?`, today).
		Order("exception_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find upcoming by student id",
			Err: err,
		}
	}

	return exceptions, nil
}

// FindByStudentIDAndDate finds a pickup exception for a specific student and date
func (r *StudentPickupExceptionRepository) FindByStudentIDAndDate(ctx context.Context, studentID int64, date time.Time) (*schedule.StudentPickupException, error) {
	var exception schedule.StudentPickupException
	dateOnly := timezone.DateOf(date)

	err := r.db.NewSelect().
		Model(&exception).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where(`"student_pickup_exception".student_id = ?`, studentID).
		Where(`"student_pickup_exception".exception_date = ?`, dateOnly).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and date",
			Err: err,
		}
	}

	return &exception, nil
}

// FindByStudentIDsAndDate finds pickup exceptions for multiple students and a specific date (bulk query)
func (r *StudentPickupExceptionRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date time.Time) ([]*schedule.StudentPickupException, error) {
	if len(studentIDs) == 0 {
		return []*schedule.StudentPickupException{}, nil
	}

	dateOnly := timezone.DateOf(date)
	var exceptions []*schedule.StudentPickupException

	err := r.db.NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where(`"student_pickup_exception".student_id IN (?)`, bun.In(studentIDs)).
		Where(`"student_pickup_exception".exception_date = ?`, dateOnly).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: err,
		}
	}

	return exceptions, nil
}

// DeleteByStudentID deletes all pickup exceptions for a student
func (r *StudentPickupExceptionRepository) DeleteByStudentID(ctx context.Context, studentID int64) error {
	_, err := r.db.NewDelete().
		Model((*schedule.StudentPickupException)(nil)).
		ModelTableExpr(tablePickupExceptions).
		Where("student_id = ?", studentID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by student id",
			Err: err,
		}
	}

	return nil
}

// DeletePastExceptions deletes all exceptions older than the given date
func (r *StudentPickupExceptionRepository) DeletePastExceptions(ctx context.Context, beforeDate time.Time) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*schedule.StudentPickupException)(nil)).
		ModelTableExpr(tablePickupExceptions).
		Where("exception_date < ?", beforeDate).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete past exceptions",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// Create overrides the base Create method to handle validation
func (r *StudentPickupExceptionRepository) Create(ctx context.Context, e *schedule.StudentPickupException) error {
	if e == nil {
		return fmt.Errorf("exception cannot be nil")
	}

	if err := e.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, e)
}

// Update overrides the base Update method to handle validation
func (r *StudentPickupExceptionRepository) Update(ctx context.Context, e *schedule.StudentPickupException) error {
	if e == nil {
		return fmt.Errorf("exception cannot be nil")
	}

	if err := e.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, e)
}

// List retrieves pickup exceptions matching the provided query options
func (r *StudentPickupExceptionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StudentPickupException, error) {
	var exceptions []*schedule.StudentPickupException
	query := r.db.NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`)

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

	return exceptions, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *StudentPickupExceptionRepository) FindByID(ctx context.Context, id any) (*schedule.StudentPickupException, error) {
	var exception schedule.StudentPickupException

	err := r.db.NewSelect().
		Model(&exception).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where(`"student_pickup_exception".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return &exception, nil
}
