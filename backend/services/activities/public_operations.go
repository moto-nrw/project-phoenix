package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
)

// ======== Public Methods ========

// GetPublicGroups retrieves groups for public display, optionally filtered by category
func (s *Service) GetPublicGroups(ctx context.Context, categoryID *int64) ([]*activities.Group, map[int64]int, error) {
	var groups []*activities.Group
	var err error

	// Retrieve groups by category or all public groups
	if categoryID != nil {
		groups, err = s.groupRepo.FindByCategory(ctx, *categoryID)
	} else {
		groups, err = s.groupRepo.FindOpenGroups(ctx)
	}

	if err != nil {
		return nil, nil, &ActivityError{Op: "get public groups", Err: err}
	}

	// Get enrollment counts
	counts := make(map[int64]int)
	for _, group := range groups {
		count, err := s.enrollmentRepo.CountByGroupID(ctx, group.ID)
		if err != nil {
			return nil, nil, &ActivityError{Op: "get enrollments", Err: err}
		}
		counts[group.ID] = count
	}

	return groups, counts, nil
}

// GetPublicCategories retrieves categories for public display
func (s *Service) GetPublicCategories(ctx context.Context) ([]*activities.Category, error) {
	categories, err := s.categoryRepo.ListAll(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get public categories", Err: err}
	}

	return categories, nil
}

// GetOpenGroups retrieves activity groups that are open for enrollment
func (s *Service) GetOpenGroups(ctx context.Context) ([]*activities.Group, error) {
	// This is likely a wrapper around a repository method
	groups, err := s.groupRepo.FindOpenGroups(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get open groups", Err: err}
	}

	return groups, nil
}

// GetTeacherTodaysActivities returns activities available for teacher selection on devices
func (s *Service) GetTeacherTodaysActivities(ctx context.Context, staffID int64) ([]*activities.Group, error) {
	groups, err := s.groupRepo.FindByStaffSupervisorToday(ctx, staffID)
	if err != nil {
		return nil, &ActivityError{Op: "get teacher today activities", Err: err}
	}
	return groups, nil
}
