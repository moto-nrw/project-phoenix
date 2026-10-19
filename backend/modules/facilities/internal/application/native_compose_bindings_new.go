package application

import (
	context "context"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

type EngineBinding struct{ Engine }

func (EngineBinding) RequireTenant(ctx context.Context) error {
	_, err := tenant.TenantFromContext(ctx)
	return err
}
