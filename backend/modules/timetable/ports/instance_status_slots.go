package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StatusSlotRows are the participant-slot writes of a Care Plan status day:
// the retained instance-student rows a scheduled sick or excused day marks
// and a withdrawn one releases.
type StatusSlotRows interface {
	ApplyStatusDay(ctx context.Context, studentID int64, date calendar.Date, statusDayID int64, substatus string) (int, error)
	ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error)
}
