package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewFamilyChildrenOffice builds the Abrechnung's check of the children of the families (#5918) over the contracts the root binds.
func NewFamilyChildrenOffice(contracts parentbilling.CareContracts) parentbilling.FamilyChildrenOffice {
	return application.NewFamilyChildren(postgres.NewStore(), contracts, calendar.TodayDate)
}

// NewParentFamilyChildren builds the parents' statement of the children of their family (#5918) in the Träger transaction the caller opens.
func NewParentFamilyChildren() parentbilling.ParentFamilyChildrenStatement {
	return application.NewParentFamilyChildren(postgres.NewStore())
}
