package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// InstanceStatusSlots serves the status-day slot writes in plain YYYY-MM-DD
// calendar days, for the composition root that binds them to Care Plan.
type InstanceStatusSlots struct{ ports.StatusSlotRows }

// NewInstanceStatusSlots binds the slot writes to the retained rows.
func NewInstanceStatusSlots(rows ports.StatusSlotRows) InstanceStatusSlots {
	return InstanceStatusSlots{StatusSlotRows: rows}
}

// ApplyStatusDay marks the student's slots of the day with the status day.
func (s InstanceStatusSlots) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error) {
	return s.StatusSlotRows.ApplyStatusDay(ctx, studentID, calendar.Date(date), statusDayID, substatus)
}
