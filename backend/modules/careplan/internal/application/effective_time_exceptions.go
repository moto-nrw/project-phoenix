package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (c *effectiveTimeCore[S, E, N, D]) exceptionByID(
	ctx context.Context,
	exceptionID int64,
) (E, error) {
	row, err := c.exceptions.FindByID(ctx, exceptionID)
	if err != nil {
		var zero E
		return zero, &careplan.ScheduleError{
			Op:  c.operation("get student %s exception by id"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) exceptionForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (E, error) {
	row, err := c.exceptions.FindByStudentIDAndDate(ctx, studentID, timezone.Date(date))
	if err != nil {
		var zero E
		return zero, &careplan.ScheduleError{
			Op:  c.operation("get student %s exception for date"),
			Err: err,
		}
	}
	return row, nil
}

func (c *effectiveTimeCore[S, E, N, D]) loadExceptions(
	ctx context.Context,
	studentID int64,
) ([]E, error) {
	rows, err := c.exceptions.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  c.operation("get student %s exceptions"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) upcomingExceptions(
	ctx context.Context,
	studentID int64,
) ([]E, error) {
	rows, err := c.exceptions.FindUpcomingByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{
			Op:  c.operation("get upcoming student %s exceptions"),
			Err: err,
		}
	}
	return rows, nil
}

func (c *effectiveTimeCore[S, E, N, D]) createException(
	ctx context.Context,
	row E,
) error {
	op := c.operation("create student %s exception")
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}

	fields := c.domain.ExceptionFields(row)
	existing, err := c.exceptions.FindByStudentIDAndDate(ctx, fields.StudentID, fields.Date)
	if err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	if !isZeroEntity(existing) {
		if c.domain.CollisionPolicy() == domain.ExceptionCollisionReject {
			return &careplan.ScheduleError{
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
			normalized := timezone.NormalizeWallClock(*fields.Time)
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
			return &careplan.ScheduleError{Op: op, Err: err}
		}
		c.domain.AssignException(row, updated)
		return nil
	}

	row.SetTenantID(c.transactions.TenantID(ctx))
	if err := c.exceptions.Create(ctx, row); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) updateExceptionRow(
	ctx context.Context,
	row E,
) error {
	op := c.operation("update student %s exception")
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}

	fields := c.domain.ExceptionFields(row)
	existing, err := c.exceptions.FindByStudentIDAndDate(ctx, fields.StudentID, fields.Date)
	if err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	if !isZeroEntity(existing) && c.domain.ExceptionFields(existing).ID != fields.ID {
		return &careplan.ScheduleError{
			Op:  op,
			Err: errors.New("exception already exists for this date"),
		}
	}

	if err := c.exceptions.Update(ctx, row); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) createOrReclaimException(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
	value *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (E, error) {
	var result E
	err := c.transactions.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := c.transactions.LockStudentAndExceptionDay(txCtx, studentID, date.String()); err != nil {
			return err
		}

		existing, err := c.exceptionForDate(txCtx, studentID, date)
		if err != nil {
			return err
		}

		fields := domain.EffectiveExceptionFields{
			StudentID: studentID, Date: date, Time: value, Reason: reason,
			Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
		}
		if !isZeroEntity(existing) {
			var reclaimErr error
			result, reclaimErr = c.reclaimException(txCtx, existing, fields, resolveStaffID)
			return reclaimErr
		}
		result = c.domain.NewException(fields)
		return c.createException(txCtx, result)
	})
	if err != nil {
		var zero E
		return zero, err
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) updateException(
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
	err := c.transactions.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := c.transactions.LockStudentAndExceptionDay(txCtx, studentID, date.String()); err != nil {
			return err
		}

		fresh, err := c.exceptionByID(txCtx, exceptionID)
		if err != nil {
			return err
		}
		if isZeroEntity(fresh) {
			return careplan.ErrCareExceptionNotFound
		}

		fields := c.domain.ExceptionFields(fresh)

		if err := prepareExceptionUpdate(&fields, studentID, date, clearValue, resolveStaffID); err != nil {
			return err
		}
		fields.ApplyTimePatch(date, reason, value, clearValue)

		result = c.domain.NewException(fields)
		return c.updateExceptionRow(txCtx, result)
	})
	if err != nil {
		var zero E
		return zero, err
	}
	return result, nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteException(
	ctx context.Context,
	exceptionID, studentID int64,
) error {
	initial, err := c.exceptions.FindByID(ctx, exceptionID)
	if err != nil {
		if c.transactions.IsNotFound(err) {
			return careplan.ErrCareExceptionNotFound
		}
		return &careplan.ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
	}
	if isZeroEntity(initial) {
		return c.deleteExceptionRow(ctx, exceptionID)
	}

	fields := c.domain.ExceptionFields(initial)
	if fields.StudentID != studentID {
		return careplan.ErrCareExceptionWrongStudent
	}
	return c.transactions.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := c.transactions.LockStudentAndExceptionDay(txCtx, fields.StudentID, fields.Date.String()); err != nil {
			return err
		}
		return c.deleteLockedException(txCtx, exceptionID, studentID)
	})
}

func (c *effectiveTimeCore[S, E, N, D]) deleteLockedException(txCtx context.Context, exceptionID, studentID int64) error {
	// Re-read after taking the same lock used by partial-absence writes.
	// Otherwise a partial could be attached between the check and delete.
	fresh, err := c.exceptions.FindByIDForUpdate(txCtx, exceptionID)
	if err != nil {
		if c.transactions.IsNotFound(err) {
			return careplan.ErrCareExceptionNotFound
		}
		return &careplan.ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
	}
	if !isZeroEntity(fresh) && c.domain.ExceptionFields(fresh).StudentID != studentID {
		return careplan.ErrCareExceptionWrongStudent
	}
	if !isZeroEntity(fresh) && c.domain.ExceptionFields(fresh).ExcusedFrom != nil {
		return careplan.ErrCareExceptionContainsPartialAbsence
	}
	return c.deleteExceptionRow(txCtx, exceptionID)
}

func (c *effectiveTimeCore[S, E, N, D]) deleteAllExceptions(
	ctx context.Context,
	studentID int64,
) error {
	return c.transactions.WithinTenant(ctx, func(txCtx context.Context) error {
		rows, err := c.exceptions.FindByStudentID(txCtx, studentID)
		if err != nil {
			return &careplan.ScheduleError{Op: c.operation("get student %s exceptions"), Err: err}
		}

		// Lock the snapshot before deleting any row. A new exception on another
		// date stays untouched, including its partial-absence provenance.
		candidates := c.exceptionDeletionSnapshot(rows)
		if err := c.lockExceptionSnapshot(txCtx, studentID, candidates); err != nil {
			return err
		}
		for _, row := range candidates {
			if err := c.deleteSnapshotException(txCtx, row.id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *effectiveTimeCore[S, E, N, D]) deleteExceptionRow(ctx context.Context, exceptionID int64) error {
	if err := c.exceptions.Delete(ctx, exceptionID); err != nil {
		return &careplan.ScheduleError{
			Op:  c.operation("delete student %s exception"),
			Err: err,
		}
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) reclaimException(ctx context.Context, existing E, fields domain.EffectiveExceptionFields, resolveStaffID func() (int64, error)) (E, error) {
	previous := c.domain.ExceptionFields(existing)
	var zero E
	if previous.Source != careplan.ExceptionSourceGuardian {
		return zero, careplan.ErrCareExceptionDayConflict
	}
	// Reclaiming a partial-absence row would erase its provenance.
	if previous.ExcusedFrom != nil {
		return zero, careplan.ErrCareExceptionContainsPartialAbsence
	}
	staffID, err := resolveStaffID()
	if err != nil || staffID == 0 {
		return zero, careplan.ErrCareExceptionStaffProfileRequired
	}
	fields.ID, fields.CreatedAt, fields.TenantID = previous.ID, previous.CreatedAt, previous.TenantID
	fields.CreatedBy = staffID
	result := c.domain.NewException(fields)
	return result, c.updateExceptionRow(ctx, result)
}

func prepareExceptionUpdate(fields *domain.EffectiveExceptionFields, studentID int64, date timezone.Date, clearValue bool, resolveStaffID func() (int64, error)) error {
	if fields.StudentID != studentID {
		return careplan.ErrCareExceptionWrongStudent
	}
	if fields.ExcusedFrom != nil && (date != fields.Date || clearValue) {
		return careplan.ErrCareExceptionContainsPartialAbsence
	}
	if fields.Source != careplan.ExceptionSourceGuardian {
		return nil
	}
	staffID, err := resolveStaffID()
	if err != nil || staffID == 0 {
		return careplan.ErrCareExceptionStaffProfileRequired
	}
	fields.Source = careplan.ExceptionSourceStaff
	fields.CreatedBy = staffID
	fields.CreatedByGuardian = nil
	return nil
}

type exceptionDeletionCandidate struct {
	id   int64
	date timezone.Date
}

func (c *effectiveTimeCore[S, E, N, D]) exceptionDeletionSnapshot(rows []E) []exceptionDeletionCandidate {
	candidates := make([]exceptionDeletionCandidate, 0, len(rows))
	for _, row := range rows {
		if isZeroEntity(row) {
			continue
		}
		fields := c.domain.ExceptionFields(row)
		candidates = append(candidates, exceptionDeletionCandidate{id: fields.ID, date: fields.Date})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].date == candidates[j].date {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].date.Before(candidates[j].date)
	})
	return candidates
}

func (c *effectiveTimeCore[S, E, N, D]) lockExceptionSnapshot(ctx context.Context, studentID int64, candidates []exceptionDeletionCandidate) error {
	var lockedDate timezone.Date
	for index, row := range candidates {
		if index > 0 && row.date == lockedDate {
			continue
		}
		if err := c.transactions.LockStudentAndExceptionDay(ctx, studentID, row.date.String()); err != nil {
			return err
		}
		lockedDate = row.date
	}
	return nil
}

func (c *effectiveTimeCore[S, E, N, D]) deleteSnapshotException(ctx context.Context, id int64) error {
	fresh, err := c.exceptions.FindByIDForUpdate(ctx, id)
	if err != nil {
		if c.transactions.IsNotFound(err) {
			return nil
		}
		return &careplan.ScheduleError{Op: c.operation("get student %s exception by id"), Err: err}
	}
	if isZeroEntity(fresh) {
		return nil
	}
	if c.domain.ExceptionFields(fresh).ExcusedFrom != nil {
		return careplan.ErrCareExceptionContainsPartialAbsence
	}
	return c.deleteExceptionRow(ctx, id)
}
