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
