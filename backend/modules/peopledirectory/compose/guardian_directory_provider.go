package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// guardianProviders validates and constructs the optional guardian directory.
func guardianProviders(provider *GuardianDirectoryProvider) ([]peopledirectory.GuardianProvider, error) {
	if provider == nil {
		return nil, nil
	}
	if provider.Guardians == nil || provider.Runtime == nil || provider.PermissionGranted == nil {
		return nil, errors.New("people directory compose: the guardian provider needs the guardian service, the tenant transactions and the permission rule")
	}
	return []peopledirectory.GuardianProvider{application.NewGuardianDirectory(provider.Guardians, provider.Runtime, provider.PermissionGranted)}, nil
}

// GuardianDirectoryProvider is what the root composes the guardian provider
// from: the legacy guardian service, the tenant transactions and the Security
// Runtime's stored-value rule of a parent-portal permission (#2663).
type GuardianDirectoryProvider struct {
	Guardians         peoplePorts.GuardianService
	Runtime           peoplePorts.TenantTxRunner
	PermissionGranted func(value any) bool
}
