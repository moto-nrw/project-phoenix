package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ======== Enrollment Methods ========

// EnrollStudent enrolls a student in an activity group
func (s *Service) EnrollStudent(ctx context.Context, groupID, studentID int64) error {
	// Timetable rosters have provenance and recurrence semantics that this
	// legacy endpoint cannot preserve.
	_, err := s.findMutableActivityGroup(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	// Check if student is already enrolled
	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	for _, enrollment := range enrollments {
		if enrollment.StudentID == studentID {
			return &ActivityError{Op: "enroll student", Err: ErrStudentAlreadyEnrolled}
		}
	}

	// Create enrollment
	enrollment := &activities.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: groupID,
		ValidFrom:       timezone.TodayDate(),
	}
	enrollment.SetTenantID(tenant.FromContext(ctx))

	if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
		return &ActivityError{Op: "enroll student", Err: err}
	}

	return nil
}

// UnenrollStudent removes a student from an activity group
func (s *Service) UnenrollStudent(ctx context.Context, groupID, studentID int64) error {
	if _, err := s.findMutableActivityGroup(ctx, groupID); err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	enrollmentID, err := s.findEnrollmentID(enrollments, studentID)
	if err != nil {
		return &ActivityError{Op: "unenroll student", Err: err}
	}

	if err := s.enrollmentRepo.Delete(ctx, enrollmentID); err != nil {
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
	_, err := s.findMutableActivityGroup(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "UpdateGroupEnrollments", Err: err}
	}

	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	currentStudentIDs, newStudentIDs := s.buildEnrollmentMaps(enrollments, studentIDs)

	if err := s.removeUnwantedEnrollmentsInTx(ctx, s, currentStudentIDs, newStudentIDs); err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	if err := s.addNewEnrollmentsInTx(ctx, s, groupID, currentStudentIDs, studentIDs); err != nil {
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
				ValidFrom:       timezone.TodayDate(),
			}
			enrollment.SetTenantID(tenant.FromContext(ctx))

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
	// Get all enrollments for this student
	enrollments, err := s.enrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	// Extract group IDs from enrollments
	groupIDs := make([]int64, 0, len(enrollments))
	for _, enrollment := range enrollments {
		groupIDs = append(groupIDs, enrollment.ActivityGroupID)
	}

	// If no enrollments found, return empty slice
	if len(groupIDs) == 0 {
		return []*activities.Group{}, nil
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
	groups, err := s.groupRepo.List(ctx, options)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}

	return groups, nil
}

// GetActiveStudentEnrollmentsByStudentIDs retrieves active activity groups for
// multiple students on one calendar date.
func (s *Service) GetActiveStudentEnrollmentsByStudentIDs(ctx context.Context, studentIDs []int64, onDate timezone.Date) (map[int64][]*activities.Group, error) {
	result := make(map[int64][]*activities.Group, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}

	enrollments, err := s.enrollmentRepo.FindActiveByStudentIDs(ctx, studentIDs, onDate)
	if err != nil {
		return nil, &ActivityError{Op: "get active student enrollments by student IDs", Err: err}
	}

	seen := make(map[int64]map[int64]bool, len(studentIDs))
	for _, enrollment := range enrollments {
		if enrollment == nil || enrollment.StudentID <= 0 {
			continue
		}
		group := enrollment.ActivityGroup
		if group == nil {
			group = &activities.Group{Model: base.Model{ID: enrollment.ActivityGroupID}}
		}
		if seen[enrollment.StudentID] == nil {
			seen[enrollment.StudentID] = map[int64]bool{}
		}
		if seen[enrollment.StudentID][group.ID] {
			continue
		}
		seen[enrollment.StudentID][group.ID] = true
		result[enrollment.StudentID] = append(result[enrollment.StudentID], group)
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
	availableGroups := make([]*activities.Group, 0)
	for _, group := range allGroups {
		if !enrolledMap[group.ID] {
			availableGroups = append(availableGroups, group)
		}
	}

	return availableGroups, nil
}
