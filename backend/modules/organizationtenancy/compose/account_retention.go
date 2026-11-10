package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
)

// NewAccountRetention composes the narrow offboarding read on the caller's
// transaction runtime. Persistence has no fallback to an unscoped pool.
func NewAccountRetention(observe func(Observation)) organizationtenancy.AccountRetention {
	return application.New(postgres.New(postgres.NewDatabase()), postgres.Transaction{}, observe)
}
