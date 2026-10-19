package compose

import (
	postgresNative "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// NewCaregiverCapabilityService composes the tenant-scoped caregiver
// capability service; its disable path runs on a Bun transaction.
func NewCaregiverCapabilityService(deps peoplePorts.CaregiverCapabilityServiceDependencies) peoplePorts.CaregiverCapabilityService {
	return application.NewCaregiverCapabilityService(deps, postgresNative.RequireBunTransaction)
}
