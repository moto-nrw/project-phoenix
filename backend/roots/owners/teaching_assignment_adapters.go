package owners

import (
	"context"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

type ClassTeacherRepositoryBinding struct{ Membership schoolmembership.Capability }

func (r *ClassTeacherRepositoryBinding) Create(ctx context.Context, assignment *schoolmembership.ClassAssignment) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := schoolmembership.ValidateClassAssignment(assignment.StaffID, assignment.SchoolClass); err != nil {
		return err
	}
	created, err := r.Membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass})
	if err != nil {
		return fmt.Errorf("create class assignment: %w", err)
	}
	copyClassAssignment(assignment, created)
	return nil
}

func (r *ClassTeacherRepositoryBinding) FindByID(ctx context.Context, id any) (*schoolmembership.ClassAssignment, error) {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return nil, err
	}
	assignments, err := r.Membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{IDs: []int64{assignmentID}})
	if err != nil {
		return nil, fmt.Errorf("find class assignment by ID: %w", err)
	}
	if len(assignments) == 0 {
		return nil, schoolmembership.ErrClassAssignmentNotFound
	}
	return classAssignmentModel(assignments[0]), nil
}

func (r *ClassTeacherRepositoryBinding) Update(ctx context.Context, assignment *schoolmembership.ClassAssignment) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := schoolmembership.ValidateClassAssignment(assignment.StaffID, assignment.SchoolClass); err != nil {
		return err
	}
	updated, err := r.Membership.UpdateClassAssignment(ctx, schoolmembership.UpdateClassAssignment{ID: assignment.ID, StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass})
	if err != nil {
		return fmt.Errorf("update class assignment: %w", err)
	}
	copyClassAssignment(assignment, updated)
	return nil
}

func (r *ClassTeacherRepositoryBinding) Delete(ctx context.Context, id any) error {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return err
	}
	if err := r.Membership.DeleteClassAssignment(ctx, assignmentID); err != nil {
		return fmt.Errorf("delete class assignment: %w", err)
	}
	return nil
}

func (r *ClassTeacherRepositoryBinding) List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error) {
	filter, err := classAssignmentFilter(filters)
	if err != nil {
		return nil, fmt.Errorf("list class assignments: %w", err)
	}
	assignments, err := r.Membership.ListClassAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list class assignments: %w", err)
	}
	return classAssignmentModels(assignments), nil
}

func (r *ClassTeacherRepositoryBinding) FindByStaff(ctx context.Context, staffID int64) ([]*schoolmembership.ClassAssignment, error) {
	assignments, err := r.Membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, fmt.Errorf("find class assignments by staff: %w", err)
	}
	return classAssignmentModels(assignments), nil
}

type GroupTeacherRepositoryBinding struct {
	Membership schoolmembership.Capability
	Groups     GroupsByID
}

// groupsByID resolves the groups the teacher blockers name.
type GroupsByID interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
}

func ComposeGroupTeacherRepository(membership schoolmembership.Capability, groups GroupsByID) *GroupTeacherRepositoryBinding {
	return &GroupTeacherRepositoryBinding{Membership: membership, Groups: groups}
}

func (r *GroupTeacherRepositoryBinding) Create(ctx context.Context, assignment *schoolmembership.GroupAssignment) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := schoolmembership.ValidateGroupAssignment(assignment.GroupID, assignment.TeacherID); err != nil {
		return err
	}
	created, err := r.Membership.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: assignment.GroupID, TeacherID: assignment.TeacherID})
	if err != nil {
		return fmt.Errorf("create group assignment: %w", err)
	}
	copyGroupAssignment(assignment, created)
	return nil
}

func (r *GroupTeacherRepositoryBinding) FindByID(ctx context.Context, id any) (*schoolmembership.GroupAssignment, error) {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return nil, err
	}
	assignments, err := r.Membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{IDs: []int64{assignmentID}})
	if err != nil {
		return nil, fmt.Errorf("find group assignment by ID: %w", err)
	}
	if len(assignments) == 0 {
		return nil, schoolmembership.ErrGroupAssignmentNotFound
	}
	return groupAssignmentModel(assignments[0]), nil
}

func (r *GroupTeacherRepositoryBinding) Update(ctx context.Context, assignment *schoolmembership.GroupAssignment) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := schoolmembership.ValidateGroupAssignment(assignment.GroupID, assignment.TeacherID); err != nil {
		return err
	}
	updated, err := r.Membership.UpdateGroupAssignment(ctx, schoolmembership.UpdateGroupAssignment{ID: assignment.ID, GroupID: assignment.GroupID, TeacherID: assignment.TeacherID})
	if err != nil {
		return fmt.Errorf("update group assignment: %w", err)
	}
	copyGroupAssignment(assignment, updated)
	return nil
}

func (r *GroupTeacherRepositoryBinding) Delete(ctx context.Context, id any) error {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return err
	}
	if err := r.Membership.DeleteGroupAssignment(ctx, assignmentID); err != nil {
		return fmt.Errorf("delete group assignment: %w", err)
	}
	return nil
}

func (r *GroupTeacherRepositoryBinding) List(ctx context.Context, filters map[string]any) ([]*schoolmembership.GroupAssignment, error) {
	filter, err := groupAssignmentFilter(filters)
	if err != nil {
		return nil, fmt.Errorf("list group assignments: %w", err)
	}
	assignments, err := r.Membership.ListGroupAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list group assignments: %w", err)
	}
	return groupAssignmentModels(assignments), nil
}

func (r *GroupTeacherRepositoryBinding) FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: []int64{groupID}}, "find by group")
}

func (r *GroupTeacherRepositoryBinding) FindByTeacher(ctx context.Context, teacherID int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}}, "find by teacher")
}

func (r *GroupTeacherRepositoryBinding) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs}, "find by group IDs")
}

func (r *GroupTeacherRepositoryBinding) list(ctx context.Context, filter schoolmembership.GroupAssignmentFilter, operation string) ([]*schoolmembership.GroupAssignment, error) {
	assignments, err := r.Membership.ListGroupAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return groupAssignmentModels(assignments), nil
}

func (r *GroupTeacherRepositoryBinding) ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]peopleValues.BlockerGroup, error) {
	blockers, err := schoolmembership.GroupTeacherBlockers(ctx, r.Membership, r.groupNames, teacherID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list group teacher blockers: %w", err)
	}
	result := make([]peopleValues.BlockerGroup, 0, len(blockers))
	for _, blocker := range blockers {
		result = append(result, peopleValues.BlockerGroup{
			ID: blocker.AssignmentID, GroupID: blocker.GroupID, GroupName: blocker.GroupName,
			TeacherID: blocker.TeacherID, TeacherIDs: blocker.TeacherIDs,
		})
	}
	return result, nil
}

// groupNames names the School Structure groups the caller can see.
func (r *GroupTeacherRepositoryBinding) groupNames(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	groups, err := r.Groups.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(groups))
	for id, group := range groups {
		if group != nil {
			names[id] = group.Name
		}
	}
	return names, nil
}

func classAssignmentModels(assignments []schoolmembership.ClassAssignment) []*schoolmembership.ClassAssignment {
	result := make([]*schoolmembership.ClassAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, classAssignmentModel(assignment))
	}
	return result
}

func classAssignmentModel(assignment schoolmembership.ClassAssignment) *schoolmembership.ClassAssignment {
	result := &schoolmembership.ClassAssignment{}
	copyClassAssignment(result, assignment)
	return result
}

func copyClassAssignment(target *schoolmembership.ClassAssignment, source schoolmembership.ClassAssignment) {
	*target = source
}

func groupAssignmentModels(assignments []schoolmembership.GroupAssignment) []*schoolmembership.GroupAssignment {
	result := make([]*schoolmembership.GroupAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, groupAssignmentModel(assignment))
	}
	return result
}

func groupAssignmentModel(assignment schoolmembership.GroupAssignment) *schoolmembership.GroupAssignment {
	result := &schoolmembership.GroupAssignment{}
	copyGroupAssignment(result, assignment)
	return result
}

func copyGroupAssignment(target *schoolmembership.GroupAssignment, source schoolmembership.GroupAssignment) {
	*target = source
}

func teachingAssignmentID(id any) (int64, error) {
	return MembershipID(id)
}

func classAssignmentFilter(filters map[string]any) (schoolmembership.ClassAssignmentFilter, error) {
	var filter schoolmembership.ClassAssignmentFilter
	for field, value := range filters {
		if value == nil {
			continue
		}
		if field == "school_class" {
			schoolClass, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("invalid class assignment filter %q: expected string, got %T", field, value)
			}
			filter.ClassKeys = []string{schoolClass}
			continue
		}
		id, err := teachingAssignmentID(value)
		if err != nil {
			return filter, fmt.Errorf("invalid class assignment filter %q: %w", field, err)
		}
		switch field {
		case "id":
			filter.IDs = []int64{id}
		case "staff_id":
			filter.StaffIDs = []int64{id}
		default:
			return filter, fmt.Errorf("unsupported class assignment filter %q", field)
		}
	}
	return filter, nil
}

func groupAssignmentFilter(filters map[string]any) (schoolmembership.GroupAssignmentFilter, error) {
	var filter schoolmembership.GroupAssignmentFilter
	for field, value := range filters {
		if value == nil {
			continue
		}
		id, err := teachingAssignmentID(value)
		if err != nil {
			return filter, fmt.Errorf("invalid group assignment filter %q: %w", field, err)
		}
		switch field {
		case "id":
			filter.IDs = []int64{id}
		case "group_id":
			filter.GroupIDs = []int64{id}
		case "teacher_id":
			filter.TeacherIDs = []int64{id}
		default:
			return filter, fmt.Errorf("unsupported group assignment filter %q", field)
		}
	}
	return filter, nil
}

func nilEntity(entity string) error { return fmt.Errorf("%s cannot be nil or zero value", entity) }

// AssignSchoolClass assigns the staff member to the class as entered.
func (r *ClassTeacherRepositoryBinding) AssignSchoolClass(ctx context.Context, staffID int64, schoolClass string) error {
	return r.Create(ctx, &schoolmembership.ClassAssignment{StaffID: staffID, SchoolClass: schoolClass})
}

// RenameSchoolClass rewrites the display form of one assignment.
func (r *ClassTeacherRepositoryBinding) RenameSchoolClass(ctx context.Context, assignmentID, staffID int64, schoolClass string) error {
	return r.Update(ctx, &schoolmembership.ClassAssignment{ID: assignmentID, StaffID: staffID, SchoolClass: schoolClass})
}

// RemoveSchoolClass removes one assignment.
func (r *ClassTeacherRepositoryBinding) RemoveSchoolClass(ctx context.Context, assignmentID int64) error {
	return r.Delete(ctx, assignmentID)
}

// SchoolClassAssignmentsOfStaff maps each assignment of the staff member to
// its class as entered.
func (r *ClassTeacherRepositoryBinding) SchoolClassAssignmentsOfStaff(ctx context.Context, staffID int64) (map[int64]string, error) {
	assignments, err := r.FindByStaff(ctx, staffID)
	if err != nil {
		return nil, err
	}
	classes := make(map[int64]string, len(assignments))
	for _, assignment := range assignments {
		classes[assignment.ID] = assignment.SchoolClass
	}
	return classes, nil
}

// AssignTeacher assigns the teacher to the group.
func (r *GroupTeacherRepositoryBinding) AssignTeacher(ctx context.Context, groupID, teacherID int64) error {
	return r.Create(ctx, &schoolmembership.GroupAssignment{GroupID: groupID, TeacherID: teacherID})
}

// RemoveTeacherAssignment removes one assignment.
func (r *GroupTeacherRepositoryBinding) RemoveTeacherAssignment(ctx context.Context, assignmentID int64) error {
	return r.Delete(ctx, assignmentID)
}

// TeacherAssignmentIDs maps each teacher of the group to its assignment.
func (r *GroupTeacherRepositoryBinding) TeacherAssignmentIDs(ctx context.Context, groupID int64) (map[int64]int64, error) {
	assignments, err := r.FindByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	ids := make(map[int64]int64, len(assignments))
	for _, assignment := range assignments {
		ids[assignment.TeacherID] = assignment.ID
	}
	return ids, nil
}

// TeacherIDsByGroup lists the teachers of each group in assignment order.
func (r *GroupTeacherRepositoryBinding) TeacherIDsByGroup(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
	assignments, err := r.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	teachers := make(map[int64][]int64, len(groupIDs))
	for _, assignment := range assignments {
		teachers[assignment.GroupID] = append(teachers[assignment.GroupID], assignment.TeacherID)
	}
	return teachers, nil
}
