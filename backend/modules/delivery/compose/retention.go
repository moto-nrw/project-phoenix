package compose

import (
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
)

// NewRetention composes the delivery retention of the nightly cleanup
// (#3955). It runs on the tenant transaction of the school in context and
// needs neither a provider nor the people directory.
func NewRetention() deliveryContract.Retention {
	store := postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
	return application.NewRetention(store)
}
