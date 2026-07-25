package activities

import (
	"context"
	"database/sql"
	"errors"
	"sort"

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

	if err := s.rejectAlumni(ctx, []int64{studentID}); err != nil {
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

	// Only the IDs this call would newly insert are validated. A graduated
	// child's existing row is deliberately kept (see `preserved` below), so
	// re-submitting a roster that still carries them must not fail — but a
	// caller must not be able to ADD one either (#405 review).
	if err := s.rejectAlumni(ctx, newEnrollmentStudentIDs(currentStudentIDs, studentIDs)); err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	// Alumni are invisible to GetEnrolledStudents, so the roster a caller reads
	// via GET /api/activities/{id}/students never contains them and the PUT that
	// replaces it never lists them either. Deleting "everything not submitted"
	// would therefore silently drop exactly the enrollments a grade transition
	// deliberately preserved — the rows the revert and future materialization
	// still need. A hidden row cannot be removed by an edit that could not show
	// it (#405 review).
	preserved := hiddenAlumnusEnrollments(enrollments)

	if err := s.removeUnwantedEnrollmentsInTx(ctx, s, currentStudentIDs, newStudentIDs, preserved); err != nil {
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

// hiddenAlumnusEnrollments returns the student IDs whose enrollment must survive
// a replacement update because the read side hides them: a graduated child is
// omitted from GetEnrolledStudents, so their absence from the submitted ID list
// carries no intent (#405 review). A row whose Student relation did not load is
// NOT preserved — an unknown status must not silently make enrollments
// undeletable.
func hiddenAlumnusEnrollments(enrollments []*activities.StudentEnrollment) map[int64]bool {
	hidden := make(map[int64]bool)
	for _, enrollment := range enrollments {
		if enrollment.Student != nil && enrollment.Student.Status == users.StudentStatusAlumnus {
			hidden[enrollment.StudentID] = true
		}
	}
	return hidden
}

// newEnrollmentStudentIDs returns the submitted IDs that are not enrolled yet,
// i.e. exactly the rows addNewEnrollmentsInTx would insert.
func newEnrollmentStudentIDs(currentStudentIDs map[int64]int64, studentIDs []int64) []int64 {
	fresh := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]bool, len(studentIDs))
	for _, studentID := range studentIDs {
		if _, exists := currentStudentIDs[studentID]; exists || seen[studentID] {
			continue
		}
		seen[studentID] = true
		fresh = append(fresh, studentID)
	}
	return fresh
}

// rejectAlumni fails when any of the given students has graduated.
//
// A graduated child is soft-deleted for the whole staff-facing read side:
// GetEnrolledStudents omits them, so an enrollment created for one is invisible
// in the activity's roster and in its counts, and no edit of that roster can
// remove it again — the PUT preserves exactly the rows it could not display.
// Worse, the row is not inert: it is what materialization and a transition
// revert reason about, so the hidden assignment can silently become an active
// one later. Enrollment writes therefore refuse alumni up front instead of
// creating a row nobody can see or delete (#405 review).
//
// The status is read UNDER a FOR UPDATE lock on the student row, in ascending
// id order (the project-wide student lock order — see
// users.LockStudentsForUpdate). An unlocked read is not enough: a grade
// transition apply locks exactly these rows, flips them to alumnus and
// reconciles their rosters, so an enrollment that checked the status just
// before that commit would insert a row for a child who is an alumnus by the
// time it lands — the hidden, undeletable row this check exists to prevent.
// With the lock the two transactions serialize: either graduation commits
// first and we observe the alumnus status, or we hold the row and graduation
// waits until our enrollment is committed and visible to its own pass
// (#405 review).
func (s *Service) rejectAlumni(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if s.studentRepo == nil {
		// Wired in services.NewFactory; a nil repo means the caller built the
		// service without the dependency this check needs. Failing is the only
		// safe answer — silently skipping would reopen the gap above.
		return errors.New("student repository not configured")
	}

	for _, studentID := range ascendingUniqueIDs(studentIDs) {
		student, err := s.studentRepo.FindByIDForUpdate(ctx, studentID)
		if err != nil {
			// Unknown IDs are left to the foreign key, which is where a
			// non-existent student has always been rejected.
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		if student.Status == users.StudentStatusAlumnus {
			return ErrStudentIsAlumnus
		}
	}
	return nil
}

// ascendingUniqueIDs normalizes a set of student ids for locking: duplicates
// removed, non-positive ids dropped, ascending order. The order is what keeps
// two enrollment writes over an overlapping roster from deadlocking against
// each other and against every other student-row locker.
func ascendingUniqueIDs(ids []int64) []int64 {
	ordered := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered
}

// removeUnwantedEnrollmentsInTx removes students that are no longer enrolled,
// except the ones the caller could not have seen (preserved).
func (s *Service) removeUnwantedEnrollmentsInTx(ctx context.Context, txService ActivityService, currentStudentIDs map[int64]int64, newStudentIDs, preserved map[int64]bool) error {
	for studentID, enrollmentID := range currentStudentIDs {
		if !newStudentIDs[studentID] && !preserved[studentID] {
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

// GetEnrolledStudents retrieves all students enrolled in a group.
//
// Graduated (alumnus) students are omitted. Their enrollment rows deliberately
// survive a grade transition so materialization and a transition revert can
// still reason about them, but this is the staff-facing read behind
// GET /api/activities/{id}/students, the activity detail response and the
// enrollment counts — a soft-deleted child must not be listed or counted there
// (#405 review).
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
		if enrollment.Student == nil {
			continue
		}
		if enrollment.Student.Status == users.StudentStatusAlumnus {
			continue
		}
		students = append(students, enrollment.Student)
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
