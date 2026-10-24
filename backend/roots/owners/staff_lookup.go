package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// StaffLookup is the slice of the School Membership query the projections
// read: who a staff ID is, and which teacher profile belongs to it.
type StaffLookup interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error)
}

// StaffByID resolves the staff members for ids through the owner query,
// keyed by staff ID. Duplicates and non-positive IDs are dropped before the
// lookup. includeDeleted mirrors whether the replaced SQL join filtered
// soft-deleted rows.
func StaffByID(ctx context.Context, query StaffLookup, ids []int64, includeDeleted bool) (map[int64]schoolmembership.Staff, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return map[int64]schoolmembership.Staff{}, nil
	}
	values, err := query.ListStaff(ctx, schoolmembership.StaffFilter{IDs: unique, IncludeDeleted: includeDeleted})
	if err != nil {
		return nil, fmt.Errorf("load staff: %w", err)
	}
	result := make(map[int64]schoolmembership.Staff, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}
