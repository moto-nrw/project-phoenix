package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

type schoolNameQuery struct {
	lookup     func(context.Context, int64) (string, error)
	principals ports.Principals
}

func NewSchoolName(lookup func(context.Context, int64) (string, error), principals ports.Principals) devicescan.SchoolNameQuery {
	return schoolNameQuery{lookup: lookup, principals: principals}
}

func (q schoolNameQuery) DeviceSchoolName(ctx context.Context) (string, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return "", devicescan.ErrDeviceUnauthorized
	}
	return q.lookup(ctx, device.TenantID)
}
