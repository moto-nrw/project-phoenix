package services

import (
	"context"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

type absenceEmailSchoolDirectory struct {
	schools platformModels.SchoolRepository
}

func (d absenceEmailSchoolDirectory) FindSchoolSubdomain(ctx context.Context, id int64) (string, bool, error) {
	school, err := d.schools.FindByID(ctx, id)
	if err != nil {
		return "", false, err
	}
	if school == nil {
		return "", false, nil
	}
	return school.Subdomain, true, nil
}
