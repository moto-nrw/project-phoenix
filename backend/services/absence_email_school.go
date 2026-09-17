package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type absenceEmailSchoolDirectory struct {
	schools organizationtenancy.Query
}

func (d absenceEmailSchoolDirectory) FindSchoolSubdomain(ctx context.Context, id int64) (string, bool, error) {
	school, found, err := findSchool(ctx, d.schools, id)
	return school.Subdomain, found, err
}
