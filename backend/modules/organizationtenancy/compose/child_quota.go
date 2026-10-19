package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// NewChildQuotaLimits composes the tenant-safe read of the Kinderkontingent
// (#3567) that School Membership checks its counting writes against. It reads
// on the caller's tenant transaction and answers only for the tenant in
// context; it needs nothing else of the owner.
func NewChildQuotaLimits() organizationtenancy.ChildQuotaLimits {
	return retainedChildQuotaLimits{}
}
