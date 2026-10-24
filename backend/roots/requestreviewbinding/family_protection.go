package requestreviewbinding

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// NewFamilyProtection keeps this decoration optional at the composition edge:
// a nil query leaves the review queues undecorated.
func NewFamilyProtection(query peopledirectory.FamilyProtectionQuery) requestreview.FamilyProtection {
	if query == nil {
		return nil
	}
	return query
}
