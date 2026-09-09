package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

// ConfigurationSource loads one consistent settings snapshot for a tenant.
type ConfigurationSource interface {
	LoadConfiguration(context.Context, int64) (devicescan.Configuration, error)
}

type configurationQuery struct {
	source     ConfigurationSource
	principals ports.Principals
}

func NewConfiguration(source ConfigurationSource, principals ports.Principals) devicescan.ConfigurationQuery {
	return configurationQuery{source: source, principals: principals}
}

func (q configurationQuery) DeviceConfiguration(ctx context.Context) (devicescan.Configuration, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return devicescan.Configuration{}, devicescan.ErrDeviceUnauthorized
	}
	result, err := q.source.LoadConfiguration(ctx, device.TenantID)
	if err != nil {
		return devicescan.Configuration{}, devicescan.Internal("failed to resolve device configuration", err)
	}
	return result, nil
}
