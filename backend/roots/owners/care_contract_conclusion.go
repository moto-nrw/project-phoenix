package owners

import (
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// CareContractConclusionBuilder builds the Worker's conclusion of care
// contracts at the acceptance (#4680) over the owners the standalone Worker
// composes. The server root supplies it, where the Betreuungsvertrag's
// ports are bound for its portal routes too.
type CareContractConclusionBuilder func(organizations organizationtenancy.Capability, settings SettingsOwner,
	delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, accessLog DataAccessAppender) schedulerPorts.CareContractConclusion
