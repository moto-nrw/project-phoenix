package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// schoolFinder is the Organisation & Tenancy read of one school.
type schoolFinder interface {
	FindSchool(ctx context.Context, id int64) (organizationtenancy.School, error)
}

// schoolLister is the Organisation & Tenancy read of schools by ID.
type schoolLister interface {
	ListSchoolsByID(ctx context.Context, ids []int64) ([]organizationtenancy.School, error)
}

// LookupSchool reads one school through the owner query and reports a missing
// row as found=false. Every other failure is returned unchanged.
func LookupSchool(ctx context.Context, schools schoolFinder, id int64) (organizationtenancy.School, bool, error) {
	school, err := schools.FindSchool(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return organizationtenancy.School{}, false, nil
	}
	if err != nil {
		return organizationtenancy.School{}, false, err
	}
	return school, true, nil
}

// schoolsByID reads the schools for ids in one query, keyed by school ID.
// Duplicate IDs are collapsed first; a school the owner does not return is
// absent.
func schoolsByID(ctx context.Context, query schoolLister, ids []int64) (map[int64]organizationtenancy.School, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	values, err := query.ListSchoolsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]organizationtenancy.School, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}
