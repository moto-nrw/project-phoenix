package application

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// StatusSlotRows are the participant-slot writes of a Care Plan status day:
// the retained instance-student rows a scheduled sick or excused day marks
// and a withdrawn one releases.
type StatusSlotRows interface {
	ApplyStatusDay(ctx context.Context, studentID int64, date scheduleModels.Date, statusDayID int64, substatus string) (int, error)
	ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error)
}

// InstanceStatusSlots serves the status-day slot writes in plain YYYY-MM-DD
// calendar days, for the composition root that binds them to Care Plan.
type InstanceStatusSlots struct{ rows StatusSlotRows }

// NewInstanceStatusSlots binds the slot writes to the retained rows.
func NewInstanceStatusSlots(rows StatusSlotRows) InstanceStatusSlots {
	return InstanceStatusSlots{rows: rows}
}

// ApplyStatusDay marks the student's slots of the day with the status day.
func (s InstanceStatusSlots) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error) {
	return s.rows.ApplyStatusDay(ctx, studentID, scheduleModels.Date(date), statusDayID, substatus)
}

// ReleaseStatusDay releases the slots the status day marked.
func (s InstanceStatusSlots) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	return s.rows.ReleaseStatusDay(ctx, statusDayID)
}
