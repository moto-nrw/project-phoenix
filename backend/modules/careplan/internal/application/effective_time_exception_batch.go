package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// exceptionBatch is one staff exception for the same day and value for
// several students, as one report writes it.
type exceptionBatch struct {
	studentIDs     []int64
	date           calendar.Date
	value          *time.Time
	reason         *string
	staffID        int64
	resolveStaffID func() (int64, error)
	// authorize rechecks a student after its locks; isRejection tells its
	// business refusal from a storage failure.
	authorize   func(context.Context, int64) error
	isRejection func(error) bool
}

// createOrReclaimExceptions is createOrReclaimException for every student of
// the batch in the given order, in one unit of work (#5824):
//
//  1. Each student is locked (student row, then care day) and rechecked
//     before the next one, as consecutive single commands lock them.
//  2. The current exceptions of the locked students are one read after the
//     last lock. Each student's row and care day stay locked from its own
//     lock on, so no writer can change what a read right after it saw.
//  3. The writes run in student order and stop at the first refusal: the
//     rows written before it and the refusal are those of the single
//     commands. A recheck refusal of a later student is returned only after
//     the earlier students are written; a storage failure ends the batch.
func (c *effectiveTimeCore[S, E, N, D]) createOrReclaimExceptions(ctx context.Context, batch exceptionBatch) error {
	return c.transactions.WithinTenant(ctx, func(txCtx context.Context) error {
		locked, stopErr := c.lockExceptionStudents(txCtx, batch)
		if stopErr != nil && !batch.isRejection(stopErr) {
			return stopErr
		}
		existing, err := c.exceptionsForDate(txCtx, locked, batch.date)
		if err != nil {
			return err
		}
		for _, studentID := range locked {
			if err := c.writeLockedException(txCtx, batch, studentID, existing[studentID]); err != nil {
				return err
			}
		}
		return stopErr
	})
}

// lockExceptionStudents locks and rechecks the students in order and returns
// those that passed before the first failure, with that failure.
func (c *effectiveTimeCore[S, E, N, D]) lockExceptionStudents(ctx context.Context, batch exceptionBatch) ([]int64, error) {
	locked := make([]int64, 0, len(batch.studentIDs))
	for _, studentID := range batch.studentIDs {
		if err := c.transactions.LockStudentAndExceptionDay(ctx, studentID, batch.date.String()); err != nil {
			return locked, err
		}
		if err := batch.authorize(ctx, studentID); err != nil {
			return locked, err
		}
		locked = append(locked, studentID)
	}
	return locked, nil
}

// exceptionsForDate reads the locked students' exceptions of the day.
func (c *effectiveTimeCore[S, E, N, D]) exceptionsForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]E, error) {
	byStudent := make(map[int64]E, len(studentIDs))
	if len(studentIDs) == 0 {
		return byStudent, nil
	}
	rows, err := c.exceptions.FindByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: c.operation("get student %s exception for date"), Err: err}
	}
	for _, row := range rows {
		if !isZeroEntity(row) {
			byStudent[c.domain.ExceptionFields(row).StudentID] = row
		}
	}
	return byStudent, nil
}

// writeLockedException creates the student's exception or reclaims the
// existing one. Under the locks the read above is still current, so the
// single command's second read of the day is not repeated.
func (c *effectiveTimeCore[S, E, N, D]) writeLockedException(ctx context.Context, batch exceptionBatch, studentID int64, existing E) error {
	fields := domain.EffectiveExceptionFields{
		StudentID: studentID, Date: batch.date, Time: batch.value, Reason: batch.reason,
		Source: careplan.ExceptionSourceStaff, CreatedBy: batch.staffID,
	}
	if isZeroEntity(existing) {
		row := c.domain.NewException(fields)
		op := c.operation("create student %s exception")
		if err := row.Validate(); err != nil {
			return &careplan.ScheduleError{Op: op, Err: err}
		}
		row.SetTenantID(c.transactions.TenantID(ctx))
		if err := c.exceptions.Create(ctx, row); err != nil {
			return &careplan.ScheduleError{Op: op, Err: err}
		}
		return nil
	}
	fields, err := c.reclaimedFields(existing, fields, batch.resolveStaffID)
	if err != nil {
		return err
	}
	row := c.domain.NewException(fields)
	op := c.operation("update student %s exception")
	if err := row.Validate(); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	if err := c.exceptions.Update(ctx, row); err != nil {
		return &careplan.ScheduleError{Op: op, Err: err}
	}
	return nil
}
