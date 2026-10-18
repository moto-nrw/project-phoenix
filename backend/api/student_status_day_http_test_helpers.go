//go:build testhooks

package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"

	projection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	presence "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
)

func NewStudentStatusDayReadHTTPForTests(writer *presence.Resource) http.HandlerFunc {
	return projection.NewResource(projection.ResourceConfig{
		PeopleDirectory: writer.PeopleDirectory, StudentStatusDayService: writer.StudentStatusDayService,
		UserContextService: writer.UserContextService, SettingsService: writer.SettingsService,
		Logger: writer.Logger, Now: writer.Now,
	}).StatusDayReadHandlerForTests()
}

func NewStudentHTTPTestStatusDays(repo presenceservice.StudentStatusDayRepository) studentpresence.StatusDays {
	return presenceservice.NewStatusDays(repo, nil, nil, nil)
}
