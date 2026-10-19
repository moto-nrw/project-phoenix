package application

import (
	"context"

	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// bindTenantRuntime is the runtime the module opens its transactions under:
// Dependencies.TenantRuntime when set, otherwise the sessions' own runtime,
// otherwise the context's. The sessions attach through it.
func BindTenantRuntime(sessions *identityBindingPorts.SessionDependencies, unit tenant.UnitOfWork) (*identityBindingPorts.SessionDependencies, func(context.Context) context.Context) {
	attach := func(ctx context.Context) context.Context { return ctx }
	if sessions != nil && sessions.TenantRuntime != nil {
		attach = sessions.TenantRuntime
	}
	if !unit.IsZero() {
		attach = func(ctx context.Context) context.Context { return tenant.WithUnitOfWork(ctx, unit) }
	}
	if sessions != nil {
		copy := *sessions
		copy.TenantRuntime = attach
		sessions = &copy
	}
	return sessions, attach
}
