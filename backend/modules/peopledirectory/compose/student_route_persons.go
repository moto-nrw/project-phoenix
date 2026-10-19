package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func NewStudentRoutePersons(source peoplePorts.PersonDirectory) application.StudentRoutePersons {
	return application.NewStudentRoutePersons(source)
}
