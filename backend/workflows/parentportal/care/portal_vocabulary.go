package care

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The child-flow results and values the guardian portal HTTP composition
// renders, named by the flow that returns them. The composition matches them
// through the workflow port instead of importing the owner models (#3421).
type (
	AllowedDepartureModes = PortalAllowedDepartureModes
	DepartureDays         = PortalDepartureDays
	DepartureMode         = PortalDepartureMode
)

const (
	DeparturePickup = PortalDepartureMode("pickup")
)

// ErrGuardianProfileNotFound is the profile flows' answer for an account
// without a guardian profile.
var ErrGuardianProfileNotFound = errors.New("guardian profile not found")

// CanonicalDayToISOWeekday translates a stored care-offering day ("mon")
// into its ISO weekday number, the mapping the offering flow stores with.
func CanonicalDayToISOWeekday(day string) (int, bool) {
	return calendar.CanonicalDayToISOWeekday(day)
}

const (
	DepartureAlone       PortalDepartureMode = "alone"
	DepartureAccompanied PortalDepartureMode = "accompanied"
)

var ErrStudentGuardianNotFound = errors.New("student guardian relationship not found")
