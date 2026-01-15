package usercontext

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// UserGroupProvider and GroupAccessProvider implementation
// These methods retrieve groups associated with the current user and provide access control

// GetMyGroups retrieves educational groups associated with the current user
func (s *userContextService) GetMyGroups(ctx context.Context) ([]*education.Group, error) {
	staff, staffErr := s.GetCurrentStaff(ctx)
	teacher, teacherErr := s.GetCurrentTeacher(ctx)

	// Check for valid linkage and unexpected errors
	valid, unexpectedErr := s.hasValidStaffOrTeacher(staffErr, teacherErr)
	if unexpectedErr != nil {
		return nil, &UserContextError{Op: "get my groups", Err: unexpectedErr}
	}
	if !valid {
		return []*education.Group{}, nil
	}

	groupMap := make(map[int64]*education.Group)
	var partialErr *PartialError

	// Add teacher's groups
	if teacher != nil && teacherErr == nil {
		if err := s.addTeacherGroups(ctx, teacher.ID, groupMap); err != nil {
			return nil, err
		}
	}

	// Add substitution groups
	if staff != nil && staffErr == nil {
		partialErr = s.addSubstitutionGroups(ctx, staff.ID, groupMap)
	}

	groups := mapToSlice(groupMap)
	return s.handlePartialError(groups, partialErr)
}

// hasValidStaffOrTeacher checks if the user has valid staff or teacher linkage
// Returns: valid bool, unexpectedErr error
func (s *userContextService) hasValidStaffOrTeacher(staffErr, teacherErr error) (bool, error) {
	// If either lookup succeeded, user has valid linkage
	if staffErr == nil || teacherErr == nil {
		return true, nil
	}

	// Both lookups failed - check if any error is unexpected
	staffExpected := isExpectedLinkageError(staffErr)
	teacherExpected := isExpectedLinkageError(teacherErr)

	// If both errors are expected "not linked" errors, user is simply not linked
	if staffExpected && teacherExpected {
		return false, nil
	}

	// At least one unexpected error - return it for visibility
	if !teacherExpected {
		return false, teacherErr
	}
	return false, staffErr
}

// isExpectedLinkageError checks if an error is an expected "user not linked" error
func isExpectedLinkageError(err error) bool {
	return errors.Is(err, ErrUserNotLinkedToTeacher) ||
		errors.Is(err, ErrUserNotLinkedToStaff) ||
		errors.Is(err, ErrUserNotLinkedToPerson)
}

// addTeacherGroups adds groups where the teacher is assigned
func (s *userContextService) addTeacherGroups(ctx context.Context, teacherID int64, groupMap map[int64]*education.Group) error {
	teacherGroups, err := s.educationGroupRepo.FindByTeacher(ctx, teacherID)
	if err != nil {
		return &UserContextError{Op: "get my groups", Err: err}
	}
	for _, group := range teacherGroups {
		groupMap[group.ID] = group
	}
	return nil
}

// addSubstitutionGroups adds groups where the staff is an active substitute
func (s *userContextService) addSubstitutionGroups(ctx context.Context, staffID int64, groupMap map[int64]*education.Group) *PartialError {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	substitutions, err := s.substitutionRepo.FindActiveBySubstituteWithRelations(ctx, staffID, today)
	if err != nil {
		return &PartialError{Op: "get my groups (substitutions)", LastErr: err, FailureCount: 1}
	}

	var partialErr *PartialError
	for _, sub := range substitutions {
		group, err := s.resolveSubstitutionGroup(ctx, sub)
		if err != nil {
			partialErr = s.recordSubstitutionFailure(partialErr, sub.GroupID, err)
			continue
		}
		if group != nil {
			groupMap[group.ID] = group
			if partialErr != nil {
				partialErr.SuccessCount++
			}
		}
	}
	return partialErr
}

// resolveSubstitutionGroup gets the group from substitution, with fallback lookup
func (s *userContextService) resolveSubstitutionGroup(ctx context.Context, sub *education.GroupSubstitution) (*education.Group, error) {
	if sub.Group != nil {
		return sub.Group, nil
	}
	return s.educationGroupRepo.FindByID(ctx, sub.GroupID)
}

// recordSubstitutionFailure records a failure to load a substitution group
func (s *userContextService) recordSubstitutionFailure(partialErr *PartialError, groupID int64, err error) *PartialError {
	logger.Logger.WithFields(map[string]interface{}{
		"group_id": groupID,
		"error":    err,
	}).Warn("Failed to load group for substitution")

	if partialErr == nil {
		partialErr = &PartialError{
			Op:        "get my groups (load substitution groups)",
			FailedIDs: make([]int64, 0),
		}
	}
	partialErr.FailedIDs = append(partialErr.FailedIDs, groupID)
	partialErr.FailureCount++
	partialErr.LastErr = err
	return partialErr
}

// handlePartialError handles partial error reporting for GetMyGroups
func (s *userContextService) handlePartialError(groups []*education.Group, partialErr *PartialError) ([]*education.Group, error) {
	if partialErr == nil || partialErr.FailureCount == 0 {
		return groups, nil
	}

	logger.Logger.WithFields(map[string]interface{}{
		"success_count": partialErr.SuccessCount,
		"failure_count": partialErr.FailureCount,
		"failed_ids":    partialErr.FailedIDs,
		"operation":     partialErr.Op,
	}).Warn("Partial failure in GetMyGroups")

	if len(groups) > 0 {
		return groups, partialErr
	}
	return nil, partialErr
}

// mapToSlice converts a group map to a slice
func mapToSlice(groupMap map[int64]*education.Group) []*education.Group {
	groups := make([]*education.Group, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, group)
	}
	return groups
}

// GetMyActivityGroups retrieves activity groups associated with the current user
func (s *userContextService) GetMyActivityGroups(ctx context.Context) ([]*activities.Group, error) {
	// Try to get the current staff
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
			return nil, err
		}

		// User is not staff or not linked to person, return empty list (could expand to other user types later)
		return []*activities.Group{}, nil
	}

	// Get activity groups where the staff is a supervisor
	// First, get all planned supervisions for this staff member
	plannedSupervisions, err := s.activityGroupRepo.FindByStaffSupervisor(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get my activity groups", Err: err}
	}

	// If no groups found, return empty list
	if len(plannedSupervisions) == 0 {
		return []*activities.Group{}, nil
	}

	return plannedSupervisions, nil
}

// GetMyActiveGroups retrieves active groups associated with the current user
func (s *userContextService) GetMyActiveGroups(ctx context.Context) ([]*active.Group, error) {
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if isExpectedLinkageError(err) {
			return []*active.Group{}, nil
		}
		return nil, err
	}

	// Get active groups from activity groups
	activeGroups, err := s.getActiveGroupsFromActivities(ctx, staff.ID)
	if err != nil {
		return nil, err
	}

	// Add supervised groups
	supervisedGroups, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - supervised", Err: err}
	}

	return mergeActiveGroups(activeGroups, supervisedGroups), nil
}

// getActiveGroupsFromActivities gets active groups for staff's activity supervisions
func (s *userContextService) getActiveGroupsFromActivities(ctx context.Context, staffID int64) ([]*active.Group, error) {
	activityGroups, err := s.activityGroupRepo.FindByStaffSupervisor(ctx, staffID)
	if err != nil {
		return nil, &UserContextError{Op: "get my active groups - activity groups", Err: err}
	}

	var result []*active.Group
	for _, group := range activityGroups {
		activeGroups, err := s.activeGroupRepo.FindActiveByGroupID(ctx, group.ID)
		if err != nil {
			return nil, &UserContextError{Op: "get my active groups - activity active", Err: err}
		}
		result = append(result, activeGroups...)
	}
	return result, nil
}

// mergeActiveGroups combines two slices of active groups, removing duplicates
func mergeActiveGroups(primary, additional []*active.Group) []*active.Group {
	groupMap := make(map[int64]*active.Group, len(primary)+len(additional))
	for _, group := range primary {
		groupMap[group.ID] = group
	}
	for _, group := range additional {
		if _, exists := groupMap[group.ID]; !exists {
			groupMap[group.ID] = group
		}
	}
	result := make([]*active.Group, 0, len(groupMap))
	for _, group := range groupMap {
		result = append(result, group)
	}
	return result
}

// GetMySupervisedGroups retrieves active groups supervised by the current user
func (s *userContextService) GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error) {
	// Get current staff member
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
			return nil, err
		}
		// User is not staff, return empty list
		return []*active.Group{}, nil
	}

	// Find active supervisions for this staff member
	supervisions, err := s.supervisorRepo.FindActiveByStaffID(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: "get supervised groups", Err: err}
	}

	if len(supervisions) == 0 {
		return []*active.Group{}, nil
	}

	// Collect group IDs for batch loading (more efficient than individual FindByID calls)
	groupIDs := make([]int64, 0, len(supervisions))
	for _, supervision := range supervisions {
		// Check if supervision itself is still active (not ended)
		if !supervision.IsActive() {
			// Log for observability; helps diagnose silent filters of ended supervisions
			logger.Logger.WithFields(map[string]interface{}{
				"supervision_id": supervision.ID,
				"group_id":       supervision.GroupID,
				"staff_id":       staff.ID,
			}).Debug("Skipping ended supervision")
			continue // Skip ended supervisions
		}
		groupIDs = append(groupIDs, supervision.GroupID)
	}

	if len(groupIDs) == 0 {
		return []*active.Group{}, nil
	}

	// Batch load all groups with their rooms (FindByIDs loads Room relation)
	groupsMap, err := s.activeGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, &UserContextError{Op: "get supervised groups", Err: err}
	}

	// Convert map to slice, filtering only active groups
	var supervisedGroups []*active.Group
	for _, groupID := range groupIDs {
		if group, ok := groupsMap[groupID]; ok && group.IsActive() {
			supervisedGroups = append(supervisedGroups, group)
		}
	}

	return supervisedGroups, nil
}

// GetActiveSubstitutionGroupIDs returns group IDs where the staff member is an active substitute
// (excludes substitutions where they're replacing a regular staff member)
func (s *userContextService) GetActiveSubstitutionGroupIDs(ctx context.Context, staffID int64) (map[int64]bool, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	activeSubs, err := s.substitutionRepo.FindActiveBySubstitute(ctx, staffID, today)
	if err != nil {
		return nil, &UserContextError{Op: "get active substitution group IDs", Err: err}
	}

	groupIDs := make(map[int64]bool)
	for _, sub := range activeSubs {
		groupIDs[sub.GroupID] = true
	}

	return groupIDs, nil
}

// checkGroupAccess verifies the current user has access to a specific group.
// Returns nil if access is granted, or an error if the group doesn't exist or user lacks access.
func (s *userContextService) checkGroupAccess(ctx context.Context, groupID int64) error {
	group, err := s.activeGroupRepo.FindByID(ctx, groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}

	userGroups, err := s.GetMyActiveGroups(ctx)
	if err != nil {
		return err
	}

	for _, g := range userGroups {
		if g.ID == groupID {
			return nil
		}
	}

	supervisedGroups, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		return err
	}
	for _, g := range supervisedGroups {
		if g.ID == groupID {
			return nil
		}
	}

	return ErrUserNotAuthorized
}

// GetGroupStudents retrieves students in a specific group where the current user has access
func (s *userContextService) GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	if err := s.checkGroupAccess(ctx, groupID); err != nil {
		return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
	}

	// Get all visits for this group
	visits, err := s.visitsRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
	}

	// Create a map to deduplicate student IDs
	studentIDs := make(map[int64]bool)
	for _, visit := range visits {
		studentIDs[visit.StudentID] = true
	}

	// Convert map keys to slice
	ids := make([]int64, 0, len(studentIDs))
	for id := range studentIDs {
		ids = append(ids, id)
	}

	// If no students found, return empty slice
	if len(ids) == 0 {
		return []*users.Student{}, nil
	}

	// Get students by IDs
	var students []*users.Student
	for _, id := range ids {
		student, err := s.studentRepo.FindByID(ctx, id)
		if err != nil {
			return nil, &UserContextError{Op: opGetGroupStudents, Err: err}
		}
		if student != nil {
			students = append(students, student)
		}
	}

	return students, nil
}

// GetGroupVisits retrieves active visits for a specific group where the current user has access
func (s *userContextService) GetGroupVisits(ctx context.Context, groupID int64) ([]*active.Visit, error) {
	if err := s.checkGroupAccess(ctx, groupID); err != nil {
		return nil, &UserContextError{Op: "get group visits", Err: err}
	}

	// Get active visits for this group
	visits, err := s.visitsRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return nil, &UserContextError{Op: "get group visits", Err: err}
	}

	// Filter to only include active visits (no end time)
	var activeVisits []*active.Visit
	for _, visit := range visits {
		if visit.ExitTime == nil {
			activeVisits = append(activeVisits, visit)
		}
	}

	return activeVisits, nil
}
