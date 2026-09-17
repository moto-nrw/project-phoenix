package enrollment_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// factorySchools reads the seeded schools through the repository factory's
// Organisation & Tenancy capability, the owner the serving root binds.
// Callers pass a factory built on the pool from testpkg.SetupTestDB.
type factorySchools struct{ repos *repositories.Factory }

func (s factorySchools) FindSchool(ctx context.Context, id int64) (*enrollmentService.School, error) {
	school, err := s.repos.School.FindSchool(ctx, id)
	if err != nil {
		return nil, err
	}
	return &enrollmentService.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}
