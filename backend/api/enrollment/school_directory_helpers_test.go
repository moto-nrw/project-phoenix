package enrollment_test

import (
	"context"
	"errors"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// capabilitySchools reads the seeded schools through the Organisation &
// Tenancy capability, the owner the serving root binds.
type capabilitySchools struct{ schools organizationtenancy.Query }

func (s capabilitySchools) FindSchool(ctx context.Context, id int64) (*enrollmentOwner.School, error) {
	school, err := s.schools.FindSchool(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &enrollmentOwner.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}
