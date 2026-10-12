package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// staffLookup is the slice of the School Membership query the projections
// below read: who a staff ID is, and which teacher profile belongs to it.
type staffLookup interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error)
}

// bindStaffProjections wraps every legacy repository that used to read
// users.staff or users.teachers through a foreign join (#2667). The wrapped
// repositories keep their interfaces and project only the IDs they own; the
// staff and teacher facts are resolved through the owner query afterwards.
//
// Order matters: this runs BEFORE bindPersonProjections, so the person layer
// sits outside and still finds the Staff rows it attaches Staff.Person to.
func (f *Factory) bindStaffProjections(membership staffLookup, workTime workforce.Capability) {
	if membership == nil {
		panic("repository factory: school membership query is required")
	}
	if staff := f.supervisionStaffOf(); staff != nil {
		staff.membership = membership
	}
	if f.StaffAbsence != nil {
		f.StaffAbsence = newStaffAbsenceRepository(f.StaffAbsence, membership)
	}
	// The group substitution adapter receives its staff resolver at
	// construction.
}

// staffByID resolves the staff members for ids through the owner query,
// keyed by staff ID. includeDeleted mirrors whether the replaced SQL join
// filtered soft-deleted rows.
func staffByID(ctx context.Context, query staffLookup, ids []int64, includeDeleted bool) (map[int64]schoolmembership.Staff, error) {
	unique := uniqueIDs(ids)
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

func uniqueIDs(ids []int64) []int64 {
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
	return unique
}

// The owner-to-legacy staff projection (toLegacyStaff) lives with the staff
// adapters in services; Staff.Person stays nil there and is attached by the
// person layer above through the People Directory.
