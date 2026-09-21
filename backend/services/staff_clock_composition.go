package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// staffClockClock is the wall clock and the Berlin calendar day the kiosk
// workflow reasons with.
type staffClockClock struct{}

func (staffClockClock) Now() time.Time               { return time.Now() }
func (staffClockClock) Day(instant time.Time) string { return timezone.DateFromTime(instant).String() }

// newStaffClockService composes the staff-clock workflow over the retained
// person and work session services (#2690), and the card owner's capability.
func newStaffClockService(people users.PersonService, cards identityaccess.RFIDCards, sessions timetracking.WorkSessionService) *staffclock.Service {
	return staffclock.NewService(staffclock.Dependencies{
		Cards:     rfidCardLookup{cards},
		Staff:     StaffClockStaffLookup(people),
		TimeClock: StaffClockTimeClock(TimeClockCapability(sessions)),
		Clock:     staffClockClock{},
	})
}
