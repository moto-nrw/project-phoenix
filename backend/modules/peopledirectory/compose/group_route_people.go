package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-ogs/project-phoenix/modules/peopledirectory/ports"
)

func NewGroupRoutePeople(source peoplePorts.PersonDirectory) application.GroupRoutePeople {
	return application.NewGroupRoutePeople(source)
}
