package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func NewGroupRoutePeople(source peoplePorts.PersonDirectory) application.GroupRoutePeople {
	return application.NewGroupRoutePeople(source)
}
