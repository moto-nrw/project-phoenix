package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	// authorize, when set, rechecks a student after its locks.
	authorize func(context.Context, int64) error
}

// createOrReclaimLocked writes one staff exception per student of the batch
// in the given order inside the caller's unit of work (#5824). Each student
// is handled completely before the next one is touched, exactly as one
// single command per student: student row lock, care-day lock, recheck,
// read of the day's exception, write. The first failure of any kind ends the
// batch; no later student is locked. Under these locks the day's exception
// cannot change between the read and the write, so the second read the
// single command did before writing is not repeated.
func (c *effectiveTimeCore[S, E, N, D]) createOrReclaimLocked(ctx context.Context, batch exceptionBatch) ([]E, error) {
	written := make([]E, 0, len(batch.studentIDs))
	for _, studentID := range batch.studentIDs {
		if err := c.transactions.LockStudentAndExceptionDay(ctx, studentID, batch.date.String()); err != nil {
			return written, err
		}
		// School-scoped writes must revalidate their subject after the student
		// lock, in the same unit as conflict checks and exception persistence.
		if batch.authorize != nil {
			if err := batch.authorize(ctx, studentID); err != nil {
				return written, err
			}
		}
		existing, err := c.exceptionForDate(ctx, studentID, batch.date)
		if err != nil {
			return written, err
		}
		row, err := c.writeLockedException(ctx, batch, studentID, existing)
		if err != nil {
			return written, err
		}
		written = append(written, row)
	}
	return written, nil
}

// writeLockedException creates the student's exception or reclaims the
// existing one under the student's locks.
func (c *effectiveTimeCore[S, E, N, D]) writeLockedException(ctx context.Context, batch exceptionBatch, studentID int64, existing E) (E, error) {
	var zero E
	fields := domain.EffectiveExceptionFields{
		StudentID: studentID, Date: batch.date, Time: batch.value, Reason: batch.reason,
		Source: careplan.ExceptionSourceStaff, CreatedBy: batch.staffID,
	}
	if isZeroEntity(existing) {
		row := c.domain.NewException(fields)
		op := c.operation("create student %s exception")
		if err := row.Validate(); err != nil {
			return zero, &careplan.ScheduleError{Op: op, Err: err}
		}
		row.SetTenantID(c.transactions.TenantID(ctx))
		if err := c.exceptions.Create(ctx, row); err != nil {
			return zero, &careplan.ScheduleError{Op: op, Err: err}
		}
		return row, nil
	}
	fields, err := c.reclaimedFields(existing, fields, batch.resolveStaffID)
	if err != nil {
		return zero, err
	}
	row := c.domain.NewException(fields)
	op := c.operation("update student %s exception")
	if err := row.Validate(); err != nil {
		return zero, &careplan.ScheduleError{Op: op, Err: err}
	}
	if err := c.exceptions.Update(ctx, row); err != nil {
		return zero, &careplan.ScheduleError{Op: op, Err: err}
	}
	return row, nil
}
