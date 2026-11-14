package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	careContractHTTP "github.com/moto-ogs/project-phoenix/modules/carecontract/http"
	carePlanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

// mountCareContractImportRoutes serves the import of children and their
// existing care contracts from a file (#4723) at /traeger/contract-imports
// behind the right "Verträge verwalten".
func mountCareContractImportRoutes(root chi.Router, organizations HolidayOrganizations, settings TraegerBrandingSettings,
	accessLog owners.DataAccessAppender) {
	resource := careContractHTTP.NewImportResource(NewCareContractImports(organizations, settings, accessLog),
		common.RequiresPermission(identityaccess.TraegerRightContractsManage))
	root.Mount("/traeger/contract-imports", resource.Router(ProtectedTraegerRoutes))
}

// NewCareContractImports binds the import to the schools of the Träger, the
// offerings of each school with the names of their phases, the school's
// access log and the debtor numbers the Elternabrechnung keeps.
func NewCareContractImports(organizations HolidayOrganizations, settings TraegerBrandingSettings,
	accessLog owners.DataAccessAppender) carecontract.ContractImports {
	return careContractCompose.NewContractImports(careContractCompose.ImportPorts{
		Schools:      careContractSchools{organizations: organizations},
		Applications: careContractApplications{applications: enrollmentCompose.NewContractApplications(securityruntime.Fingerprint)},
		Bookings:     careContractBookings{offerings: carePlanCompose.NewContractOfferings(), bookings: carePlanCompose.NewOfferingBookings()},
		Access:       careContractAccessLog{log: accessLog}, Runtime: owners.CareContractRuntime{},
		Numbers:     owners.NewCareContractDebtorNumbers(parentBillingSettings{settings: settings}),
		Fingerprint: securityruntime.Fingerprint,
	})
}
