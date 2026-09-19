package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// NewFamilyProtection keeps this decoration optional at the composition edge.
func NewFamilyProtection(query peopledirectory.FamilyProtectionQuery) requestreview.FamilyProtection {
	if query == nil {
		return nil
	}
	return familyProtection{query: query}
}

type familyProtection struct {
	query peopledirectory.FamilyProtectionQuery
}

func (f familyProtection) Protected(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return f.query.CurrentFamilyProtection(ctx, ids)
}
