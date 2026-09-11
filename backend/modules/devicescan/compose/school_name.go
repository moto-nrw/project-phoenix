package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

func NewSchoolName(lookup func(context.Context, int64) (string, error)) devicescan.SchoolNameQuery {
	return application.NewSchoolName(lookup, principals{})
}
