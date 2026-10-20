package peopledirectory

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
)

// EnrollmentStudent contains only the student facts an enrollment decision
// creates or renews. It cannot overwrite health, consent or departure state.
// Dates are calendar days in YYYY-MM-DD format.
type EnrollmentStudent = enrollment.Input

type CreatedEnrollmentStudent = enrollment.CreatedStudent
type EnrollmentProfilePatch = enrollment.ProfilePatch
type EnrollmentRecord = enrollment.Record
