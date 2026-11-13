package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	careContractHTTP "github.com/moto-ogs/project-phoenix/modules/carecontract/http"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

// mountCareContractChangeRoutes serves the Träger-Büro's decision of the
// parents' changes of the booked care (#5979) at /traeger/contract-changes
// behind the right "Verträge verwalten". The decision is Care Plan's
// offering-change review of the school, run in the contract school's tenant
// transaction; the contract follows the booked care with the Worker's
// amendment like after a decision of the school.
func mountCareContractChangeRoutes(root chi.Router, organizations HolidayOrganizations, changes careplan.OfferingChangeCapability,
	schoolSettings owners.CareContractChangeSettings, accessLog owners.DataAccessAppender) {
	resource := careContractHTTP.NewChangeResource(NewCareContractChanges(organizations, changes, schoolSettings, accessLog),
		common.RequiresPermission(identityaccess.TraegerRightContractsManage))
	root.Mount("/traeger/contract-changes", resource.Router(ProtectedTraegerRoutes))
}

// NewCareContractChanges binds the office's decision to the schools of the
// Träger, Care Plan's offering-change review and the school's access log.
func NewCareContractChanges(organizations HolidayOrganizations, changes careplan.OfferingChangeCapability,
	schoolSettings owners.CareContractChangeSettings, accessLog owners.DataAccessAppender) carecontract.ContractChangeDecisions {
	if changes == nil || schoolSettings == nil {
		panic("care contract changes: the offering changes and the school settings are required")
	}
	return careContractCompose.NewContractChanges(carecontract.ChangePorts{
		Schools:  careContractSchools{organizations: organizations},
		Requests: owners.CareContractChangeRequests{Changes: changes, Settings: schoolSettings},
		Access:   careContractAccessLog{log: accessLog}, Runtime: owners.CareContractRuntime{},
	})
}
