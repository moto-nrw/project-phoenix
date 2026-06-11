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

// Table names for arrival schedule repositories.
const (
	tableArrivalSchedules  = "schedule.student_arrival_schedules"
	tableArrivalExceptions = "schedule.student_arrival_exceptions"
	tableArrivalNotes      = "schedule.student_arrival_notes"
)

// errArrivalScheduleNil is returned when a nil schedule is passed to a repository method.
var errArrivalScheduleNil = fmt.Errorf("arrival schedule cannot be nil")

// StudentArrivalScheduleRepository implements schedule.StudentArrivalScheduleRepository interface
type StudentArrivalScheduleRepository struct {
	*base.Repository[*schedule.StudentArrivalSchedule]
	db *bun.DB
}

// NewStudentArrivalScheduleRepository creates a new StudentArrivalScheduleRepository
func NewStudentArrivalScheduleRepository(db *bun.DB) schedule.StudentArrivalScheduleRepository {
	repo := base.NewRepository[*schedule.StudentArrivalSchedule](db, tableArrivalSchedules, "StudentArrivalSchedule")
	repo.TenantScoped = true
	return &StudentArrivalScheduleRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStudentID finds all arrival schedules for a student
func (r *StudentArrivalScheduleRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalSchedule, error) {
	var schedules []*schedule.StudentArrivalSchedule
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".student_id = ?`, studentID).
		Order("weekday ASC")

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByStudentID,
			Err: err,
		}
	}

	return schedules, nil
}

// FindByStudentIDAndWeekday finds an arrival schedule for a specific student and weekday
func (r *StudentArrivalScheduleRepository) FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentArrivalSchedule, error) {
	var arrivalSchedule schedule.StudentArrivalSchedule
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&arrivalSchedule).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".student_id = ?`, studentID).
		Where(`"student_arrival_schedule".weekday = ?`, weekday)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and weekday",
			Err: err,
		}
	}

	return &arrivalSchedule, nil
}

// FindByStudentIDsAndWeekday finds arrival schedules for multiple students and a specific weekday (bulk query)
func (r *StudentArrivalScheduleRepository) FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedule.StudentArrivalSchedule, error) {
	if len(studentIDs) == 0 {
		return []*schedule.StudentArrivalSchedule{}, nil
	}

	var schedules []*schedule.StudentArrivalSchedule
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_arrival_schedule".weekday = ?`, weekday)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and weekday",
			Err: err,
		}
	}

	return schedules, nil
}

// UpsertSchedule creates or updates an arrival schedule for a student and weekday
func (r *StudentArrivalScheduleRepository) UpsertSchedule(ctx context.Context, s *schedule.StudentArrivalSchedule) error {
	if s == nil {
		return errArrivalScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, s)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(s).
		ModelTableExpr(tableArrivalSchedules).
		On("CONFLICT (tenant_id, student_id, weekday) DO UPDATE").
		Set("expected_arrival = EXCLUDED.expected_arrival").
		Set("notes = EXCLUDED.notes").
		Set("updated_at = NOW()").
		Returning("id").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "upsert arrival schedule",
			Err: err,
		}
	}

	return nil
}

// DeleteByStudentID deletes all arrival schedules for a student
func (r *StudentArrivalScheduleRepository) DeleteByStudentID(ctx context.Context, studentID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.StudentArrivalSchedule)(nil)).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".student_id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  opDeleteByStudentID,
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *StudentArrivalScheduleRepository) Create(ctx context.Context, s *schedule.StudentArrivalSchedule) error {
	if s == nil {
		return errArrivalScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, s)
}

// Update overrides the base Update method to handle validation
func (r *StudentArrivalScheduleRepository) Update(ctx context.Context, s *schedule.StudentArrivalSchedule) error {
	if s == nil {
		return errArrivalScheduleNil
	}

	if err := s.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, s)
}

// List retrieves arrival schedules matching the provided query options
func (r *StudentArrivalScheduleRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StudentArrivalSchedule, error) {
	schedules := make([]*schedule.StudentArrivalSchedule, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
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

	return schedules, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *StudentArrivalScheduleRepository) FindByID(ctx context.Context, id any) (*schedule.StudentArrivalSchedule, error) {
	var arrivalSchedule schedule.StudentArrivalSchedule

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&arrivalSchedule).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_schedule"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}

	return &arrivalSchedule, nil
}

// StudentArrivalExceptionRepository implements schedule.StudentArrivalExceptionRepository interface
type StudentArrivalExceptionRepository struct {
	*base.Repository[*schedule.StudentArrivalException]
	db *bun.DB
}

// NewStudentArrivalExceptionRepository creates a new StudentArrivalExceptionRepository
func NewStudentArrivalExceptionRepository(db *bun.DB) schedule.StudentArrivalExceptionRepository {
	repo := base.NewRepository[*schedule.StudentArrivalException](db, tableArrivalExceptions, "StudentArrivalException")
	repo.TenantScoped = true
	return &StudentArrivalExceptionRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStudentID finds all arrival exceptions for a student
func (r *StudentArrivalExceptionRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error) {
	var exceptions []*schedule.StudentArrivalException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id = ?`, studentID).
		Order("exception_date ASC")

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByStudentID,
			Err: err,
		}
	}

	return exceptions, nil
}

// FindUpcomingByStudentID finds upcoming arrival exceptions for a student (from today onwards)
func (r *StudentArrivalExceptionRepository) FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error) {
	var exceptions []*schedule.StudentArrivalException
	today := timezone.TodayDate()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id = ?`, studentID).
		Where(`"student_arrival_exception".exception_date >= ?`, today).
		Order("exception_date ASC")

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find upcoming by student id",
			Err: err,
		}
	}

	return exceptions, nil
}

// FindByStudentIDAndDate finds an arrival exception for a specific student and date
func (r *StudentArrivalExceptionRepository) FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) (*schedule.StudentArrivalException, error) {
	var exception schedule.StudentArrivalException

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exception).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id = ?`, studentID).
		Where(`"student_arrival_exception".exception_date = ?`, date)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
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

// FindByStudentIDsAndDate finds arrival exceptions for multiple students and a specific date (bulk query)
func (r *StudentArrivalExceptionRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*schedule.StudentArrivalException, error) {
	if len(studentIDs) == 0 {
		return []*schedule.StudentArrivalException{}, nil
	}

	var exceptions []*schedule.StudentArrivalException

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_arrival_exception".exception_date = ?`, date)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: err,
		}
	}

	return exceptions, nil
}

// FindByStudentIDAndDateRange finds arrival exceptions for a student whose
// exception_date lies in the inclusive [from, to] range.
func (r *StudentArrivalExceptionRepository) FindByStudentIDAndDateRange(
	ctx context.Context, studentID int64, from, to timezone.Date,
) ([]*schedule.StudentArrivalException, error) {
	var exceptions []*schedule.StudentArrivalException
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id = ?`, studentID).
		Where(`"student_arrival_exception".exception_date >= ?`, from).
		Where(`"student_arrival_exception".exception_date <= ?`, to).
		Order("exception_date ASC")

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and date range",
			Err: err,
		}
	}
	return exceptions, nil
}

// DeleteByStudentID deletes all arrival exceptions for a student
func (r *StudentArrivalExceptionRepository) DeleteByStudentID(ctx context.Context, studentID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.StudentArrivalException)(nil)).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".student_id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  opDeleteByStudentID,
			Err: err,
		}
	}

	return nil
}

// DeletePastExceptions deletes all exceptions older than the given date
func (r *StudentArrivalExceptionRepository) DeletePastExceptions(ctx context.Context, beforeDate timezone.Date) (int64, error) {
	delQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.StudentArrivalException)(nil)).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".exception_date < ?`, beforeDate)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		delQuery = delQuery.Where(where, val)
	}

	result, err := delQuery.Exec(ctx)
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
func (r *StudentArrivalExceptionRepository) Create(ctx context.Context, e *schedule.StudentArrivalException) error {
	if e == nil {
		return fmt.Errorf("exception cannot be nil")
	}

	if err := e.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, e)
}

// Update overrides the base Update method to handle validation
func (r *StudentArrivalExceptionRepository) Update(ctx context.Context, e *schedule.StudentArrivalException) error {
	if e == nil {
		return fmt.Errorf("exception cannot be nil")
	}

	if err := e.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, e)
}

// List retrieves arrival exceptions matching the provided query options
func (r *StudentArrivalExceptionRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StudentArrivalException, error) {
	exceptions := make([]*schedule.StudentArrivalException, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
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

	return exceptions, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *StudentArrivalExceptionRepository) FindByID(ctx context.Context, id any) (*schedule.StudentArrivalException, error) {
	var exception schedule.StudentArrivalException

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exception).
		ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).
		Where(`"student_arrival_exception".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_exception"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}

	return &exception, nil
}

// StudentArrivalNoteRepository implements schedule.StudentArrivalNoteRepository interface
type StudentArrivalNoteRepository struct {
	*base.Repository[*schedule.StudentArrivalNote]
	db *bun.DB
}

// NewStudentArrivalNoteRepository creates a new StudentArrivalNoteRepository
func NewStudentArrivalNoteRepository(db *bun.DB) schedule.StudentArrivalNoteRepository {
	repo := base.NewRepository[*schedule.StudentArrivalNote](db, tableArrivalNotes, "StudentArrivalNote")
	repo.TenantScoped = true
	return &StudentArrivalNoteRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStudentID finds all arrival notes for a student
func (r *StudentArrivalNoteRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalNote, error) {
	var notes []*schedule.StudentArrivalNote
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".student_id = ?`, studentID).
		Order("note_date ASC", orderCreatedAtASC)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByStudentID,
			Err: err,
		}
	}

	return notes, nil
}

// FindByStudentIDAndDate finds all arrival notes for a student on a specific date
func (r *StudentArrivalNoteRepository) FindByStudentIDAndDate(ctx context.Context, studentID int64, date timezone.Date) ([]*schedule.StudentArrivalNote, error) {
	var notes []*schedule.StudentArrivalNote

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".student_id = ?`, studentID).
		Where(`"student_arrival_note".note_date = ?`, date).
		Order(orderCreatedAtASC)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and date",
			Err: err,
		}
	}

	return notes, nil
}

// FindByStudentIDsAndDate finds all arrival notes for multiple students on a specific date (bulk query)
func (r *StudentArrivalNoteRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*schedule.StudentArrivalNote, error) {
	if len(studentIDs) == 0 {
		return []*schedule.StudentArrivalNote{}, nil
	}

	var notes []*schedule.StudentArrivalNote

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"student_arrival_note".note_date = ?`, date).
		Order(orderCreatedAtASC)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: err,
		}
	}

	return notes, nil
}

// DeleteByStudentID deletes all arrival notes for a student
func (r *StudentArrivalNoteRepository) DeleteByStudentID(ctx context.Context, studentID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.StudentArrivalNote)(nil)).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".student_id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  opDeleteByStudentID,
			Err: err,
		}
	}

	return nil
}

// DeletePastNotes deletes all notes older than the given date
func (r *StudentArrivalNoteRepository) DeletePastNotes(ctx context.Context, beforeDate timezone.Date) (int64, error) {
	delQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.StudentArrivalNote)(nil)).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".note_date < ?`, beforeDate)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		delQuery = delQuery.Where(where, val)
	}

	result, err := delQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete past notes",
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
func (r *StudentArrivalNoteRepository) Create(ctx context.Context, n *schedule.StudentArrivalNote) error {
	if n == nil {
		return fmt.Errorf("note cannot be nil")
	}

	if err := n.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, n)
}

// Update overrides the base Update method to handle validation
func (r *StudentArrivalNoteRepository) Update(ctx context.Context, n *schedule.StudentArrivalNote) error {
	if n == nil {
		return fmt.Errorf("note cannot be nil")
	}

	if err := n.Validate(); err != nil {
		return err
	}

	return r.Repository.Update(ctx, n)
}

// List retrieves arrival notes matching the provided query options
func (r *StudentArrivalNoteRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.StudentArrivalNote, error) {
	notes := make([]*schedule.StudentArrivalNote, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
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

	return notes, nil
}

// FindByID overrides base method to ensure schema qualification
func (r *StudentArrivalNoteRepository) FindByID(ctx context.Context, id any) (*schedule.StudentArrivalNote, error) {
	var note schedule.StudentArrivalNote

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&note).
		ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).
		Where(`"student_arrival_note".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "student_arrival_note"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  opFindByID,
			Err: err,
		}
	}

	return &note, nil
}
