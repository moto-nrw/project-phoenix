package operator

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// SchoolLookup reads one school for the operator settings routes. The
// Organisation & Tenancy capability satisfies it.
type SchoolLookup interface {
	FindSchool(ctx context.Context, id int64) (organizationtenancy.School, error)
}
