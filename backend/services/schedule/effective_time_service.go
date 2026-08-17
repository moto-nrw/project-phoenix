package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type effectiveTimeEntity interface {
	modelBase.Entity
	modelBase.Validator
	modelBase.TenantScoped
}

type effectiveScheduleRepository[S effectiveTimeEntity] interface {
	FindByStudentID(context.Context, int64) ([]S, error)
	FindByStudentIDAndWeekday(context.Context, int64, int) (S, error)
	FindByStudentIDsAndWeekday(context.Context, []int64, int) ([]S, error)
	FindByStudentIDs(context.Context, []int64) ([]S, error)
	UpsertSchedule(context.Context, S) error
	Create(context.Context, S) error
	Delete(context.Context, any) error
	DeleteByStudentID(context.Context, int64) error
}

type effectiveExceptionRepository[E effectiveTimeEntity] interface {
	FindByID(context.Context, any) (E, error)
	FindByIDForUpdate(context.Context, any) (E, error)
	FindByStudentID(context.Context, int64) ([]E, error)
	FindUpcomingByStudentID(context.Context, int64) ([]E, error)
	FindByStudentIDAndDate(context.Context, int64, timezone.Date) (E, error)
	FindByStudentIDsAndDate(context.Context, []int64, timezone.Date) ([]E, error)
	Create(context.Context, E) error
	Update(context.Context, E) error
	Delete(context.Context, any) error
	DeleteByStudentID(context.Context, int64) error
}

type effectiveNoteRepository[N effectiveTimeEntity] interface {
	FindByID(context.Context, any) (N, error)
	FindByStudentID(context.Context, int64) ([]N, error)
	FindByStudentIDAndDate(context.Context, int64, timezone.Date) ([]N, error)
	FindByStudentIDsAndDate(context.Context, []int64, timezone.Date) ([]N, error)
	Create(context.Context, N) error
	Update(context.Context, N) error
	Delete(context.Context, any) error
	DeleteByStudentID(context.Context, int64) error
}

type effectiveScheduleFields struct {
	StudentID int64
	Weekday   int
	Time      time.Time
	Notes     *string
}

type effectiveExceptionFields struct {
	ID                int64
	StudentID         int64
	Date              timezone.Date
	Time              *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
	CreatedAt         time.Time
	TenantID          int64
	ExcusedFrom       *time.Time
	ExcusedReason     *string
	ExcusedCreatedBy  *int64
	ExcusedOwnsTime   bool
	ExcusedAuto       bool
	TimeChanged       bool
}

type effectiveNoteFields struct {
	ID        int64
	StudentID int64
	Content   string
}

type effectiveTimeDomain[S effectiveTimeEntity, E effectiveTimeEntity, N effectiveTimeEntity] interface {
	Name() string
	CollisionPolicy() exceptionCollisionPolicy
	ScheduleFields(S) effectiveScheduleFields
	SetScheduleStudentID(S, int64)
	ExceptionFields(E) effectiveExceptionFields
	NewException(effectiveExceptionFields) E
	AssignException(E, E)
	NoteFields(N) effectiveNoteFields
}

type exceptionCollisionPolicy uint8

const (
	exceptionCollisionReject exceptionCollisionPolicy = iota
	exceptionCollisionUpdate
)

var ErrCareExceptionContainsPartialAbsence = errors.New("pickup exception contains a partial absence")

type effectiveTimeData[S, E, N any] struct {
	Schedules  []S
	Exceptions []E
	Notes      []N
}

type effectiveDayNote struct {
	ID      int64
	Content string
}

type effectiveTimeResult struct {
	Date        timezone.Date
	Time        *time.Time
	WeekdayName string
	IsException bool
	Notes       string
	DayNotes    []effectiveDayNote
}

type effectiveTimeCore[
	S effectiveTimeEntity,
	E effectiveTimeEntity,
	N effectiveTimeEntity,
	D effectiveTimeDomain[S, E, N],
] struct {
	schedules  effectiveScheduleRepository[S]
	exceptions effectiveExceptionRepository[E]
	notes      effectiveNoteRepository[N]
	db         *bun.DB
	domain     D
}

func newEffectiveTimeCore[
	S effectiveTimeEntity,
	E effectiveTimeEntity,
	N effectiveTimeEntity,
	D effectiveTimeDomain[S, E, N],
](
	schedules effectiveScheduleRepository[S],
	exceptions effectiveExceptionRepository[E],
	notes effectiveNoteRepository[N],
	db *bun.DB,
	domain D,
) *effectiveTimeCore[S, E, N, D] {
	return &effectiveTimeCore[S, E, N, D]{
		schedules:  schedules,
		exceptions: exceptions,
		notes:      notes,
		db:         db,
		domain:     domain,
	}
}

func (c *effectiveTimeCore[S, E, N, D]) operation(action string) string {
	return fmt.Sprintf(action, c.domain.Name())
}

func (c *effectiveTimeCore[S, E, N, D]) Schedules(
	ctx context.Context,
	studentID int64,
) ([]S, error) {
	rows, err := c.schedules.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{
			Op:  c.operation("get student %s schedules"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) ScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (S, error) {
	if weekday < scheduleModel.WeekdayMonday || weekday > scheduleModel.WeekdayFriday {
		var zero S
		return zero, &ScheduleError{
			Op:  c.operation("get student %s schedule for weekday"),
			Err: errors.New("invalid weekday"),
		}
	}

	row, err := c.schedules.FindByStudentIDAndWeekday(ctx, studentID, weekday)
	if err != nil {
		var zero S
		return zero, &ScheduleError{
			Op:  c.operation("get student %s schedule for weekday"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpsertSchedule(
	ctx context.Context,
	row S,
) error {
	if err := row.Validate(); err != nil {
		return &ScheduleError{
			Op:  c.operation("upsert student %s schedule"),
			Err: err,
		}
	}
	if err := c.schedules.UpsertSchedule(ctx, row); err != nil {
		return &ScheduleError{
			Op:  c.operation("upsert student %s schedule"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpsertBulkSchedules(
	ctx context.Context,
	studentID int64,
	rows []S,
) error {
	op := c.operation("upsert bulk student %s schedules")
	if err := c.schedules.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{
			Op:  op,
			Err: fmt.Errorf("failed to delete existing schedules: %w", err),
		}
	}

	for _, row := range rows {
		c.domain.SetScheduleStudentID(row, studentID)
		fields := c.domain.ScheduleFields(row)
		if err := row.Validate(); err != nil {
			return &ScheduleError{
				Op:  op,
				Err: fmt.Errorf("invalid schedule for weekday %d: %w", fields.Weekday, err),
			}
		}
		row.SetTenantID(tenant.FromContext(ctx))
		if err := c.schedules.Create(ctx, row); err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	if err := c.schedules.Delete(ctx, scheduleID); err != nil {
		return &ScheduleError{
			Op:  c.operation("delete student %s schedule"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteAllSchedules(
	ctx context.Context,
	studentID int64,
) error {
	if err := c.schedules.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{
			Op:  c.operation("delete all student %s schedules"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) ExceptionByID(
	ctx context.Context,
	exceptionID int64,
) (E, error) {
	row, err := c.exceptions.FindByID(ctx, exceptionID)
	if err != nil {
		var zero E
		return zero, &ScheduleError{
			Op:  c.operation("get student %s exception by id"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) ExceptionForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (E, error) {
	row, err := c.exceptions.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		var zero E
		return zero, &ScheduleError{
			Op:  c.operation("get student %s exception for date"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) Exceptions(
	ctx context.Context,
	studentID int64,
) ([]E, error) {
	rows, err := c.exceptions.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{
			Op:  c.operation("get student %s exceptions"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpcomingExceptions(
	ctx context.Context,
	studentID int64,
) ([]E, error) {
	rows, err := c.exceptions.FindUpcomingByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{
			Op:  c.operation("get upcoming student %s exceptions"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) CreateException(
	ctx context.Context,
	row E,
) error {
	op := c.operation("create student %s exception")
	if err := row.Validate(); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}

	fields := c.domain.ExceptionFields(row)
	existing, err := c.exceptions.FindByStudentIDAndDate(ctx, fields.StudentID, fields.Date)
	if err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	if !isZeroEntity(existing) {
		if c.domain.CollisionPolicy() == exceptionCollisionReject {
			return &ScheduleError{
				Op:  op,
				Err: errors.New("exception already exists for this date"),
			}
		}

		existingFields := c.domain.ExceptionFields(existing)
		// Detect wall-clock changes so partial-absence ownership is dropped when
		// an upsert overwrites the pickup time. Leaving TimeChanged false keeps
		// ExcusedOwnsPickupTime and a later partial delete can wipe the explicit
		// pickup override along with the excusal metadata.
		switch {
		case fields.Time != nil:
			normalized := timezone.WallClock(*fields.Time)
			if existingFields.Time == nil || !timezone.SameClockTime(*existingFields.Time, normalized) {
				existingFields.TimeChanged = true
			}
			existingFields.Time = &normalized
		case existingFields.Time != nil:
			existingFields.Time = nil
			existingFields.TimeChanged = true
		}
		existingFields.Reason = fields.Reason
		updated := c.domain.NewException(existingFields)
		if err := c.exceptions.Update(ctx, updated); err != nil {
			return &ScheduleError{Op: op, Err: err}
		}
		c.domain.AssignException(row, updated)
		return nil
	}

	row.SetTenantID(tenant.FromContext(ctx))
	if err := c.exceptions.Create(ctx, row); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpdateExceptionRow(
	ctx context.Context,
	row E,
) error {
	op := c.operation("update student %s exception")
	if err := row.Validate(); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}

	fields := c.domain.ExceptionFields(row)
	existing, err := c.exceptions.FindByStudentIDAndDate(ctx, fields.StudentID, fields.Date)
	if err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	if !isZeroEntity(existing) && c.domain.ExceptionFields(existing).ID != fields.ID {
		return &ScheduleError{
			Op:  op,
			Err: errors.New("exception already exists for this date"),
		}
	}

	if err := c.exceptions.Update(ctx, row); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) CreateOrReclaimException(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
	value *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (E, error) {
	var result E
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, c.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := LockCareExceptionDay(txCtx, c.db, studentID, date); err != nil {
			return err
		}

		existing, err := c.ExceptionForDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if !isZeroEntity(existing) {
			existingFields := c.domain.ExceptionFields(existing)
			if existingFields.Source != scheduleModel.ExceptionSourceGuardian {
				return ErrCareExceptionDayConflict
			}
			// A partial absence may deliberately share a guardian-authored pickup
			// exception. Reclaiming that row would erase the partial's metadata
			// while its owned attendance rows remained absent.
			if existingFields.ExcusedFrom != nil {
				return ErrCareExceptionContainsPartialAbsence
			}
			resolvedStaffID, staffErr := resolveStaffID()
			if staffErr != nil || resolvedStaffID == 0 {
				return ErrCareExceptionStaffProfileRequired
			}
			result = c.domain.NewException(effectiveExceptionFields{
				ID:        existingFields.ID,
				StudentID: studentID,
				Date:      date,
				Time:      value,
				Reason:    reason,
				Source:    scheduleModel.ExceptionSourceStaff,
				CreatedBy: resolvedStaffID,
				CreatedAt: existingFields.CreatedAt,
				TenantID:  existingFields.TenantID,
			})
			return c.UpdateExceptionRow(txCtx, result)
		}

		result = c.domain.NewException(effectiveExceptionFields{
			StudentID: studentID,
			Date:      date,
			Time:      value,
			Reason:    reason,
			Source:    scheduleModel.ExceptionSourceStaff,
			CreatedBy: staffID,
		})
		return c.CreateException(txCtx, result)
	})
	if err != nil {
		var zero E
		return zero, err
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpdateException(
	ctx context.Context,
	exceptionID int64,
	studentID int64,
	date timezone.Date,
	reason *string,
	value *time.Time,
	clearValue bool,
	resolveStaffID func() (int64, error),
) (E, error) {
	var result E
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, c.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := LockCareExceptionDay(txCtx, c.db, studentID, date); err != nil {
			return err
		}

		fresh, err := c.ExceptionByID(txCtx, exceptionID)
		if err != nil {
			return err
		}
		if isZeroEntity(fresh) {
			return ErrCareExceptionNotFound
		}

		fields := c.domain.ExceptionFields(fresh)
		if fields.StudentID != studentID {
			return ErrCareExceptionWrongStudent
		}
		if fields.ExcusedFrom != nil && (date != fields.Date || clearValue) {
			return ErrCareExceptionContainsPartialAbsence
		}
		if fields.Source == scheduleModel.ExceptionSourceGuardian {
			resolvedStaffID, staffErr := resolveStaffID()
			if staffErr != nil || resolvedStaffID == 0 {
				return ErrCareExceptionStaffProfileRequired
			}
			fields.Source = scheduleModel.ExceptionSourceStaff
			fields.CreatedBy = resolvedStaffID
			fields.CreatedByGuardian = nil
		}

		fields.StudentID = studentID
		fields.Date = date
		if fields.Time != nil {
			normalized := timezone.WallClock(*fields.Time)
			fields.Time = &normalized
		}
		if fields.ExcusedFrom != nil {
			normalized := timezone.WallClock(*fields.ExcusedFrom)
			fields.ExcusedFrom = &normalized
		}
		if reason != nil {
			// A supplied empty string explicitly clears the reason. Keep nil as
			// the patch sentinel for clients that did not intend to touch it.
			if *reason == "" {
				fields.Reason = nil
			} else {
				fields.Reason = reason
			}
		}
		if value != nil {
			// Normalize before comparing so driver-specific date anchors do not
			// look like a real time edit. An identical wall-clock must leave
			// ExcusedOwnsPickupTime intact: otherwise deleting the partial
			// clears only excusal metadata and leaves a stale pickup override.
			normalized := timezone.WallClock(*value)
			if fields.Time == nil || !timezone.SameClockTime(*fields.Time, normalized) {
				fields.TimeChanged = true
			}
			fields.Time = &normalized
		} else if clearValue {
			fields.Time = nil
			fields.TimeChanged = true
		}

		result = c.domain.NewException(fields)
		return c.UpdateExceptionRow(txCtx, result)
	})
	if err != nil {
		var zero E
		return zero, err
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteException(
	ctx context.Context,
	exceptionID int64,
) error {
	initial, err := c.exceptions.FindByID(ctx, exceptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCareExceptionNotFound
		}
		return &ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
	}
	if isZeroEntity(initial) {
		return c.deleteExceptionRow(ctx, exceptionID)
	}

	fields := c.domain.ExceptionFields(initial)
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, c.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := LockCareExceptionDay(txCtx, c.db, fields.StudentID, fields.Date); err != nil {
			return err
		}
		// Re-read after taking the same lock used by partial-absence writes.
		// Otherwise a partial could be attached between the check and delete.
		fresh, err := c.exceptions.FindByIDForUpdate(txCtx, exceptionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrCareExceptionNotFound
			}
			return &ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
		}
		if !isZeroEntity(fresh) && c.domain.ExceptionFields(fresh).ExcusedFrom != nil {
			return ErrCareExceptionContainsPartialAbsence
		}
		return c.deleteExceptionRow(txCtx, exceptionID)
	})
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteAllExceptions(
	ctx context.Context,
	studentID int64,
) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, c.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := c.exceptions.FindByStudentID(txCtx, studentID)
		if err != nil {
			return &ScheduleError{Op: c.operation("get student %s exceptions"), Err: err}
		}

		// Delete the locked snapshot row-by-row. A newly-created exception on a
		// different date is intentionally left alone; a broad DELETE could race
		// with partial creation and silently remove its provenance.
		type candidate struct {
			id   int64
			date timezone.Date
		}
		candidates := make([]candidate, 0, len(rows))
		for _, row := range rows {
			if isZeroEntity(row) {
				continue
			}
			fields := c.domain.ExceptionFields(row)
			candidates = append(candidates, candidate{id: fields.ID, date: fields.Date})
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].date == candidates[j].date {
				return candidates[i].id < candidates[j].id
			}
			return candidates[i].date.Before(candidates[j].date)
		})

		var lockedDate timezone.Date
		for index, row := range candidates {
			if index == 0 || row.date != lockedDate {
				if err := LockCareExceptionDay(txCtx, c.db, studentID, row.date); err != nil {
					return err
				}
				lockedDate = row.date
			}
		}
		for _, row := range candidates {
			fresh, err := c.exceptions.FindByIDForUpdate(txCtx, row.id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return &ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
			}
			if isZeroEntity(fresh) {
				continue
			}
			if c.domain.ExceptionFields(fresh).ExcusedFrom != nil {
				return ErrCareExceptionContainsPartialAbsence
			}
			if err := c.deleteExceptionRow(txCtx, row.id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *effectiveTimeCore[S, E, N, D]) deleteExceptionRow(ctx context.Context, exceptionID int64) error {
	if err := c.exceptions.Delete(ctx, exceptionID); err != nil {
		return &ScheduleError{
			Op:  c.operation("delete student %s exception"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) NoteByID(
	ctx context.Context,
	noteID int64,
) (N, error) {
	row, err := c.notes.FindByID(ctx, noteID)
	if err != nil {
		var zero N
		return zero, &ScheduleError{
			Op:  c.operation("get student %s note by id"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) Notes(
	ctx context.Context,
	studentID int64,
) ([]N, error) {
	rows, err := c.notes.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{
			Op:  c.operation("get student %s notes"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) NotesForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) ([]N, error) {
	rows, err := c.notes.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, &ScheduleError{
			Op:  c.operation("get student %s notes for date"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) CreateNote(
	ctx context.Context,
	row N,
) error {
	op := c.operation("create student %s note")
	if err := row.Validate(); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	row.SetTenantID(tenant.FromContext(ctx))
	if err := c.notes.Create(ctx, row); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) UpdateNote(
	ctx context.Context,
	row N,
) error {
	op := c.operation("update student %s note")
	if err := row.Validate(); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	if err := c.notes.Update(ctx, row); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteNote(
	ctx context.Context,
	noteID int64,
) error {
	if err := c.notes.Delete(ctx, noteID); err != nil {
		return &ScheduleError{
			Op:  c.operation("delete student %s note"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) DeleteAllNotes(
	ctx context.Context,
	studentID int64,
) error {
	if err := c.notes.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{
			Op:  c.operation("delete all student %s notes"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) Data(
	ctx context.Context,
	studentID int64,
) (*effectiveTimeData[S, E, N], error) {
	op := c.operation("get student %s data")
	schedules, err := c.schedules.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	exceptions, err := c.exceptions.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	notes, err := c.notes.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	return &effectiveTimeData[S, E, N]{
		Schedules:  schedules,
		Exceptions: exceptions,
		Notes:      notes,
	}, nil
}

func (c *effectiveTimeCore[S, E, N, D]) EffectiveTimeForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*effectiveTimeResult, error) {
	weekday := effectiveISOWeekday(date)
	result := &effectiveTimeResult{
		Date:        date,
		WeekdayName: scheduleModel.WeekdayNames[weekday],
	}
	if weekday > scheduleModel.WeekdayFriday {
		return result, nil
	}

	op := c.operation("get effective %s time")
	exception, err := c.exceptions.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	if !isZeroEntity(exception) {
		fields := c.domain.ExceptionFields(exception)
		result.IsException = true
		result.Time = fields.Time
		result.Notes = trimmed(fields.Reason)
		if result.Notes == "" {
			schedule, err := c.schedules.FindByStudentIDAndWeekday(ctx, studentID, weekday)
			if err != nil {
				return nil, &ScheduleError{Op: op, Err: err}
			}
			result.Notes = c.scheduleNotes(schedule)
		}
	} else {
		schedule, err := c.schedules.FindByStudentIDAndWeekday(ctx, studentID, weekday)
		if err != nil {
			return nil, &ScheduleError{Op: op, Err: err}
		}
		if !isZeroEntity(schedule) {
			fields := c.domain.ScheduleFields(schedule)
			value := fields.Time
			result.Time = &value
			result.Notes = trimmed(fields.Notes)
		}
	}

	notes, err := c.notes.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	for _, note := range notes {
		fields := c.domain.NoteFields(note)
		result.DayNotes = append(result.DayNotes, effectiveDayNote{
			ID:      fields.ID,
			Content: fields.Content,
		})
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) BulkEffectiveTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*effectiveTimeResult, error) {
	if len(studentIDs) == 0 {
		return map[int64]*effectiveTimeResult{}, nil
	}

	weekday := effectiveISOWeekday(date)
	result := make(map[int64]*effectiveTimeResult, len(studentIDs))
	for _, studentID := range studentIDs {
		result[studentID] = &effectiveTimeResult{
			Date:        date,
			WeekdayName: scheduleModel.WeekdayNames[weekday],
		}
	}
	if weekday > scheduleModel.WeekdayFriday {
		return result, nil
	}

	op := c.operation("get bulk effective %s times")
	exceptions, err := c.exceptions.FindByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	exceptionMap := make(map[int64]E, len(exceptions))
	for _, exception := range exceptions {
		exceptionMap[c.domain.ExceptionFields(exception).StudentID] = exception
	}

	schedules, err := c.schedules.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	scheduleMap := make(map[int64]S, len(schedules))
	for _, schedule := range schedules {
		scheduleMap[c.domain.ScheduleFields(schedule).StudentID] = schedule
	}

	notes, err := c.notes.FindByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	notesMap := make(map[int64][]N)
	for _, note := range notes {
		fields := c.domain.NoteFields(note)
		notesMap[fields.StudentID] = append(notesMap[fields.StudentID], note)
	}

	for _, studentID := range studentIDs {
		target := result[studentID]
		schedule := scheduleMap[studentID]
		if exception, ok := exceptionMap[studentID]; ok {
			fields := c.domain.ExceptionFields(exception)
			target.IsException = true
			target.Time = fields.Time
			target.Notes = trimmed(fields.Reason)
			if target.Notes == "" {
				target.Notes = c.scheduleNotes(schedule)
			}
		} else if !isZeroEntity(schedule) {
			fields := c.domain.ScheduleFields(schedule)
			value := fields.Time
			target.Time = &value
			target.Notes = trimmed(fields.Notes)
		}

		for _, note := range notesMap[studentID] {
			fields := c.domain.NoteFields(note)
			target.DayNotes = append(target.DayNotes, effectiveDayNote{
				ID:      fields.ID,
				Content: fields.Content,
			})
		}
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) WeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]S, error) {
	return c.schedules.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
}

func (c *effectiveTimeCore[S, E, N, D]) WeeklySchedulesByStudentIDs(
	ctx context.Context,
	studentIDs []int64,
) ([]S, error) {
	return c.schedules.FindByStudentIDs(ctx, studentIDs)
}

func (c *effectiveTimeCore[S, E, N, D]) scheduleNotes(schedule S) string {
	if isZeroEntity(schedule) {
		return ""
	}
	return trimmed(c.domain.ScheduleFields(schedule).Notes)
}

func effectiveISOWeekday(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return scheduleModel.WeekdaySunday
	}
	return weekday
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isZeroEntity[T any](value T) bool {
	return reflect.ValueOf(value).IsZero()
}
