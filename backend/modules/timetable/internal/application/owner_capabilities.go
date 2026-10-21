package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func NewEndedSessionInstances(rows ports.ActivityInstanceRecords) (ports.EndedSessionInstances, error) {
	instances, ok := rows.(ports.EndedSessionInstances)
	if !ok {
		return nil, fmt.Errorf("ended session completion: %T cannot list instances by session", rows)
	}
	return instances, nil
}

// SickCascadeTimetableRows serves the shift-plan-sync workflow's timetable
// port from the retained rows, in the Timetable owner's public vocabulary.
type SickCascadeTimetableRows struct {
	TimetableInstanceReads
	TimetableInstanceStaffReads
	dayLock func(context.Context, calendar.Date) error
}

// AcquireSubstituteDayLock serializes the day-wide staffing mutations of the
// tenant within the caller's transaction.
func (r SickCascadeTimetableRows) AcquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return r.dayLock(ctx, date)
}

func NewSickCascadeTimetableRows(instances ports.ActivityInstanceRecords, staff ports.InstanceStaffRecords, dayLock func(context.Context, calendar.Date) error) SickCascadeTimetableRows {
	return SickCascadeTimetableRows{TimetableInstanceReads: NewTimetableInstanceReads(instances), TimetableInstanceStaffReads: NewTimetableInstanceStaffReads(staff), dayLock: dayLock}
}
