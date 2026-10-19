package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func NewStudentDirectory(directory application.StudentDirectoryCapability) peoplePorts.StudentDirectoryAccess {
	return application.NewStudentDirectory(directory)
}
