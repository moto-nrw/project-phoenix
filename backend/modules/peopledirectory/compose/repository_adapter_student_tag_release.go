package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

func NewStudentTagReleaser(people peopledirectory.Capability) StudentTagReleaser {
	return StudentTagReleaser{StudentTagReleaser: applicationBindings.StudentTagReleaser{StudentTagRelease: people}}
}

type StudentTagReleaser struct {
	applicationBindings.StudentTagReleaser
}
