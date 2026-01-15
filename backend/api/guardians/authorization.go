package guardians

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// hasAdminPermissions checks if user has admin permissions
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// canModifyStudent checks if the current user can modify a student's guardians
func (rs *Resource) canModifyStudent(ctx context.Context, studentID int64) (bool, error) {
	userPermissions := jwt.PermissionsFromCtx(ctx)

	// Admin users have full access
	if hasAdminPermissions(userPermissions) {
		return true, nil
	}

	// Get the student
	student, err := rs.StudentService.Get(ctx, studentID)
	if err != nil {
		return false, fmt.Errorf("student not found")
	}

	// Student must have a group for non-admin operations
	if student.GroupID == nil {
		return false, fmt.Errorf("only administrators can modify guardians for students without assigned groups")
	}

	// Check if user is a staff member who supervises the student's group
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("insufficient permissions to modify this student's guardians")
	}

	// Check if staff supervises the student's group
	educationGroups, err := rs.UserContextService.GetMyGroups(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get supervised groups")
	}

	for _, group := range educationGroups {
		if group.ID == *student.GroupID {
			return true, nil
		}
	}

	return false, fmt.Errorf("you can only modify guardians for students in groups you supervise")
}

// canModifyGuardian checks if the current user can modify a guardian profile
// User can modify if they are admin OR if they supervise at least one student linked to this guardian
func (rs *Resource) canModifyGuardian(ctx context.Context, guardianID int64) (bool, error) {
	userPermissions := jwt.PermissionsFromCtx(ctx)

	// Admin users have full access
	if hasAdminPermissions(userPermissions) {
		return true, nil
	}

	// Check if user is a staff member
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("only staff members can modify guardian profiles")
	}

	// Get students linked to this guardian
	studentsWithRel, err := rs.GuardianService.GetGuardianStudents(ctx, guardianID)
	if err != nil {
		return false, fmt.Errorf("failed to get guardian's students")
	}

	// If guardian has no linked students, only admins can modify
	if len(studentsWithRel) == 0 {
		return false, fmt.Errorf("only administrators can modify guardians with no linked students")
	}

	// Check if staff supervises at least one of the guardian's students
	educationGroups, err := rs.UserContextService.GetMyGroups(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get supervised groups")
	}

	// Build map of supervised group IDs for efficient lookup
	supervisedGroupIDs := make(map[int64]bool)
	for _, group := range educationGroups {
		supervisedGroupIDs[group.ID] = true
	}

	// Check if any of the guardian's students are in supervised groups
	for _, studentRel := range studentsWithRel {
		// Get full student details to check their group
		student, err := rs.StudentService.Get(ctx, studentRel.Student.ID)
		if err != nil {
			continue
		}

		// Check if this student's group is supervised by current user
		if student.GroupID != nil && supervisedGroupIDs[*student.GroupID] {
			return true, nil
		}
	}

	return false, fmt.Errorf("you can only modify guardians for students in groups you supervise")
}
