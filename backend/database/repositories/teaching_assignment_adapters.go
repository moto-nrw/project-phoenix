package repositories

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

type classTeacherRepository struct{ membership schoolmembership.Capability }

var _ ClassTeacherRepository = (*classTeacherRepository)(nil)

func newClassTeacherRepository(membership schoolmembership.Capability) ClassTeacherRepository {
	return &classTeacherRepository{membership: membership}
}

func (r *classTeacherRepository) Create(ctx context.Context, assignment *schoolmembership.ClassAssignment) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := validateClassAssignment(assignment); err != nil {
		return err
	}
	created, err := r.membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass})
	if err != nil {
		return fmt.Errorf("create class assignment: %w", err)
	}
	copyClassAssignment(assignment, created)
	return nil
}

func (r *classTeacherRepository) FindByID(ctx context.Context, id any) (*schoolmembership.ClassAssignment, error) {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return nil, err
	}
	assignments, err := r.membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{IDs: []int64{assignmentID}})
	if err != nil {
		return nil, fmt.Errorf("find class assignment by ID: %w", err)
	}
	if len(assignments) == 0 {
		return nil, schoolmembership.ErrClassAssignmentNotFound
	}
	return classAssignmentModel(assignments[0]), nil
}

func (r *classTeacherRepository) Update(ctx context.Context, assignment *schoolmembership.ClassAssignment) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := validateClassAssignment(assignment); err != nil {
		return err
	}
	updated, err := r.membership.UpdateClassAssignment(ctx, schoolmembership.UpdateClassAssignment{ID: assignment.ID, StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass})
	if err != nil {
		return fmt.Errorf("update class assignment: %w", err)
	}
	copyClassAssignment(assignment, updated)
	return nil
}

func (r *classTeacherRepository) Delete(ctx context.Context, id any) error {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return err
	}
	if err := r.membership.DeleteClassAssignment(ctx, assignmentID); err != nil {
		return fmt.Errorf("delete class assignment: %w", err)
	}
	return nil
}

func (r *classTeacherRepository) List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error) {
	filter, err := classAssignmentFilter(filters)
	if err != nil {
		return nil, fmt.Errorf("list class assignments: %w", err)
	}
	assignments, err := r.membership.ListClassAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list class assignments: %w", err)
	}
	return classAssignmentModels(assignments), nil
}

func (r *classTeacherRepository) FindByStaff(ctx context.Context, staffID int64) ([]*schoolmembership.ClassAssignment, error) {
	assignments, err := r.membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, fmt.Errorf("find class assignments by staff: %w", err)
	}
	return classAssignmentModels(assignments), nil
}

type groupTeacherRepository struct {
	membership schoolmembership.Capability
	groups     EducationGroupRepository
}

var _ GroupTeacherRepository = (*groupTeacherRepository)(nil)

func newGroupTeacherRepository(membership schoolmembership.Capability, groups EducationGroupRepository) GroupTeacherRepository {
	return &groupTeacherRepository{membership: membership, groups: groups}
}

func (r *groupTeacherRepository) Create(ctx context.Context, assignment *schoolmembership.GroupAssignment) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := validateGroupAssignment(assignment); err != nil {
		return err
	}
	created, err := r.membership.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: assignment.GroupID, TeacherID: assignment.TeacherID})
	if err != nil {
		return fmt.Errorf("create group assignment: %w", err)
	}
	copyGroupAssignment(assignment, created)
	return nil
}

func (r *groupTeacherRepository) FindByID(ctx context.Context, id any) (*schoolmembership.GroupAssignment, error) {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return nil, err
	}
	assignments, err := r.membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{IDs: []int64{assignmentID}})
	if err != nil {
		return nil, fmt.Errorf("find group assignment by ID: %w", err)
	}
	if len(assignments) == 0 {
		return nil, schoolmembership.ErrGroupAssignmentNotFound
	}
	return groupAssignmentModel(assignments[0]), nil
}

func (r *groupTeacherRepository) Update(ctx context.Context, assignment *schoolmembership.GroupAssignment) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := validateGroupAssignment(assignment); err != nil {
		return err
	}
	updated, err := r.membership.UpdateGroupAssignment(ctx, schoolmembership.UpdateGroupAssignment{ID: assignment.ID, GroupID: assignment.GroupID, TeacherID: assignment.TeacherID})
	if err != nil {
		return fmt.Errorf("update group assignment: %w", err)
	}
	copyGroupAssignment(assignment, updated)
	return nil
}

func (r *groupTeacherRepository) Delete(ctx context.Context, id any) error {
	assignmentID, err := teachingAssignmentID(id)
	if err != nil {
		return err
	}
	if err := r.membership.DeleteGroupAssignment(ctx, assignmentID); err != nil {
		return fmt.Errorf("delete group assignment: %w", err)
	}
	return nil
}

func (r *groupTeacherRepository) List(ctx context.Context, filters map[string]any) ([]*schoolmembership.GroupAssignment, error) {
	filter, err := groupAssignmentFilter(filters)
	if err != nil {
		return nil, fmt.Errorf("list group assignments: %w", err)
	}
	assignments, err := r.membership.ListGroupAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list group assignments: %w", err)
	}
	return groupAssignmentModels(assignments), nil
}

func (r *groupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: []int64{groupID}}, "find by group")
}

func (r *groupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}}, "find by teacher")
}

func (r *groupTeacherRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*schoolmembership.GroupAssignment, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs}, "find by group IDs")
}

func (r *groupTeacherRepository) list(ctx context.Context, filter schoolmembership.GroupAssignmentFilter, operation string) ([]*schoolmembership.GroupAssignment, error) {
	assignments, err := r.membership.ListGroupAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return groupAssignmentModels(assignments), nil
}

func (r *groupTeacherRepository) ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error) {
	assigned, err := r.membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
	if err != nil {
		return nil, fmt.Errorf("list group teacher blockers: %w", err)
	}
	assigned = groupAssignmentsForTenant(assigned, tenantID)
	groupIDs := make([]int64, 0, len(assigned))
	for _, assignment := range assigned {
		groupIDs = append(groupIDs, assignment.GroupID)
	}
	all, err := r.membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, fmt.Errorf("list group teacher blockers: %w", err)
	}
	all = groupAssignmentsForTenant(all, tenantID)
	groups, err := r.groups.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("list group teacher blockers: %w", err)
	}
	teachersByGroup := make(map[int64][]int64, len(groupIDs))
	for _, assignment := range all {
		teachersByGroup[assignment.GroupID] = append(teachersByGroup[assignment.GroupID], assignment.TeacherID)
	}
	result := make([]userModels.BlockerGroup, 0, len(assigned))
	for _, assignment := range assigned {
		name := "Unbekannte Gruppe"
		if group := groups[assignment.GroupID]; group != nil {
			name = group.Name
		}
		result = append(result, userModels.BlockerGroup{
			ID: assignment.ID, GroupID: assignment.GroupID, GroupName: name,
			TeacherID: assignment.TeacherID, TeacherIDs: teachersByGroup[assignment.GroupID],
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupName < result[j].GroupName })
	return result, nil
}

func groupAssignmentsForTenant(assignments []schoolmembership.GroupAssignment, tenantID int64) []schoolmembership.GroupAssignment {
	result := make([]schoolmembership.GroupAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.TenantID == tenantID {
			result = append(result, assignment)
		}
	}
	return result
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

// validateClassAssignment requires the staff member and a non-blank class.
func validateClassAssignment(assignment *schoolmembership.ClassAssignment) error {
	if assignment.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if strings.TrimSpace(assignment.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	return nil
}

// validateGroupAssignment requires the group and the teacher.
func validateGroupAssignment(assignment *schoolmembership.GroupAssignment) error {
	if assignment.GroupID <= 0 {
		return errors.New("group ID is required")
	}
	if assignment.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}
	return nil
}

func teachingAssignmentID(id any) (int64, error) {
	return membershipID(id)
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
func (r *classTeacherRepository) AssignSchoolClass(ctx context.Context, staffID int64, schoolClass string) error {
	return r.Create(ctx, &schoolmembership.ClassAssignment{StaffID: staffID, SchoolClass: schoolClass})
}

// RenameSchoolClass rewrites the display form of one assignment.
func (r *classTeacherRepository) RenameSchoolClass(ctx context.Context, assignmentID, staffID int64, schoolClass string) error {
	return r.Update(ctx, &schoolmembership.ClassAssignment{ID: assignmentID, StaffID: staffID, SchoolClass: schoolClass})
}

// RemoveSchoolClass removes one assignment.
func (r *classTeacherRepository) RemoveSchoolClass(ctx context.Context, assignmentID int64) error {
	return r.Delete(ctx, assignmentID)
}

// SchoolClassAssignmentsOfStaff maps each assignment of the staff member to
// its class as entered.
func (r *classTeacherRepository) SchoolClassAssignmentsOfStaff(ctx context.Context, staffID int64) (map[int64]string, error) {
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
func (r *groupTeacherRepository) AssignTeacher(ctx context.Context, groupID, teacherID int64) error {
	return r.Create(ctx, &schoolmembership.GroupAssignment{GroupID: groupID, TeacherID: teacherID})
}

// RemoveTeacherAssignment removes one assignment.
func (r *groupTeacherRepository) RemoveTeacherAssignment(ctx context.Context, assignmentID int64) error {
	return r.Delete(ctx, assignmentID)
}

// TeacherAssignmentIDs maps each teacher of the group to its assignment.
func (r *groupTeacherRepository) TeacherAssignmentIDs(ctx context.Context, groupID int64) (map[int64]int64, error) {
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
func (r *groupTeacherRepository) TeacherIDsByGroup(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
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
