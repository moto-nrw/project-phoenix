package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	careContractHTTP "github.com/moto-nrw/project-phoenix/modules/carecontract/http"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// mountCareContractNoticeRoutes serves withdrawal and termination (#4682) at
// /traeger/contract-notices behind the right "Verträge verwalten".
func mountCareContractNoticeRoutes(root chi.Router, organizations HolidayOrganizations, settings TraegerBrandingSettings,
	schoolSettings CareContractSchoolSettings, delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, accessLog owners.DataAccessAppender,
	addressees carecontract.NoticeAddressees, parentsURL string) {
	contractPorts, _ := careContractPorts(organizations, settings, schoolSettings, delivery, outcomes, accessLog, parentsURL)
	notices := careContractCompose.NewNotices(contractPorts, owners.NewCareContractNoticePorts(settings, delivery, addressees))
	resource := careContractHTTP.NewNoticeResource(notices, common.RequiresPermission(identityaccess.TraegerRightContractsManage))
	root.Mount("/traeger/contract-notices", resource.Router(ProtectedTraegerRoutes))
}
