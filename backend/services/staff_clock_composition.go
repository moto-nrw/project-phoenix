package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	staffclock "github.com/moto-nrw/project-phoenix/workflows/staffclock/compose"
)

// staffClockClock is the wall clock and the Berlin calendar day the kiosk
// workflow reasons with.
type staffClockClock struct{}

func (staffClockClock) Now() time.Time               { return time.Now() }
func (staffClockClock) Day(instant time.Time) string { return timezone.DateFromTime(instant).String() }

// NewStaffClockService composes the staff-clock workflow over the retained
// person service, the Workforce time clock and the card owner's capability.
func NewStaffClockService(people *peopleCompose.PersonDirectory, cards identityaccess.RFIDCards, clock workforce.TimeClock) *staffclock.Service {
	return staffclock.NewService(staffclock.Dependencies{
		Cards:     rfidCardLookup{cards},
		Staff:     StaffClockStaffLookup(people),
		TimeClock: StaffClockTimeClock(clock),
		Clock:     staffClockClock{},
	})
}
