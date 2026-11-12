package compose

import (
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/application"
	delivery "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
)

// NewRetention binds the school's nightly cleanup (#3955) to its tenant transaction.
func NewRetention() deliveryContract.Retention {
	return application.NewRetention(postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase))
}

// NewHolidayRegistrationMailErasure needs only the caller's admin transaction.
func NewHolidayRegistrationMailErasure() delivery.HolidayRegistrationMailErasure {
	return postgres.New(postgres.RequireTenantDatabase, postgres.RequireAdminDatabase)
}
