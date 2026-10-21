package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// InstanceStatusSlots serves the status-day slot writes in plain YYYY-MM-DD
// calendar days, for the composition root that binds them to Care Plan.
type InstanceStatusSlots struct {
	presence ports.ParticipantPresence
}

// NewInstanceStatusSlots binds the slot writes to the participants'
// attendance.
func NewInstanceStatusSlots(rows ports.ParticipantRows) InstanceStatusSlots {
	return InstanceStatusSlots{presence: rows.Presence}
}

// ApplyStatusDay marks the student's slots of the day with the status day.
func (s InstanceStatusSlots) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error) {
	changed, err := s.presence.ApplyStatusDay(ctx, studentID, date, statusDayID, substatus)
	if err != nil {
		return 0, WrapDatabaseError("apply student status day to slots", err)
	}
	return changed, nil
}

// ReleaseStatusDay releases the slots the status day marked.
func (s InstanceStatusSlots) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	changed, err := s.presence.ReleaseStatusDay(ctx, statusDayID)
	if err != nil {
		return 0, WrapDatabaseError("release student status day from slots", err)
	}
	return changed, nil
}
