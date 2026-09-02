package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// staffLookup is the slice of the School Membership query the projections
// below read: who a staff ID is, and which teacher profile belongs to it.
type staffLookup interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error)
}

// lazyStaffLookup reads the factory's capability at call time, so a later
// BindSchoolMembership swap (the observed module) reaches the projections
// that were wired at construction.
type lazyStaffLookup struct {
	get func() schoolmembership.Capability
}

func (l lazyStaffLookup) ListStaff(ctx context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return l.get().ListStaff(ctx, filter)
}

func (l lazyStaffLookup) ListTeachers(ctx context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	return l.get().ListTeachers(ctx, filter)
}

// bindStaffProjections wraps every legacy repository that used to read
// users.staff or users.teachers through a foreign join (#2667). The wrapped
// repositories keep their interfaces and project only the IDs they own; the
// staff and teacher facts are resolved through the owner query afterwards.
//
// Order matters: this runs BEFORE bindPersonProjections, so the person layer
// sits outside and still finds the Staff rows it attaches Staff.Person to.
func (f *Factory) bindStaffProjections(membership staffLookup) {
	if membership == nil {
		panic("repository factory: school membership query is required")
	}
	if f.GroupSupervisor != nil {
		f.GroupSupervisor = staffGroupSupervisorRepository{GroupSupervisorRepository: f.GroupSupervisor, membership: membership}
	}
	if f.StaffAbsence != nil {
		f.StaffAbsence = newStaffAbsenceRepository(f.StaffAbsence, membership)
	}
	if f.ActivityGroup != nil {
		f.ActivityGroup = newStaffActivityGroupRepository(f.ActivityGroup, membership)
	}
	if f.ActivitySupervisor != nil {
		f.ActivitySupervisor = staffSupervisorPlannedRepository{SupervisorPlannedRepository: f.ActivitySupervisor, membership: membership}
	}
	// The three repositories below keep their own method signatures — those
	// speak calendar dates and query options, which this package must not
	// import — and take the owner lookup as an injected function instead.
	if setter, ok := f.Group.(supervisionStaffResolverSetter); ok {
		setter.SetSupervisionStaffResolver(supervisionStaffResolver(membership))
	}
	if setter, ok := f.GroupSubstitution.(substitutionStaffResolverSetter); ok {
		setter.SetSubstitutionStaffResolver(substitutionStaffResolver(membership))
	}
	if setter, ok := f.Room.(supervisorPersonsResolverSetter); ok {
		setter.SetSupervisorPersonsResolver(supervisorPersonsResolver(membership))
	}
	if f.AccountTenant != nil {
		f.AccountTenant = newStaffAccountTenantRepository(f.AccountTenant, membership)
	}
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

// The owner-to-legacy staff projection (toLegacyStaff) lives in
// staff_membership_adapters.go; Staff.Person stays nil there and is attached
// by the person layer above through the People Directory.
