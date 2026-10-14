package services

import (
	"context"

	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type absenceEmailSchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d absenceEmailSchoolDirectory) FindSchoolSubdomain(ctx context.Context, id int64) (string, bool, error) {
	school, found, err := organizationTenancyCompose.FindSchool(ctx, d.schools, id)
	return school.Subdomain, found, err
}
