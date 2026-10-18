package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func NativeGuardianChildSummary(value *models.GuardianChildSummary) *directory.GuardianChildSummary {
	if value == nil {
		return nil
	}
	result := &directory.GuardianChildSummary{}
	result.StudentID = value.StudentID
	result.FirstName = value.FirstName
	result.LastName = value.LastName
	result.SchoolClass = value.SchoolClass
	result.EnrollmentSubmit = value.EnrollmentSubmit
	result.Status = value.Status
	return result
}
