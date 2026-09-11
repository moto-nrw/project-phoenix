package schoolmembership

import (
	"context"
	"errors"
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
