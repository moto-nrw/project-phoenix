package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

type ConfigurationQuery = devicescan.ConfigurationQuery

// ConfigurationLoader loads one tenant's device configuration; the root
// binds it to the Settings Platform.
type ConfigurationLoader interface {
	LoadConfiguration(ctx context.Context, tenantID int64) (devicescan.Configuration, error)
}

func NewConfiguration(loader ConfigurationLoader, principal devicescan.DevicePrincipal) ConfigurationQuery {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	return application.NewConfiguration(loader, principal)
}
