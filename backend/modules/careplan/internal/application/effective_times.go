package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// effectiveTimeCore is private implementation shared by arrival and pickup.
// Embedding it keeps persistence, exception precedence, and wall-clock rules
// local to each native application module without publishing a second service.
type effectiveTimeCore[
	S ports.EffectiveTimeEntity,
	E ports.EffectiveTimeEntity,
	N ports.EffectiveTimeEntity,
	D ports.EffectiveTimeDomain[S, E, N],
] struct {
	schedules    ports.EffectiveScheduleRepository[S]
	exceptions   ports.EffectiveExceptionRepository[E]
	notes        ports.EffectiveNoteRepository[N]
	transactions ports.EffectiveTimeTransaction
	domain       D
}

func newEffectiveTimeCore[
	S ports.EffectiveTimeEntity,
	E ports.EffectiveTimeEntity,
	N ports.EffectiveTimeEntity,
	D ports.EffectiveTimeDomain[S, E, N],
](
	schedules ports.EffectiveScheduleRepository[S],
	exceptions ports.EffectiveExceptionRepository[E],
	notes ports.EffectiveNoteRepository[N],
	transactions ports.EffectiveTimeTransaction,
	domain D,
) *effectiveTimeCore[S, E, N, D] {
	return &effectiveTimeCore[S, E, N, D]{
		schedules:    schedules,
		exceptions:   exceptions,
		notes:        notes,
		transactions: transactions,
		domain:       domain,
	}
}

func (c *effectiveTimeCore[S, E, N, D]) operation(action string) string {
	return fmt.Sprintf(action, c.domain.Name())
}

func (c *effectiveTimeCore[S, E, N, D]) loadSchedules(
	ctx context.Context,
	studentID int64,
) ([]S, error) {
	rows, err := c.schedules.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  c.operation("get student %s schedules"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) scheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (S, error) {
	if weekday < 1 || weekday > 5 {
		var zero S
		return zero, &careplan.ScheduleError{
			Op:  c.operation("get student %s schedule for weekday"),
			Err: errors.New("invalid weekday"),
		}
	}

	row, err := c.schedules.FindByStudentIDAndWeekday(ctx, studentID, weekday)
	if err != nil {
		var zero S
		return zero, &careplan.ScheduleError{
			Op:  c.operation("get student %s schedule for weekday"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) upsertSchedule(
	ctx context.Context,
	row S,
) error {
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("upsert student %s schedule"),
			Err: err,
		}
	}
	if err := c.schedules.UpsertSchedule(ctx, row); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("upsert student %s schedule"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) upsertBulkSchedules(
	ctx context.Context,
	studentID int64,
	rows []S,
) error {
	op := c.operation("upsert bulk student %s schedules")
	if err := c.schedules.DeleteByStudentID(ctx, studentID); err != nil {
		return &careplan.ScheduleError{
			Op:  op,
			Err: fmt.Errorf("failed to delete existing schedules: %w", err),
		}
	}

	for _, row := range rows {
		c.domain.SetScheduleStudentID(row, studentID)
		fields := c.domain.ScheduleFields(row)
		if err := row.Validate(); err != nil {
			return &careplan.ScheduleError{
				Op:  op,
				Err: fmt.Errorf("invalid schedule for weekday %d: %w", fields.Weekday, err),
			}
		}
		row.SetTenantID(c.transactions.TenantID(ctx))
		if err := c.schedules.Create(ctx, row); err != nil {
			return &careplan.ScheduleError{Op: op, Err: err}
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	if err := c.schedules.Delete(ctx, scheduleID); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("delete student %s schedule"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteAllSchedules(
	ctx context.Context,
	studentID int64,
) error {
	if err := c.schedules.DeleteByStudentID(ctx, studentID); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("delete all student %s schedules"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) noteByID(
	ctx context.Context,
	noteID int64,
) (N, error) {
	row, err := c.notes.FindByID(ctx, noteID)
	if err != nil {
		var zero N
		return zero, &careplan.ScheduleError{
			Op:  c.operation("get student %s note by id"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) loadNotes(
	ctx context.Context,
	studentID int64,
) ([]N, error) {
	rows, err := c.notes.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  c.operation("get student %s notes"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) notesForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) ([]N, error) {
	rows, err := c.notes.FindByStudentIDAndDate(ctx, studentID, calendar.Date(date))
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  c.operation("get student %s notes for date"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) createNote(
	ctx context.Context,
	row N,
) error {
	op := c.operation("create student %s note")
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	row.SetTenantID(c.transactions.TenantID(ctx))
	if err := c.notes.Create(ctx, row); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) updateNote(
	ctx context.Context,
	row N,
) error {
	op := c.operation("update student %s note")
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	if err := c.notes.Update(ctx, row); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteNote(
	ctx context.Context,
	noteID int64,
) error {
	if err := c.notes.Delete(ctx, noteID); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("delete student %s note"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteAllNotes(
	ctx context.Context,
	studentID int64,
) error {
	if err := c.notes.DeleteByStudentID(ctx, studentID); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("delete all student %s notes"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) data(
	ctx context.Context,
	studentID int64,
) (*domain.EffectiveTimeData[S, E, N], error) {
	op := c.operation("get student %s data")
	schedules, err := c.schedules.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	exceptions, err := c.exceptions.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	notes, err := c.notes.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	return &domain.EffectiveTimeData[S, E, N]{
		Schedules:  schedules,
		Exceptions: exceptions,
		Notes:      notes,
	}, nil
}

func (c *effectiveTimeCore[S, E, N, D]) effectiveTimeForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) (*domain.EffectiveTimeResult, error) {
	weekday := domain.ISOWeekday(date)
	var row S
	if weekday <= 5 {
		var err error
		row, err = c.schedules.FindByStudentIDAndWeekday(ctx, studentID, weekday)
		if err != nil {
			return nil, &careplan.ScheduleError{Op: c.operation("get effective %s time"), Err: err}
		}
	}
	return c.effectiveTimeForDateWithSchedule(ctx, studentID, date, row)
}

// EffectiveTimeForDateWithSchedule applies the shared exception/note merge to
// a caller-provided recurring row. Pickup planning uses it with the
// date-aware booking projection; arrival planning keeps using the stored row.
func (c *effectiveTimeCore[S, E, N, D]) effectiveTimeForDateWithSchedule(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
	scheduleRow S,
) (*domain.EffectiveTimeResult, error) {
	weekday := domain.ISOWeekday(date)
	result := &domain.EffectiveTimeResult{
		Date:        date,
		WeekdayName: domain.WeekdayName(weekday),
	}
	if weekday > 5 {
		return result, nil
	}

	op := c.operation("get effective %s time")
	exception, err := c.exceptions.FindByStudentIDAndDate(ctx, studentID, calendar.Date(date))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	c.applyScheduleAndException(result, scheduleRow, exception)

	notes, err := c.notes.FindByStudentIDAndDate(ctx, studentID, calendar.Date(date))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	c.appendDayNotes(result, notes)
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) applyScheduleAndException(
	result *domain.EffectiveTimeResult,
	schedule S,
	exception E,
) {
	// The recurring time is recorded even when an exception overrides the
	// day (#2294): "geht heute um 12:15" only becomes an instruction a
	// Lehrkraft can act on next to the "sonst 15:00" it deviates from.
	// An arrival row may mark a care day without carrying a time (#2414):
	// the class timetable supplies it, and where no class time exists the
	// day has no arrival time rather than midnight. Pickup rows always
	// carry a time, so the zero check never trips for them.
	var regularNotes string
	if !isZeroEntity(schedule) {
		fields := c.domain.ScheduleFields(schedule)
		regularNotes = trimmed(fields.Notes)
		if !fields.Time.IsZero() {
			value := fields.Time
			result.RegularTime = &value
		}
	}
	if !isZeroEntity(exception) {
		fields := c.domain.ExceptionFields(exception)
		result.IsException = true
		result.Time = fields.Time
		if !fields.CreatedAt.IsZero() {
			recorded := fields.CreatedAt
			result.ChangedAt = &recorded
		}
		result.Notes = trimmed(fields.Reason)
		if result.Notes == "" {
			result.Notes = c.scheduleNotes(schedule)
		}
		return
	}
	result.Notes = regularNotes
	if result.RegularTime != nil {
		// Own copy: Time and RegularTime must never alias the same value.
		effective := *result.RegularTime
		result.Time = &effective
	}
}

func (c *effectiveTimeCore[S, E, N, D]) appendDayNotes(result *domain.EffectiveTimeResult, notes []N) {
	for _, note := range notes {
		fields := c.domain.NoteFields(note)
		result.DayNotes = append(result.DayNotes, domain.EffectiveDayNote{ID: fields.ID, Content: fields.Content})
	}
}

func (c *effectiveTimeCore[S, E, N, D]) bulkEffectiveTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
) (map[int64]*domain.EffectiveTimeResult, error) {
	weekday := domain.ISOWeekday(date)
	scheduleMap := make(map[int64]S, len(studentIDs))
	if len(studentIDs) > 0 && weekday <= 5 {
		schedules, err := c.schedules.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
		if err != nil {
			return nil, &careplan.ScheduleError{Op: c.operation("get bulk effective %s times"), Err: err}
		}
		for _, schedule := range schedules {
			scheduleMap[c.domain.ScheduleFields(schedule).StudentID] = schedule
		}
	}
	return c.bulkEffectiveTimesForDateWithSchedules(ctx, studentIDs, date, scheduleMap)
}

// BulkEffectiveTimesForDateWithSchedules is the batched counterpart of
// EffectiveTimeForDateWithSchedule.
func (c *effectiveTimeCore[S, E, N, D]) bulkEffectiveTimesForDateWithSchedules(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
	scheduleMap map[int64]S,
) (map[int64]*domain.EffectiveTimeResult, error) {
	if len(studentIDs) == 0 {
		return map[int64]*domain.EffectiveTimeResult{}, nil
	}

	weekday := domain.ISOWeekday(date)
	result := initialEffectiveResults(studentIDs, date, weekday)
	if weekday > 5 {
		return result, nil
	}

	op := c.operation("get bulk effective %s times")
	exceptions, err := c.exceptions.FindByStudentIDsAndDate(ctx, studentIDs, calendar.Date(date))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	exceptionMap := c.exceptionsByStudent(exceptions)

	notes, err := c.notes.FindByStudentIDsAndDate(ctx, studentIDs, calendar.Date(date))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: op, Err: err}
	}
	notesMap := c.notesByStudent(notes)

	for _, studentID := range studentIDs {
		target := result[studentID]
		c.applyScheduleAndException(target, scheduleMap[studentID], exceptionMap[studentID])
		c.appendDayNotes(target, notesMap[studentID])
	}
	return result, nil
}

func initialEffectiveResults(
	studentIDs []int64,
	date calendar.Date,
	weekday int,
) map[int64]*domain.EffectiveTimeResult {
	result := make(map[int64]*domain.EffectiveTimeResult, len(studentIDs))
	for _, studentID := range studentIDs {
		result[studentID] = &domain.EffectiveTimeResult{
			Date: date, WeekdayName: domain.WeekdayName(weekday),
		}
	}
	return result
}

func (c *effectiveTimeCore[S, E, N, D]) exceptionsByStudent(rows []E) map[int64]E {
	result := make(map[int64]E, len(rows))
	for _, row := range rows {
		result[c.domain.ExceptionFields(row).StudentID] = row
	}
	return result
}

func (c *effectiveTimeCore[S, E, N, D]) notesByStudent(rows []N) map[int64][]N {
	result := make(map[int64][]N)
	for _, row := range rows {
		fields := c.domain.NoteFields(row)
		result[fields.StudentID] = append(result[fields.StudentID], row)
	}
	return result
}

func (c *effectiveTimeCore[S, E, N, D]) scheduleNotes(schedule S) string {
	if isZeroEntity(schedule) {
		return ""
	}
	return trimmed(c.domain.ScheduleFields(schedule).Notes)
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
