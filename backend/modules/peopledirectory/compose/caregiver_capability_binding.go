package compose

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewCaregiverCapabilityService composes the tenant-scoped caregiver
// capability service; its disable path runs on a Bun transaction.
func NewCaregiverCapabilityService(deps peoplePorts.CaregiverCapabilityServiceDependencies) peoplePorts.CaregiverCapabilityService {
	return application.NewCaregiverCapabilityService(deps, requireBunTransaction)
}

func requireBunTransaction(ctx context.Context) error {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return fmt.Errorf("disable caregiver capability: unit of work did not provide a transaction")
	}
	if _, ok := raw.(bun.Tx); !ok {
		return fmt.Errorf("disable caregiver capability: unsupported transaction type %T", raw)
	}
	return nil
}
