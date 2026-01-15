// Package students provides HTTP handlers for student-related operations.
package students

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	userContextService "github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
)

// containsIgnoreCase checks if a string contains another string, ignoring case.
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// hasAdminPermissions checks if user has admin permissions.
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// canModifyStudent centralizes the authorization logic for modifying student data (update/delete).
func canModifyStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService, operation string) (bool, error) {
	// Admin users have full access
	if hasAdminPermissions(userPermissions) {
		return true, nil
	}

	// Student must have a group for non-admin operations
	if student.GroupID == nil {
		return false, fmt.Errorf("only administrators can %s students without assigned groups", operation)
	}

	// Check if user is a staff member
	staff, err := userContextService.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("insufficient permissions to %s this student's data", operation)
	}

	// Check if staff supervises the student's group
	if isGroupSupervisor(ctx, *student.GroupID, userContextService) {
		return true, nil
	}

	return false, fmt.Errorf("you can only %s students in groups you supervise", operation)
}

// canUpdateStudent is a convenience wrapper for update operations.
func canUpdateStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService) (bool, error) {
	return canModifyStudent(ctx, userPermissions, student, userContextService, "update")
}

// canDeleteStudent is a convenience wrapper for delete operations.
func canDeleteStudent(ctx context.Context, userPermissions []string, student *users.Student, userContextService userContextService.UserContextService) (bool, error) {
	return canModifyStudent(ctx, userPermissions, student, userContextService, "delete")
}

// isGroupSupervisor checks if the current user supervises a specific group.
func isGroupSupervisor(ctx context.Context, groupID int64, userContextService userContextService.UserContextService) bool {
	// Check education groups
	educationGroups, err := userContextService.GetMyGroups(ctx)
	if err == nil {
		for _, g := range educationGroups {
			if g.ID == groupID {
				return true
			}
		}
	}

	// Also check active groups
	activeGroups, err := userContextService.GetMyActiveGroups(ctx)
	if err == nil {
		for _, ag := range activeGroups {
			if ag.GroupID == groupID {
				return true
			}
		}
	}

	return false
}
