package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

// NewChildQuotaLimits composes the tenant-safe read of the Kinderkontingent (#3567) School Membership checks its
// counting writes against, on the caller's tenant transaction for the tenant in context only.
func NewChildQuotaLimits() organizationtenancy.ChildQuotaLimits {
	return retainedChildQuotaLimits{}
}
