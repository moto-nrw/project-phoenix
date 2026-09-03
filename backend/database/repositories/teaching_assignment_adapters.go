package repositories

import (
	"context"
	"fmt"
	"sort"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

type classTeacherRepository struct{ membership schoolmembership.Capability }

var _ educationModels.ClassTeacherRepository = (*classTeacherRepository)(nil)

func newClassTeacherRepository(membership schoolmembership.Capability) educationModels.ClassTeacherRepository {
	return &classTeacherRepository{membership: membership}
}

func (r *classTeacherRepository) Create(ctx context.Context, assignment *educationModels.ClassTeacher) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := assignment.Validate(); err != nil {
		return err
	}
	created, err := r.membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass})
	if err != nil {
		return fmt.Errorf("create class assignment: %w", err)
	}
	copyClassAssignment(assignment, created)
	return nil
}

func (r *classTeacherRepository) FindByID(ctx context.Context, id any) (*educationModels.ClassTeacher, error) {
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

func (r *classTeacherRepository) Update(ctx context.Context, assignment *educationModels.ClassTeacher) error {
	if assignment == nil {
		return nilEntity("ClassTeacher")
	}
	if err := assignment.Validate(); err != nil {
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

func (r *classTeacherRepository) List(ctx context.Context, filters map[string]any) ([]*educationModels.ClassTeacher, error) {
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

func (r *classTeacherRepository) FindByStaff(ctx context.Context, staffID int64) ([]*educationModels.ClassTeacher, error) {
	assignments, err := r.membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, fmt.Errorf("find class assignments by staff: %w", err)
	}
	return classAssignmentModels(assignments), nil
}

func (r *classTeacherRepository) DeleteByStaffID(ctx context.Context, staffID int64) (int64, error) {
	rows, err := r.membership.DeleteClassAssignmentsByStaff(ctx, staffID)
	if err != nil {
		return 0, fmt.Errorf("delete class assignments by staff ID: %w", err)
	}
	return rows, nil
}

type groupTeacherRepository struct {
	membership schoolmembership.Capability
	groups     educationModels.GroupRepository
}

var _ educationModels.GroupTeacherRepository = (*groupTeacherRepository)(nil)

func newGroupTeacherRepository(membership schoolmembership.Capability, groups educationModels.GroupRepository) educationModels.GroupTeacherRepository {
	return &groupTeacherRepository{membership: membership, groups: groups}
}

func (r *groupTeacherRepository) Create(ctx context.Context, assignment *educationModels.GroupTeacher) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := assignment.Validate(); err != nil {
		return err
	}
	created, err := r.membership.CreateGroupAssignment(ctx, schoolmembership.CreateGroupAssignment{GroupID: assignment.GroupID, TeacherID: assignment.TeacherID})
	if err != nil {
		return fmt.Errorf("create group assignment: %w", err)
	}
	copyGroupAssignment(assignment, created)
	return nil
}

func (r *groupTeacherRepository) FindByID(ctx context.Context, id any) (*educationModels.GroupTeacher, error) {
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

func (r *groupTeacherRepository) Update(ctx context.Context, assignment *educationModels.GroupTeacher) error {
	if assignment == nil {
		return nilEntity("GroupTeacher")
	}
	if err := assignment.Validate(); err != nil {
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

func (r *groupTeacherRepository) List(ctx context.Context, filters map[string]any) ([]*educationModels.GroupTeacher, error) {
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

func (r *groupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupTeacher, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: []int64{groupID}}, "find by group")
}

func (r *groupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.GroupTeacher, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}}, "find by teacher")
}

func (r *groupTeacherRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*educationModels.GroupTeacher, error) {
	return r.list(ctx, schoolmembership.GroupAssignmentFilter{GroupIDs: groupIDs}, "find by group IDs")
}

func (r *groupTeacherRepository) list(ctx context.Context, filter schoolmembership.GroupAssignmentFilter, operation string) ([]*educationModels.GroupTeacher, error) {
	assignments, err := r.membership.ListGroupAssignments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return groupAssignmentModels(assignments), nil
}

func (r *groupTeacherRepository) DeleteByTeacherID(ctx context.Context, teacherID int64) (int64, error) {
	rows, err := r.membership.DeleteGroupAssignmentsByTeacher(ctx, teacherID)
	if err != nil {
		return 0, fmt.Errorf("delete group assignments by teacher ID: %w", err)
	}
	return rows, nil
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

func classAssignmentModels(assignments []schoolmembership.ClassAssignment) []*educationModels.ClassTeacher {
	result := make([]*educationModels.ClassTeacher, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, classAssignmentModel(assignment))
	}
	return result
}

func classAssignmentModel(assignment schoolmembership.ClassAssignment) *educationModels.ClassTeacher {
	result := &educationModels.ClassTeacher{}
	copyClassAssignment(result, assignment)
	return result
}

func copyClassAssignment(target *educationModels.ClassTeacher, source schoolmembership.ClassAssignment) {
	target.ID, target.TenantID = source.ID, source.TenantID
	target.CreatedAt, target.UpdatedAt = source.CreatedAt, source.UpdatedAt
	target.StaffID, target.SchoolClass = source.StaffID, source.SchoolClass
}

func groupAssignmentModels(assignments []schoolmembership.GroupAssignment) []*educationModels.GroupTeacher {
	result := make([]*educationModels.GroupTeacher, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, groupAssignmentModel(assignment))
	}
	return result
}

func groupAssignmentModel(assignment schoolmembership.GroupAssignment) *educationModels.GroupTeacher {
	result := &educationModels.GroupTeacher{}
	copyGroupAssignment(result, assignment)
	return result
}

func copyGroupAssignment(target *educationModels.GroupTeacher, source schoolmembership.GroupAssignment) {
	target.ID, target.TenantID = source.ID, source.TenantID
	target.CreatedAt, target.UpdatedAt = source.CreatedAt, source.UpdatedAt
	target.GroupID, target.TeacherID = source.GroupID, source.TeacherID
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
