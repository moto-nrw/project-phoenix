package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
)

// NewAccountRetention composes the narrow offboarding read on the caller's transaction runtime, without an unscoped pool.
func NewAccountRetention(observe func(Observation)) organizationtenancy.AccountRetention {
	return application.New(postgres.New(postgres.NewDatabase()), postgres.Transaction{}, observe)
}

// NewTraegerMemberNames composes the names of the Träger memberships (#4693) on the caller's transaction runtime.
func NewTraegerMemberNames(observe func(Observation)) organizationtenancy.TraegerMemberNames {
	return application.New(postgres.New(postgres.NewDatabase()), postgres.Transaction{}, observe)
}
