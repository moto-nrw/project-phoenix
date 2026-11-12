package owners

import (
	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	deliveryRuntime "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	schedulerPorts "github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
)

// CareContractConclusionBuilder builds the Worker's conclusion of care
// contracts at the acceptance (#4680) over the owners the standalone Worker
// composes. The server root supplies it, where the Betreuungsvertrag's
// ports are bound for its portal routes too. parentsURL is the base of the
// parents portal the signing links of those ports point to (#4679);
// holidays dates the withdrawal period at each school (#6350).
type CareContractConclusionBuilder func(organizations organizationtenancy.Capability, settings SettingsOwner,
	delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, accessLog DataAccessAppender,
	holidays carecontract.SchoolHolidays, parentsURL string) schedulerPorts.CareContractConclusion
