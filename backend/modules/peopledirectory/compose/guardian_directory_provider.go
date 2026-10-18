package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner.

// newModule wraps the engine and installs the guardian provider
// Dependencies.GuardianProvider names at construction; every part of a
// named provider is required.
func newModule(engine application.Engine, provider *GuardianDirectoryProvider) (*peopledirectory.Module, error) {
	if provider == nil {
		return peopledirectory.NewModule(engine), nil
	}
	if provider.Guardians == nil || provider.Runtime == nil || provider.PermissionGranted == nil {
		return nil, errors.New("people directory compose: the guardian provider needs the guardian service, the tenant transactions and the permission rule")
	}
	return peopledirectory.NewModule(engine, application.NewGuardianDirectory(provider.Guardians, provider.Runtime, provider.PermissionGranted)), nil
}

// GuardianDirectoryProvider is what the root composes the guardian provider
// from: the legacy guardian service, the tenant transactions and the Security
// Runtime's stored-value rule of a parent-portal permission (#2663).
type GuardianDirectoryProvider struct {
	Guardians         peoplePorts.GuardianService
	Runtime           peoplePorts.TenantTxRunner
	PermissionGranted func(value any) bool
}
