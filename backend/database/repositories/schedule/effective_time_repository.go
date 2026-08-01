package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	opFindByStudentID   = "find by student id"
	opDeleteByStudentID = "delete by student id"
	opFindByID          = "find by id"
	orderCreatedAtASC   = "created_at ASC"
)

type validatedEntity interface {
	modelBase.Entity
	modelBase.Validator
}

type effectiveTimeRepositoryConfig struct {
	table      string
	alias      string
	entityName string
}

func newTenantRepository[T modelBase.Entity](
	db *bun.DB,
	config effectiveTimeRepositoryConfig,
) *base.Repository[T] {
	repo := base.NewRepository[T](db, config.table, config.entityName)
	repo.TenantScoped = true
	return repo
}

func newRepositoryEntity[T modelBase.Entity]() T {
	entityType := reflect.TypeFor[T]()
	if entityType.Kind() != reflect.Pointer {
		panic("effective-time repositories require pointer entities")
	}
	return reflect.New(entityType.Elem()).Interface().(T)
}

type studentScheduleRepository[T validatedEntity] struct {
	*base.Repository[T]
	db         *bun.DB
	config     effectiveTimeRepositoryConfig
	timeColumn string
	upsertOp   string
	nilErr     error
}

func newStudentScheduleRepository[T validatedEntity](
	db *bun.DB,
	config effectiveTimeRepositoryConfig,
	timeColumn string,
	upsertOp string,
	nilErr error,
) *studentScheduleRepository[T] {
	return &studentScheduleRepository[T]{
		Repository: newTenantRepository[T](db, config),
		db:         db,
		config:     config,
		timeColumn: timeColumn,
		upsertOp:   upsertOp,
		nilErr:     nilErr,
	}
}

func (r *studentScheduleRepository[T]) FindByStudentID(
	ctx context.Context,
	studentID int64,
) ([]T, error) {
	var schedules []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Order("weekday ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: opFindByStudentID, Err: err}
	}
	return schedules, nil
}

func (r *studentScheduleRepository[T]) FindByStudentIDAndWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (T, error) {
	schedule := newRepositoryEntity[T]()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(schedule).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Where(fmt.Sprintf(`"%s".weekday = ?`, r.config.alias), weekday)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var zero T
			return zero, nil
		}
		var zero T
		return zero, &modelBase.DatabaseError{
			Op:  "find by student id and weekday",
			Err: err,
		}
	}
	return schedule, nil
}

func (r *studentScheduleRepository[T]) FindByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]T, error) {
	if len(studentIDs) == 0 {
		return []T{}, nil
	}

	var schedules []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id IN (?)`, r.config.alias), bun.List(studentIDs)).
		Where(fmt.Sprintf(`"%s".weekday = ?`, r.config.alias), weekday)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and weekday",
			Err: err,
		}
	}
	return schedules, nil
}

func (r *studentScheduleRepository[T]) FindByStudentIDs(
	ctx context.Context,
	studentIDs []int64,
) ([]T, error) {
	if len(studentIDs) == 0 {
		return []T{}, nil
	}

	var schedules []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&schedules).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id IN (?)`, r.config.alias), bun.List(studentIDs)).
		Order("student_id ASC", "weekday ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids",
			Err: err,
		}
	}
	return schedules, nil
}

func (r *studentScheduleRepository[T]) UpsertSchedule(ctx context.Context, schedule T) error {
	if reflect.ValueOf(schedule).IsZero() {
		return r.nilErr
	}
	if err := schedule.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, schedule)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(schedule).
		ModelTableExpr(r.config.table).
		On("CONFLICT (tenant_id, student_id, weekday) DO UPDATE").
		Set(fmt.Sprintf("%s = EXCLUDED.%s", r.timeColumn, r.timeColumn)).
		Set("notes = EXCLUDED.notes").
		Set("updated_at = NOW()").
		Returning("id").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: r.upsertOp, Err: err}
	}
	return nil
}

func (r *studentScheduleRepository[T]) DeleteByStudentID(
	ctx context.Context,
	studentID int64,
) error {
	var model T
	query := base.GetDB(ctx, r.db).NewDelete().
		Model(model).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: opDeleteByStudentID, Err: err}
	}
	return nil
}

func (r *studentScheduleRepository[T]) List(
	ctx context.Context,
	options *modelBase.QueryOptions,
) ([]T, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []T{}
	}
	return rows, nil
}

type studentExceptionRepository[T validatedEntity] struct {
	*base.Repository[T]
	db     *bun.DB
	config effectiveTimeRepositoryConfig
}

func newStudentExceptionRepository[T validatedEntity](
	db *bun.DB,
	config effectiveTimeRepositoryConfig,
) *studentExceptionRepository[T] {
	return &studentExceptionRepository[T]{
		Repository: newTenantRepository[T](db, config),
		db:         db,
		config:     config,
	}
}

func (r *studentExceptionRepository[T]) FindByStudentID(
	ctx context.Context,
	studentID int64,
) ([]T, error) {
	var exceptions []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Order("exception_date ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: opFindByStudentID, Err: err}
	}
	return exceptions, nil
}

func (r *studentExceptionRepository[T]) FindUpcomingByStudentID(
	ctx context.Context,
	studentID int64,
) ([]T, error) {
	var exceptions []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Where(fmt.Sprintf(`"%s".exception_date >= ?`, r.config.alias), timezone.TodayDate()).
		Order("exception_date ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find upcoming by student id",
			Err: err,
		}
	}
	return exceptions, nil
}

func (r *studentExceptionRepository[T]) FindByStudentIDAndDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (T, error) {
	exception := newRepositoryEntity[T]()
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(exception).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Where(fmt.Sprintf(`"%s".exception_date = ?`, r.config.alias), date)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var zero T
			return zero, nil
		}
		var zero T
		return zero, &modelBase.DatabaseError{
			Op:  "find by student id and date",
			Err: err,
		}
	}
	return exception, nil
}

func (r *studentExceptionRepository[T]) FindByStudentIDsAndDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) ([]T, error) {
	if len(studentIDs) == 0 {
		return []T{}, nil
	}

	var exceptions []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id IN (?)`, r.config.alias), bun.List(studentIDs)).
		Where(fmt.Sprintf(`"%s".exception_date = ?`, r.config.alias), date)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: err,
		}
	}
	return exceptions, nil
}

func (r *studentExceptionRepository[T]) FindByStudentIDAndDateRange(
	ctx context.Context,
	studentID int64,
	from timezone.Date,
	to timezone.Date,
) ([]T, error) {
	var exceptions []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Where(fmt.Sprintf(`"%s".exception_date >= ?`, r.config.alias), from).
		Where(fmt.Sprintf(`"%s".exception_date <= ?`, r.config.alias), to).
		Order("exception_date ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and date range",
			Err: err,
		}
	}
	return exceptions, nil
}

func (r *studentExceptionRepository[T]) FindByStudentIDsAndDateRange(
	ctx context.Context,
	studentIDs []int64,
	from timezone.Date,
	to timezone.Date,
) ([]T, error) {
	if len(studentIDs) == 0 {
		return []T{}, nil
	}

	var exceptions []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&exceptions).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id IN (?)`, r.config.alias), bun.List(studentIDs)).
		Where(fmt.Sprintf(`"%s".exception_date >= ?`, r.config.alias), from).
		Where(fmt.Sprintf(`"%s".exception_date <= ?`, r.config.alias), to).
		Order("exception_date ASC")
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date range",
			Err: err,
		}
	}
	return exceptions, nil
}

func (r *studentExceptionRepository[T]) DeleteByStudentID(
	ctx context.Context,
	studentID int64,
) error {
	var model T
	query := base.GetDB(ctx, r.db).NewDelete().
		Model(model).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: opDeleteByStudentID, Err: err}
	}
	return nil
}

func (r *studentExceptionRepository[T]) DeletePastExceptions(
	ctx context.Context,
	beforeDate timezone.Date,
) (int64, error) {
	var model T
	query := base.GetDB(ctx, r.db).NewDelete().
		Model(model).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".exception_date < ?`, r.config.alias), beforeDate)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete past exceptions", Err: err}
	}
	return result.RowsAffected()
}

func (r *studentExceptionRepository[T]) List(
	ctx context.Context,
	options *modelBase.QueryOptions,
) ([]T, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []T{}
	}
	return rows, nil
}

type studentNoteRepository[T validatedEntity] struct {
	*base.Repository[T]
	db     *bun.DB
	config effectiveTimeRepositoryConfig
}

func newStudentNoteRepository[T validatedEntity](
	db *bun.DB,
	config effectiveTimeRepositoryConfig,
) *studentNoteRepository[T] {
	return &studentNoteRepository[T]{
		Repository: newTenantRepository[T](db, config),
		db:         db,
		config:     config,
	}
}

func (r *studentNoteRepository[T]) FindByStudentID(
	ctx context.Context,
	studentID int64,
) ([]T, error) {
	var notes []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Order("note_date ASC", orderCreatedAtASC)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: opFindByStudentID, Err: err}
	}
	return notes, nil
}

func (r *studentNoteRepository[T]) FindByStudentIDAndDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) ([]T, error) {
	var notes []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID).
		Where(fmt.Sprintf(`"%s".note_date = ?`, r.config.alias), date).
		Order(orderCreatedAtASC)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student id and date",
			Err: err,
		}
	}
	return notes, nil
}

func (r *studentNoteRepository[T]) FindByStudentIDsAndDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) ([]T, error) {
	if len(studentIDs) == 0 {
		return []T{}, nil
	}

	var notes []T
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&notes).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id IN (?)`, r.config.alias), bun.List(studentIDs)).
		Where(fmt.Sprintf(`"%s".note_date = ?`, r.config.alias), date).
		Order(orderCreatedAtASC)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ids and date",
			Err: err,
		}
	}
	return notes, nil
}

func (r *studentNoteRepository[T]) DeleteByStudentID(
	ctx context.Context,
	studentID int64,
) error {
	var model T
	query := base.GetDB(ctx, r.db).NewDelete().
		Model(model).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".student_id = ?`, r.config.alias), studentID)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: opDeleteByStudentID, Err: err}
	}
	return nil
}

func (r *studentNoteRepository[T]) DeletePastNotes(
	ctx context.Context,
	beforeDate timezone.Date,
) (int64, error) {
	var model T
	query := base.GetDB(ctx, r.db).NewDelete().
		Model(model).
		ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.config.table, r.config.alias)).
		Where(fmt.Sprintf(`"%s".note_date < ?`, r.config.alias), beforeDate)
	query = base.WithTenantFilter(ctx, query, r.config.alias)

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete past notes", Err: err}
	}
	return result.RowsAffected()
}

func (r *studentNoteRepository[T]) List(
	ctx context.Context,
	options *modelBase.QueryOptions,
) ([]T, error) {
	rows, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []T{}
	}
	return rows, nil
}
