package schoolmembership

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrClassAssignmentNotFound = errors.New("class assignment not found")
	ErrGroupAssignmentNotFound = errors.New("group assignment not found")
	ErrClassAssignmentConflict = errors.New("class assignment already exists")
	ErrGroupAssignmentConflict = errors.New("group assignment already exists")
)

// ClassAssignment assigns a staff member to one free-text school class.
type ClassAssignment struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StaffID     int64     `json:"staff_id"`
	SchoolClass string    `json:"school_class"`
}

type CreateClassAssignment struct {
	StaffID     int64
	SchoolClass string
}

type UpdateClassAssignment struct {
	ID          int64
	StaffID     int64
	SchoolClass string
}

type ClassAssignmentFilter struct {
	IDs       []int64
	StaffIDs  []int64
	ClassKeys []string
}

// GroupAssignment assigns a teacher profile to an education group.
type GroupAssignment struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	GroupID   int64     `json:"group_id"`
	TeacherID int64     `json:"teacher_id"`
}

type CreateGroupAssignment struct {
	GroupID   int64
	TeacherID int64
}

type UpdateGroupAssignment struct {
	ID        int64
	GroupID   int64
	TeacherID int64
}

type GroupAssignmentFilter struct {
	IDs             []int64
	GroupIDs        []int64
	TeacherIDs      []int64
	TeacherStaffIDs []int64
}

func (m *Module) ListClassAssignments(ctx context.Context, filter ClassAssignmentFilter) ([]ClassAssignment, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StaffIDs = uniquePositive(filter.StaffIDs)
	for index := range filter.ClassKeys {
		filter.ClassKeys[index] = strings.ToLower(strings.TrimSpace(filter.ClassKeys[index]))
	}
	return m.teachingAssignments.ListClassAssignments(ctx, filter)
}

func (m *Module) CreateClassAssignment(ctx context.Context, input CreateClassAssignment) (ClassAssignment, error) {
	if err := validateClassAssignment(input.StaffID, &input.SchoolClass); err != nil {
		return ClassAssignment{}, err
	}
	return m.teachingAssignments.CreateClassAssignment(ctx, input)
}

func (m *Module) UpdateClassAssignment(ctx context.Context, input UpdateClassAssignment) (ClassAssignment, error) {
	if input.ID <= 0 {
		return ClassAssignment{}, invalid("class assignment ID is required")
	}
	if err := validateClassAssignment(input.StaffID, &input.SchoolClass); err != nil {
		return ClassAssignment{}, err
	}
	return m.teachingAssignments.UpdateClassAssignment(ctx, input)
}

func (m *Module) DeleteClassAssignment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("class assignment ID is required")
	}
	return m.teachingAssignments.DeleteClassAssignment(ctx, id)
}

func (m *Module) DeleteClassAssignmentsByStaff(ctx context.Context, staffID int64) (int64, error) {
	if staffID <= 0 {
		return 0, invalid("staff ID is required")
	}
	return m.teachingAssignments.DeleteClassAssignmentsByStaff(ctx, staffID)
}

func (m *Module) ListGroupAssignments(ctx context.Context, filter GroupAssignmentFilter) ([]GroupAssignment, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.GroupIDs = uniquePositive(filter.GroupIDs)
	filter.TeacherIDs = uniquePositive(filter.TeacherIDs)
	filter.TeacherStaffIDs = uniquePositive(filter.TeacherStaffIDs)
	return m.teachingAssignments.ListGroupAssignments(ctx, filter)
}

func (m *Module) CreateGroupAssignment(ctx context.Context, input CreateGroupAssignment) (GroupAssignment, error) {
	if err := validateGroupAssignment(input.GroupID, input.TeacherID); err != nil {
		return GroupAssignment{}, err
	}
	return m.teachingAssignments.CreateGroupAssignment(ctx, input)
}

func (m *Module) UpdateGroupAssignment(ctx context.Context, input UpdateGroupAssignment) (GroupAssignment, error) {
	if input.ID <= 0 {
		return GroupAssignment{}, invalid("group assignment ID is required")
	}
	if err := validateGroupAssignment(input.GroupID, input.TeacherID); err != nil {
		return GroupAssignment{}, err
	}
	return m.teachingAssignments.UpdateGroupAssignment(ctx, input)
}

func (m *Module) DeleteGroupAssignment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("group assignment ID is required")
	}
	return m.teachingAssignments.DeleteGroupAssignment(ctx, id)
}

func (m *Module) DeleteGroupAssignmentsByTeacher(ctx context.Context, teacherID int64) (int64, error) {
	if teacherID <= 0 {
		return 0, invalid("teacher ID is required")
	}
	return m.teachingAssignments.DeleteGroupAssignmentsByTeacher(ctx, teacherID)
}

func (m *Module) LockGroupAssignments(ctx context.Context) error {
	return m.teachingAssignments.LockGroupAssignments(ctx)
}

func validateClassAssignment(staffID int64, schoolClass *string) error {
	*schoolClass = strings.TrimSpace(*schoolClass)
	if staffID <= 0 {
		return invalid("staff ID is required")
	}
	if *schoolClass == "" {
		return invalid("school class is required")
	}
	return nil
}

func validateGroupAssignment(groupID, teacherID int64) error {
	if groupID <= 0 {
		return invalid("group ID is required")
	}
	if teacherID <= 0 {
		return invalid("teacher ID is required")
	}
	return nil
}

// ValidateClassAssignment requires the staff member and a non-blank class.
func ValidateClassAssignment(staffID int64, schoolClass string) error {
	if staffID <= 0 {
		return errors.New("staff ID is required")
	}
	if strings.TrimSpace(schoolClass) == "" {
		return errors.New("school class is required")
	}
	return nil
}

// ValidateGroupAssignment requires the group and the teacher.
func ValidateGroupAssignment(groupID, teacherID int64) error {
	if groupID <= 0 {
		return errors.New("group ID is required")
	}
	if teacherID <= 0 {
		return errors.New("teacher ID is required")
	}
	return nil
}

// GroupTeacherBlocker is one group assignment that keeps a teacher from
// losing the caregiver role: the group, its name and every teacher of it.
type GroupTeacherBlocker struct {
	AssignmentID int64
	GroupID      int64
	GroupName    string
	TeacherID    int64
	TeacherIDs   []int64
}

// GroupAssignmentLister lists group assignments.
type GroupAssignmentLister interface {
	ListGroupAssignments(context.Context, GroupAssignmentFilter) ([]GroupAssignment, error)
}

// GroupNames names the groups the caller can see; an unseen group is absent.
type GroupNames func(ctx context.Context, groupIDs []int64) (map[int64]string, error)

// unknownGroupName labels a blocking group the caller cannot see.
const unknownGroupName = "Unbekannte Gruppe"

// GroupTeacherBlockers lists the teacher's group assignments of the tenant,
// each with every teacher of its group, ordered by group name.
func GroupTeacherBlockers(ctx context.Context, assignments GroupAssignmentLister, names GroupNames, teacherID, tenantID int64) ([]GroupTeacherBlocker, error) {
	assigned, err := assignments.ListGroupAssignments(ctx, GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
	if err != nil {
		return nil, err
	}
	assigned = groupAssignmentsOfTenant(assigned, tenantID)
	groupIDs := make([]int64, 0, len(assigned))
	for _, assignment := range assigned {
		groupIDs = append(groupIDs, assignment.GroupID)
	}
	all, err := assignments.ListGroupAssignments(ctx, GroupAssignmentFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, err
	}
	all = groupAssignmentsOfTenant(all, tenantID)
	groupNames, err := names(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	teachersByGroup := make(map[int64][]int64, len(groupIDs))
	for _, assignment := range all {
		teachersByGroup[assignment.GroupID] = append(teachersByGroup[assignment.GroupID], assignment.TeacherID)
	}
	result := make([]GroupTeacherBlocker, 0, len(assigned))
	for _, assignment := range assigned {
		name, found := groupNames[assignment.GroupID]
		if !found {
			name = unknownGroupName
		}
		result = append(result, GroupTeacherBlocker{
			AssignmentID: assignment.ID, GroupID: assignment.GroupID, GroupName: name,
			TeacherID: assignment.TeacherID, TeacherIDs: teachersByGroup[assignment.GroupID],
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupName < result[j].GroupName })
	return result, nil
}

func groupAssignmentsOfTenant(assignments []GroupAssignment, tenantID int64) []GroupAssignment {
	result := make([]GroupAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment.TenantID == tenantID {
			result = append(result, assignment)
		}
	}
	return result
}
