package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

func NewStudentTagReleaser(people peopledirectory.Capability) applicationBindings.StudentTagReleaser {
	return applicationBindings.StudentTagReleaser{StudentTagRelease: people}
}
