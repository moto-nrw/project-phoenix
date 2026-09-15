package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewSchoolName binds the invitation label to the current school's owner query.
func NewSchoolName(schools organizationtenancy.Capability) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return "", err
		}
		school, err := schools.FindSchool(ctx, id.Int64())
		if err != nil {
			return "", err
		}
		return school.Name, nil
	}
}
