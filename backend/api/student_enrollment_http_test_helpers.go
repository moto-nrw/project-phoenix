//go:build testhooks

package api

import (
	"log/slog"

	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
)

func NewStudentEnrollmentHTTPTestResource(people StudentTestPeople, userContext *CallerRows) *enrollmentHTTP.StudentResource {
	directory := studentsAPI.NewResource(studentsAPI.ResourceConfig{PeopleDirectory: people.PeopleDirectory, UserContextService: userContext})
	return &enrollmentHTTP.StudentResource{StudentReadAccess: directory.StudentReadAccess, Logger: slog.Default()}
}
