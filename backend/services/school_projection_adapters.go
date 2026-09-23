package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

type activeSchoolQuery struct {
	schools organizationtenancy.Query
}

func (q activeSchoolQuery) ListSchoolsByID(ctx context.Context, ids []int64) ([]presenceservice.School, error) {
	schools, err := q.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]presenceservice.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, presenceservice.School{ID: school.ID, Slug: school.Slug})
	}
	return result, nil
}

func newActiveSchoolQuery(schools organizationtenancy.Query) presenceservice.SchoolQuery {
	return activeSchoolQuery{schools: schools}
}
