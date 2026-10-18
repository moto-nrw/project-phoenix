package api

import (
	projection "github.com/moto-nrw/project-phoenix/modules/timetableprojection/compose"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

func NewStudentActivityEnrollments() studentdeletion.ActivityEnrollments {
	return projection.New().StudentEnrollments(studentdeletioncompose.EnrollmentDatabase())
}
