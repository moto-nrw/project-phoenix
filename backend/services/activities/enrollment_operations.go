package activities

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ======== Enrollment Methods ========

// EnrollStudent enrolls a student in an activity group
func (s *Service) EnrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Execute in transaction to ensure consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if group exists
		_, err := txService.(*Service).groupRepo.FindByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrGroupNotFound
			}
			return &ActivityError{Op: opFindGroup, Err: err}
		}

		// Check if student is already enrolled
		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "check existing enrollment", Err: err}
		}

		// Check if student is already enrolled
		for _, enrollment := range enrollments {
			if enrollment.StudentID == studentID {
				return ErrStudentAlreadyEnrolled
			}
		}

		// Create enrollment
		enrollment := &activities.StudentEnrollment{
			StudentID:       studentID,
			ActivityGroupID: groupID,
			EnrollmentDate:  time.Now(),
		}

		if err := txService.(*Service).enrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ActivityError{Op: "create enrollment", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	return nil
}

// UnenrollStudent removes a student from an activity group
func (s *Service) UnenrollStudent(ctx context.Context, groupID, studentID int64) error {
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		if err := s.validateGroupExists(ctx, txService, groupID); err != nil {
			return err
		}

		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "find enrollments", Err: err}
		}

		enrollmentID, err := s.findEnrollmentID(enrollments, studentID)
		if err != nil {
			return err
		}

		if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
			return &ActivityError{Op: "delete enrollment", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	return nil
}

// findEnrollmentID finds the enrollment ID for a specific student in a group
func (s *Service) findEnrollmentID(enrollments []*activities.StudentEnrollment, studentID int64) (int64, error) {
	for _, enrollment := range enrollments {
		if enrollment.StudentID == studentID {
			return enrollment.ID, nil
		}
	}
	return 0, ErrNotEnrolled
}

// UpdateGroupEnrollments updates the student enrollments for a group
// This follows the education.UpdateGroupTeachers pattern but for student enrollments
func (s *Service) UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error {
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupEnrollments", Err: ErrGroupNotFound}
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		enrollments, err := txService.(*Service).enrollmentRepo.FindByGroupID(ctx, groupID)
		if err != nil {
			return &ActivityError{Op: "get current enrollments", Err: err}
		}

		currentStudentIDs, newStudentIDs := s.buildEnrollmentMaps(enrollments, studentIDs)

		if err := s.removeUnwantedEnrollmentsInTx(ctx, txService, currentStudentIDs, newStudentIDs); err != nil {
			return err
		}

		if err := s.addNewEnrollmentsInTx(ctx, txService, groupID, currentStudentIDs, studentIDs); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	return nil
}

// buildEnrollmentMaps creates comparison maps for current and new enrollments
func (s *Service) buildEnrollmentMaps(enrollments []*activities.StudentEnrollment, studentIDs []int64) (map[int64]int64, map[int64]bool) {
	currentStudentIDs := make(map[int64]int64) // studentID -> enrollmentID
	for _, enrollment := range enrollments {
		currentStudentIDs[enrollment.StudentID] = enrollment.ID
	}

	newStudentIDs := make(map[int64]bool)
	for _, studentID := range studentIDs {
		newStudentIDs[studentID] = true
	}

	return currentStudentIDs, newStudentIDs
}

// removeUnwantedEnrollmentsInTx removes students that are no longer enrolled
func (s *Service) removeUnwantedEnrollmentsInTx(ctx context.Context, txService ActivityService, currentStudentIDs map[int64]int64, newStudentIDs map[int64]bool) error {
	for studentID, enrollmentID := range currentStudentIDs {
		if !newStudentIDs[studentID] {
			if err := txService.(*Service).enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
				return &ActivityError{Op: "delete enrollment", Err: err}
			}
		}
	}
	return nil
}

// addNewEnrollmentsInTx adds new student enrollments
func (s *Service) addNewEnrollmentsInTx(ctx context.Context, txService ActivityService, groupID int64, currentStudentIDs map[int64]int64, studentIDs []int64) error {
	for _, studentID := range studentIDs {
		if _, exists := currentStudentIDs[studentID]; !exists {
			enrollment := &activities.StudentEnrollment{
				StudentID:       studentID,
				ActivityGroupID: groupID,
				EnrollmentDate:  time.Now(),
			}

			if err := txService.(*Service).enrollmentRepo.Create(ctx, enrollment); err != nil {
				return &ActivityError{Op: "create enrollment", Err: err}
			}
		}
	}
	return nil
}

// GetEnrolledStudents retrieves all students enrolled in a group
func (s *Service) GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	// Get the enrollments for this group
	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get enrolled students", Err: err}
	}

	// Extract the Student objects from the enrollments
	students := make([]*users.Student, 0, len(enrollments))
	for _, enrollment := range enrollments {
		// Check if the Student relation is loaded
		if enrollment.Student != nil {
			students = append(students, enrollment.Student)
		}
	}

	return students, nil
}

// GetStudentEnrollments retrieves all groups a student is enrolled in
func (s *Service) GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	var result []*activities.Group

	// Use transaction to ensure consistent data
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Get all enrollments for this student
		enrollments, err := txService.(*Service).enrollmentRepo.FindByStudentID(ctx, studentID)
		if err != nil {
			return &ActivityError{Op: "find student enrollments", Err: err}
		}

		// Extract group IDs from enrollments
		groupIDs := make([]int64, 0, len(enrollments))
		for _, enrollment := range enrollments {
			groupIDs = append(groupIDs, enrollment.ActivityGroupID)
		}

		// If no enrollments found, return empty slice
		if len(groupIDs) == 0 {
			result = []*activities.Group{}
			return nil
		}

		// Create a filter to get groups by IDs
		options := base.NewQueryOptions()
		filter := base.NewFilter()

		// Convert int64 slice to []interface{}
		interfaceIDs := make([]interface{}, len(groupIDs))
		for i, id := range groupIDs {
			interfaceIDs[i] = id
		}

		filter.In("id", interfaceIDs...)
		options.Filter = filter

		// Get groups using List method
		groups, err := txService.(*Service).groupRepo.List(ctx, options)
		if err != nil {
			return &ActivityError{Op: "get groups by ids", Err: err}
		}

		// Store result for returning after transaction completes
		result = groups
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	return result, nil
}

// GetAvailableGroups retrieves all groups a student can enroll in (not already enrolled)
func (s *Service) GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	// Get all active groups - assuming FindOpenGroups is the correct method
	allGroups, err := s.groupRepo.FindOpenGroups(ctx)
	if err != nil {
		return nil, &ActivityError{Op: "get all groups", Err: err}
	}

	// Get enrollments for this student
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	// Create a map of enrolled group IDs for quick lookup
	enrolledMap := make(map[int64]bool)
	for _, enrollment := range enrollments {
		enrolledMap[enrollment.ActivityGroupID] = true
	}

	// Filter out already enrolled groups
	var availableGroups []*activities.Group
	for _, group := range allGroups {
		if !enrolledMap[group.ID] {
			availableGroups = append(availableGroups, group)
		}
	}

	return availableGroups, nil
}

// UpdateAttendanceStatus updates the attendance status for an enrollment
func (s *Service) UpdateAttendanceStatus(ctx context.Context, enrollmentID int64, status *string) error {
	// Execute in transaction for consistency
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(ActivityService)

		// Check if enrollment exists
		_, err := txService.(*Service).enrollmentRepo.FindByID(ctx, enrollmentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrEnrollmentNotFound
			}
			return &ActivityError{Op: "find enrollment", Err: err}
		}

		// Update the status
		if err := txService.(*Service).enrollmentRepo.UpdateAttendanceStatus(ctx, enrollmentID, status); err != nil {
			return &ActivityError{Op: "update attendance status", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "update attendance status", Err: err}
	}

	return nil
}

// GetEnrollmentsByDate gets all enrollments for a specific date
func (s *Service) GetEnrollmentsByDate(ctx context.Context, date time.Time) ([]*activities.StudentEnrollment, error) {
	// NOTE: This is a placeholder implementation
	// The actual implementation would likely query enrollments with schedules active on the given date
	// and possibly filter for attendance status

	// This would require a custom repository method with SQL join between enrollments and schedules
	// For now, we just use the date range finder with the date as both start and end

	// Using the repository's FindByEnrollmentDateRange method
	enrollments, err := s.enrollmentRepo.FindByEnrollmentDateRange(ctx, date, date)
	if err != nil {
		return nil, &ActivityError{Op: "get enrollments by date", Err: err}
	}

	return enrollments, nil
}

// GetEnrollmentHistory gets a student's enrollment history within a date range
func (s *Service) GetEnrollmentHistory(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*activities.StudentEnrollment, error) {
	// NOTE: This is a placeholder implementation
	// The actual implementation would query enrollments with schedules active in the given date range

	// Get all enrollments for the student
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get enrollment history", Err: err}
	}

	// Filter by date range (simplified logic, actual implementation would be more complex)
	var filteredEnrollments []*activities.StudentEnrollment
	for _, enrollment := range enrollments {
		if !enrollment.EnrollmentDate.Before(startDate) && !enrollment.EnrollmentDate.After(endDate) {
			filteredEnrollments = append(filteredEnrollments, enrollment)
		}
	}

	return filteredEnrollments, nil
}
