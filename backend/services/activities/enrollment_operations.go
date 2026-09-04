package activities

import (
	"context"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/users"
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

	added := newEnrollmentStudentIDs(currentStudentIDs, studentIDs)

	// Lock EVERY student this call reasons about — the ones it would insert AND
	// the ones already enrolled — in one ascending-id pass, and decide from the
	// LOCKED rows. Locking only the additions was not enough: a transition can
	// graduate an already-enrolled child right after `enrollments` was read, so
	// the preserved set below would be computed from a stale "active" relation
	// and would delete the very enrollment the graduation must keep for the
	// revert and for future materialization (#405 review).
	locked, err := s.lockEnrollmentStudents(ctx, enrollments, added)
	if err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	// Only the IDs this call would newly insert are validated. A graduated
	// child's existing row is deliberately kept (see `preserved` below), so
	// re-submitting a roster that still carries them must not fail — but a
	// caller must not be able to ADD one either (#405 review).
	if err := rejectAlumniFrom(locked, added); err != nil {
		return &ActivityError{Op: "update group enrollments", Err: err}
	}

	// Alumni are invisible to GetEnrolledStudents, so the roster a caller reads
	// via GET /api/activities/{id}/students never contains them and the PUT that
	// replaces it never lists them either. Deleting "everything not submitted"
	// would therefore silently drop exactly the enrollments a grade transition
	// deliberately preserved — the rows the revert and future materialization
	// still need. A hidden row cannot be removed by an edit that could not show
	// it (#405 review).
	preserved := hiddenAlumnusEnrollments(enrollments, locked)

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
// carries no intent (#405 review).
//
// The LOCKED status wins where we have one: it is the row a concurrent
// graduation had to commit before, whereas the Student relation on `enrollments`
// was loaded before the lock and can already be stale. Where no locked status is
// available (no student repository wired, or the row vanished) the loaded
// relation is the fallback, and a row with neither is NOT preserved — an unknown
// status must not silently make enrollments undeletable.
func hiddenAlumnusEnrollments(
	enrollments []*activities.StudentEnrollment,
	locked map[int64]*users.Student,
) map[int64]bool {
	hidden := make(map[int64]bool)
	for _, enrollment := range enrollments {
		if student, ok := locked[enrollment.StudentID]; ok {
			if student.Status == users.StudentStatusAlumnus {
				hidden[enrollment.StudentID] = true
			}
			continue
		}
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
// The status is read UNDER a FOR UPDATE lock on the student row — see
// lockStudentStatuses for why an unlocked read is not enough.
func (s *Service) rejectAlumni(ctx context.Context, studentIDs []int64) error {
	locked, err := s.lockStudentStatuses(ctx, studentIDs)
	if err != nil {
		return err
	}
	return rejectAlumniFrom(locked, studentIDs)
}

// rejectAlumniFrom applies the alumni refusal to statuses already read under
// their row locks by lockStudentStatuses. IDs with no entry are left to the
// foreign key, which is where a non-existent student has always been rejected.
func rejectAlumniFrom(locked map[int64]*users.Student, studentIDs []int64) error {
	today := timezone.TodayDate()
	for _, studentID := range studentIDs {
		student := locked[studentID]
		if student == nil {
			continue
		}
		if student.Status == users.StudentStatusAlumnus {
			return ErrStudentIsAlumnus
		}
		// A child whose care has ended cannot be booked into an AG or care
		// offering any more (#2487).
		if student.CareEndedOn(today) {
			return ErrStudentCareEnded
		}
	}
	return nil
}

// lockEnrollmentStudents locks — and reads the status of — every student a
// roster replacement decides about: the ones it would insert plus the ones
// already enrolled, whose rows the preservation decision depends on just as
// much. One ascending-id pass over the union keeps the project-wide student lock
// order and takes each row only once.
func (s *Service) lockEnrollmentStudents(
	ctx context.Context,
	enrollments []*activities.StudentEnrollment,
	added []int64,
) (map[int64]*users.Student, error) {
	ids := make([]int64, 0, len(enrollments)+len(added))
	for _, enrollment := range enrollments {
		ids = append(ids, enrollment.StudentID)
	}
	ids = append(ids, added...)

	locked, err := s.lockStudentStatuses(ctx, ids)
	if err == nil {
		return locked, nil
	}
	// Without a student repository the enrolled rows simply stay unlocked and
	// the preservation decision falls back to the loaded relation (the behaviour
	// before this pass existed). An ADD, however, still has to fail closed.
	if errors.Is(err, errStudentRepoMissing) && len(added) == 0 {
		return nil, nil
	}
	return nil, err
}

// lockStudentStatuses takes a FOR UPDATE lock on each given student row, in
// ascending id order (the project-wide student lock order — see
// users.LockStudentsForUpdate), and returns the ROW read UNDER that lock. It
// returns whole students rather than just the status because two questions are
// decided from it now: graduated, and care ended (#2487).
// Rows that no longer exist are absent from the result.
//
// An unlocked read is not enough: a grade transition apply locks exactly these
// rows, flips them to alumnus and reconciles their rosters, so a status checked
// just before that commit is already obsolete when the enrollment write lands.
// With the lock the two transactions serialize: either graduation commits first
// and we observe the alumnus status, or we hold the rows and graduation waits
// until our write is committed and visible to its own pass (#405 review).
func (s *Service) lockStudentStatuses(ctx context.Context, studentIDs []int64) (map[int64]*users.Student, error) {
	ordered := ascendingUniqueIDs(studentIDs)
	if len(ordered) == 0 {
		return nil, nil
	}
	if s.studentRepo == nil {
		// Wired in services.NewFactory; a nil repo means the caller built the
		// service without the dependency this check needs. Failing is the only
		// safe answer — silently skipping would reopen the gap above.
		return nil, errStudentRepoMissing
	}

	locked := make(map[int64]*users.Student, len(ordered))
	for _, studentID := range ordered {
		student, err := s.studentRepo.FindByIDForUpdate(ctx, studentID)
		if err != nil {
			if isRepositoryNotFound(err) {
				continue
			}
			return nil, err
		}
		locked[studentID] = student
	}
	return locked, nil
}

// isAlumnus reports whether a student has graduated. It backs the read-side
// alumnus gates, so it takes NO row lock: a read decides nothing a later write
// depends on, and locking every enrollment read would put staff traffic in the
// queue of a running grade transition for no gain. A missing row is not an
// alumnus — the caller's own lookups decide what "unknown student" means.
//
// A nil student repository is an error rather than a silent "not graduated":
// the repository is wired in services.NewFactory, so a nil one means the caller
// built the service without the dependency this decision needs, and answering
// "no" would hand out exactly the rows the gate exists to hide.
func (s *Service) isAlumnus(ctx context.Context, studentID int64) (bool, error) {
	student, err := s.loadStudentForGate(ctx, studentID)
	if err != nil || student == nil {
		return false, err
	}
	return student.IsAlumnus(), nil
}

// careEnded reports whether the child has left the OGS. Same unlocked-read
// rationale as isAlumnus (#2487).
func (s *Service) careEnded(ctx context.Context, studentID int64) (bool, error) {
	student, err := s.loadStudentForGate(ctx, studentID)
	if err != nil || student == nil {
		return false, err
	}
	return student.CareEndedOn(timezone.TodayDate()), nil
}

func (s *Service) loadStudentForGate(ctx context.Context, studentID int64) (*users.Student, error) {
	if studentID <= 0 {
		return nil, nil
	}
	if s.studentRepo == nil {
		return nil, errStudentRepoMissing
	}
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		if isRepositoryNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return student, nil
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
		// A departed child leaves the AG roster (#2487). Their booking was
		// capped at their last care day; this keeps the roster read agreeing
		// with the date-aware ones.
		if enrollment.Student.CareEndedOn(timezone.TodayDate()) {
			continue
		}
		students = append(students, enrollment.Student)
	}

	return students, nil
}

// GetStudentEnrollments retrieves all groups a student is enrolled in.
//
// A graduated (alumnus) student has no enrollments as far as this read is
// concerned. Their rows deliberately survive the grade transition so a revert
// and future materialization can still reason about them, but GetEnrolledStudents
// already hides them from the roster side; without the same gate here the
// staff-facing GET /api/activities/student/{id} handed the identical hidden
// assignments straight back when queried with the graduate's ID (#405 review).
func (s *Service) GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error) {
	alumnus, err := s.isAlumnus(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get student enrollments", Err: err}
	}
	if alumnus {
		return []*activities.Group{}, nil
	}

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

	groups, err := s.groupRepo.ListWithCategory(ctx, &activities.GroupListQuery{IDs: groupIDs})
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
			group = &activities.Group{}
			group.ID = enrollment.ActivityGroupID
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
	alumnus, err := s.isAlumnus(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get available groups", Err: err}
	}
	if alumnus {
		return nil, &ActivityError{Op: "get available groups", Err: ErrStudentNotFound}
	}
	// A child whose care has ended cannot be booked into anything any more, so
	// there is nothing available to offer (#2487).
	ended, err := s.careEnded(ctx, studentID)
	if err != nil {
		return nil, &ActivityError{Op: "get available groups", Err: err}
	}
	if ended {
		return nil, &ActivityError{Op: "get available groups", Err: ErrStudentCareEnded}
	}

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
