package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// SickCascadeTimetableRows serves the shift-plan-sync workflow's timetable
// port from the retained rows, in the Timetable owner's public vocabulary.
type SickCascadeTimetableRows struct {
	TimetableInstanceReads
	TimetableInstanceStaffReads
	// RecurrenceWriteLock is the tenant recurrence gate, which the
	// materialization and the series writes hold while they plan rows from
	// the sick reports; a sick-report write takes it too (#6232).
	timetable.RecurrenceWriteLock
	dayLock func(context.Context, string) error
}

// AcquireSubstituteDayLock serializes the day-wide staffing mutations of the
// tenant within the caller's transaction, in the lock order of every other
// day-lock writer (lockScheduleDays).
func (r SickCascadeTimetableRows) AcquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return lockScheduleDays(ctx, r.dayLock, tenant.FromContext(ctx), date)
}

// NewSickCascadeTimetableRows binds the rows, dayLock, which takes one
// transaction-scoped advisory lock on a hashed key, and the recurrence gate.
func NewSickCascadeTimetableRows(instances ports.LegacyInstanceReads, staff timetable.InstanceStaffCapability, dayLock func(context.Context, string) error, recurrence timetable.RecurrenceWriteLock) SickCascadeTimetableRows {
	return SickCascadeTimetableRows{TimetableInstanceReads: NewTimetableInstanceReads(instances), TimetableInstanceStaffReads: NewTimetableInstanceStaffReads(staff), RecurrenceWriteLock: recurrence, dayLock: dayLock}
}
