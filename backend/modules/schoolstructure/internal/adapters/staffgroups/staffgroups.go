// Package staffgroups answers the staff-keyed group reads of School
// Structure. The assignment and substitution rows stay with their owners
// (School Membership: education.group_teacher and education.class_teachers;
// Workforce: education.group_substitution); this adapter resolves group ids
// through their public capabilities and loads the groups through School
// Structure's own query, so each source applies its own tenant scoping on the
// caller's ambient transaction.
package staffgroups

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Groups is the School Structure read the staff reads resolve ids through.
type Groups interface {
	ListGroupsByID(context.Context, []int64) ([]schoolstructure.Group, error)
}

// TeachingAssignments is the School Membership read of teacher and class
// assignments.
type TeachingAssignments interface {
	ListGroupAssignments(context.Context, schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error)
	ListClassAssignments(context.Context, schoolmembership.ClassAssignmentFilter) ([]schoolmembership.ClassAssignment, error)
}

// GroupSubstitutions is the Workforce read of group substitutions.
type GroupSubstitutions interface {
	ListGroupSubstitutions(context.Context, workforce.GroupSubstitutionFilter) ([]workforce.GroupSubstitution, error)
}

type Reader struct {
	groups        Groups
	assignments   TeachingAssignments
	substitutions GroupSubstitutions
}

var _ schoolstructure.StaffGroupQuery = (*Reader)(nil)

func New(groups Groups, assignments TeachingAssignments, substitutions GroupSubstitutions) *Reader {
	if groups == nil || assignments == nil || substitutions == nil {
		panic("school structure staff groups: all dependencies are required")
	}
	return &Reader{groups: groups, assignments: assignments, substitutions: substitutions}
}

func (r *Reader) ListGroupsByTeacher(ctx context.Context, teacherID int64) ([]schoolstructure.Group, error) {
	if teacherID <= 0 {
		return nil, schoolstructure.ErrInvalidStaff
	}
	assignments, err := r.assignments.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.GroupID)
	}
	return r.groups.ListGroupsByID(ctx, ids)
}

func (r *Reader) ListSubstitutedGroups(ctx context.Context, staffID int64, day string) ([]schoolstructure.SubstitutedGroup, error) {
	if staffID <= 0 {
		return nil, schoolstructure.ErrInvalidStaff
	}
	on, err := calendar.ParseDate(day)
	if err != nil {
		return nil, schoolstructure.ErrInvalidDay
	}
	substitutions, err := r.substitutions.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{SubstituteStaffID: staffID, On: on.String()})
	if err != nil {
		return nil, err
	}
	// A group with several active substitutions is flagged when any of them
	// leaves the regular staff slot unassigned.
	unassigned := make(map[int64]bool, len(substitutions))
	ids := make([]int64, 0, len(substitutions))
	for _, substitution := range substitutions {
		if _, seen := unassigned[substitution.GroupID]; !seen {
			ids = append(ids, substitution.GroupID)
		}
		unassigned[substitution.GroupID] = unassigned[substitution.GroupID] || substitution.RegularStaffID == nil
	}
	groups, err := r.groups.ListGroupsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]schoolstructure.SubstitutedGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, schoolstructure.SubstitutedGroup{Group: group, ViaSubstitution: unassigned[group.ID]})
	}
	return result, nil
}

func (r *Reader) ListSchoolClassesByStaff(ctx context.Context, staffID int64) ([]string, error) {
	if staffID <= 0 {
		return nil, schoolstructure.ErrInvalidStaff
	}
	assignments, err := r.assignments.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, err
	}
	classes := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		classes = append(classes, assignment.SchoolClass)
	}
	return classes, nil
}
