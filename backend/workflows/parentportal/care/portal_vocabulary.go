package care

import (
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The child-flow results and values the guardian portal HTTP composition
// renders, named by the flow that returns them. The composition matches them
// through the workflow port instead of importing the owner models (#3421).
type (
	AllowedDepartureModes    = usersModels.AllowedDepartureModes
	ChildSummary             = parentModels.ChildSummary
	DepartureDays            = usersModels.DepartureDays
	DepartureMode            = usersModels.DepartureMode
	EnrollablePhase          = parentModels.EnrollablePhase
	EnrollmentRequestSummary = parentModels.EnrollmentRequestSummary
	GuardianSubmitStatus     = parentModels.GuardianSubmitStatus
	StudentDataChangeRequest = usersModels.StudentDataChangeRequest
)

const (
	DataChangeStatusPending = usersModels.DataChangeStatusPending
	DataChangeTargetPerson  = usersModels.DataChangeTargetPerson
	DataChangeTargetStudent = usersModels.DataChangeTargetStudent
	DeparturePickup         = usersModels.DeparturePickup
)

// ErrGuardianProfileNotFound is the profile flows' answer for an account
// without a guardian profile.
var ErrGuardianProfileNotFound = usersModels.ErrGuardianProfileNotFound

// CanonicalDayToISOWeekday translates a stored care-offering day ("mon")
// into its ISO weekday number, the mapping the offering flow stores with.
func CanonicalDayToISOWeekday(day string) (int, bool) {
	return enrollmentModels.CanonicalDayToISOWeekday(day)
}
