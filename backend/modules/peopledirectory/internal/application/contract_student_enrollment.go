package application

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func enrollmentInput(input peopledirectory.EnrollmentStudent) domain.EnrollmentStudent {
	result := domain.EnrollmentStudent{PersonID: input.PersonID, SchoolClass: input.SchoolClass, Status: input.Status, EnrolledFrom: input.EnrolledFrom, EnrolledUntil: input.EnrolledUntil}
	if input.InitialProfile != nil {
		profile := domain.EnrollmentProfilePatch(*input.InitialProfile)
		result.InitialProfile = &profile
	}
	return result
}
