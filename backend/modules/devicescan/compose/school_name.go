package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

func NewSchoolName(lookup func(context.Context, int64) (string, error), principal devicescan.DevicePrincipal) devicescan.SchoolNameQuery {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	return application.NewSchoolName(lookup, principal)
}
