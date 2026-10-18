//go:build testhooks

package api

import (
	"log/slog"

	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
)

func NewStudentEnrollmentHTTPTestResource(module StudentTestModule) *enrollmentHTTP.StudentResource {
	directory := studentsAPI.NewResource(studentsAPI.ResourceConfig{PeopleDirectory: module.PeopleDirectory, UserContextService: module.UserContext})
	return &enrollmentHTTP.StudentResource{StudentReadAccess: directory.StudentReadAccess, Logger: slog.Default()}
}
