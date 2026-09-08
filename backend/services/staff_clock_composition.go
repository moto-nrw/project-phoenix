package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// staffClockClock is the wall clock and the Berlin calendar day the kiosk
// workflow reasons with.
type staffClockClock struct{}

func (staffClockClock) Now() time.Time               { return time.Now() }
func (staffClockClock) Day(instant time.Time) string { return timezone.DateFromTime(instant).String() }

// newStaffClockService composes the staff-clock workflow over the retained
// person and work session services (#2690). find is the identity-access card
// repository lookup.
func newStaffClockService[C interface {
	comparable
	rfidCard
}](people users.PersonService, find func(context.Context, string) (C, error), sessions active.WorkSessionService) *staffclock.Service {
	return staffclock.NewService(staffclock.Dependencies{
		Cards:     newRFIDCardLookup(find),
		Staff:     StaffClockStaffLookup(people),
		TimeClock: StaffClockTimeClock(TimeClockCapability(sessions)),
		Clock:     staffClockClock{},
	})
}
