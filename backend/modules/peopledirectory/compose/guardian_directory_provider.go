package compose

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner.

// GuardianProviderBinder is what the People Directory module exposes for the
// late binding of its guardian provider.
type GuardianProviderBinder interface {
	BindGuardianProvider(peopledirectory.GuardianProvider)
}

// BindGuardianDirectory installs the guardian provider on the People
// Directory. A capability without the binder cannot serve guardians at all,
// so the composition refuses to continue silently.
func BindGuardianDirectory(persons peopledirectory.Capability, provider *GuardianDirectoryProvider) {
	binder, ok := persons.(GuardianProviderBinder)
	if !ok {
		panic(fmt.Sprintf("guardian directory: people directory %T cannot bind the guardian provider", persons))
	}
	binder.BindGuardianProvider(application.NewGuardianDirectory(provider.Guardians, provider.Runtime, provider.PermissionGranted))
}

// newGuardianDirectory is the guardian provider Dependencies.GuardianProvider
// installs at construction; every part of it is required.
func newGuardianDirectory(provider *GuardianDirectoryProvider) (peopledirectory.GuardianProvider, error) {
	if provider.Guardians == nil || provider.Runtime == nil || provider.PermissionGranted == nil {
		return nil, errors.New("people directory compose: the guardian provider needs the guardian service, the tenant transactions and the permission rule")
	}
	return application.NewGuardianDirectory(provider.Guardians, provider.Runtime, provider.PermissionGranted), nil
}

// GuardianDirectoryProvider is what the root binds the guardian provider
// from: the legacy guardian service, the tenant transactions and the Security
// Runtime's stored-value rule of a parent-portal permission (#2663).
type GuardianDirectoryProvider struct {
	Guardians         peoplePorts.GuardianService
	Runtime           peoplePorts.TenantTxRunner
	PermissionGranted func(value any) bool
}
