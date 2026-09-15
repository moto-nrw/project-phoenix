package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

type activeSchoolQuery struct {
	schools organizationtenancy.Query
}

func (q activeSchoolQuery) ListSchoolsByID(ctx context.Context, ids []int64) ([]activeService.School, error) {
	schools, err := q.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]activeService.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, activeService.School{ID: school.ID, Slug: school.Slug})
	}
	return result, nil
}

func newActiveSchoolQuery(schools organizationtenancy.Query) activeService.SchoolQuery {
	return activeSchoolQuery{schools: schools}
}
