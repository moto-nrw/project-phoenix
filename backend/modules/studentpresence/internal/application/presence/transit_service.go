package presence

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

const (
	TransitSkipNotInTransit = studentpresence.TransitSkipNotInTransit
	TransitSkipCreateFailed = studentpresence.TransitSkipCreateFailed

	StudentMoveSkipNotPresent = studentpresence.StudentMoveSkipNotPresent
	StudentMoveSkipConflict   = studentpresence.StudentMoveSkipConflict
)

const opAssignTransit = "AssignTransitStudentsToActiveGroup"

// ListStudentsInTransit returns students who are checked in today but do not
// currently have an open room visit.
func (s *service) ListStudentsInTransit(ctx context.Context) ([]int64, error) {
	openAttendanceIDs, err := s.SchoolPresence.ListOpenAttendanceStudentIDs(ctx, s.todayDate().String())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}
	if len(openAttendanceIDs) == 0 {
		return []int64{}, nil
	}

	currentVisits, err := s.currentPresenceVisits(ctx, openAttendanceIDs, false)
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}

	ids := make([]int64, 0, len(openAttendanceIDs))
	for _, studentID := range openAttendanceIDs {
		if _, hasVisit := currentVisits[studentID]; hasVisit {
			continue
		}
		ids = append(ids, studentID)
	}

	return ids, nil
}

// ListStudentsPresentToday returns students with open attendance today,
// regardless of whether they currently have an open room visit.
func (s *service) ListStudentsPresentToday(ctx context.Context) ([]int64, error) {
	ids, err := s.SchoolPresence.ListOpenAttendanceStudentIDs(ctx, s.todayDate().String())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsPresentToday", Err: ErrDatabaseOperation}
	}
	if ids == nil {
		return []int64{}, nil
	}
	return ids, nil
}

// AssignTransitStudentsToActiveGroupAuthorized checks target access in the
// same transaction that assigns children, after locking the target session.
func (s *service) AssignTransitStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*TransitAssignResult, error) {
	return s.assignTransitStudentsInTransaction(ctx, studentIDs, activeGroupID, &auth)
}

// AssignTransitStudentsToActiveGroup is the trusted, pre-authorized entry point.
func (s *service) AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*TransitAssignResult, error) {
	return s.assignTransitStudentsInTransaction(ctx, studentIDs, activeGroupID, nil)
}

func (s *service) assignTransitStudentsInTransaction(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization) (*TransitAssignResult, error) {
	var result *TransitAssignResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.assignTransitStudentsToActiveGroup(txCtx, studentIDs, activeGroupID, auth)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) assignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization) (*TransitAssignResult, error) {
	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: opAssignTransit, Err: ErrInvalidData}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, opAssignTransit)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeTransitTarget(ctx, auth, targetGroup.ID); err != nil {
		return nil, err
	}

	uniqueIDs := slices.DeleteFunc(dedupeStudentIDs(studentIDs), func(id int64) bool { return id <= 0 })
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: opAssignTransit, Err: ErrInvalidData}
	}

	// Binary-mode tenants track no room visits, so there is no transit state
	// to resolve — mirror moveStudentsToActiveGroup's short-circuit.
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: opAssignTransit, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	result := newTransitAssignResult(targetGroup)
	if mode == PresenceModeBinary {
		for _, studentID := range uniqueIDs {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
		}
		return result, nil
	}

	if err := s.assignStudentsInTransit(ctx, targetGroup, uniqueIDs, result); err != nil {
		return nil, err
	}
	if len(result.Assigned) > 0 {
		if err := s.UpdateSessionActivity(ctx, targetGroup.ID); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// authorizeTransitTarget lets a restricted caller assign children only to a
// session they may move children into.
func (s *service) authorizeTransitTarget(ctx context.Context, auth *StudentMoveAuthorization, targetGroupID int64) error {
	if auth == nil || auth.BypassResourceChecks {
		return nil
	}
	allowed, _, err := s.moveTargetAccess(ctx, *auth, targetGroupID, opAssignTransit)
	if err != nil {
		return err
	}
	if !allowed {
		return studentMoveForbidden(opAssignTransit)
	}
	return nil
}

func newTransitAssignResult(targetGroup *ports.ActiveGroup) *TransitAssignResult {
	return &TransitAssignResult{
		Assigned:      []int64{},
		Skipped:       []TransitAssignSkipped{},
		ActiveGroupID: targetGroup.ID,
		RoomID:        targetGroup.RoomID,
	}
}

// assignStudentsInTransit gives every child that is checked in but in no room
// a visit in the target session, after checking the room has space for them.
func (s *service) assignStudentsInTransit(ctx context.Context, targetGroup *ports.ActiveGroup, studentIDs []int64, result *TransitAssignResult) error {
	openAttendance, currentVisits, err := s.loadMoveState(ctx, studentIDs, opAssignTransit)
	if err != nil {
		return err
	}
	inTransit := make(map[int64]bool, len(studentIDs))
	for _, studentID := range studentIDs {
		_, hasAttendance := openAttendance[studentID]
		_, hasVisit := currentVisits[studentID]
		inTransit[studentID] = hasAttendance && !hasVisit
	}
	if incoming := countTrue(inTransit); incoming > 0 {
		if err := s.ensureRoomCapacity(ctx, targetGroup.RoomID, incoming); err != nil {
			return err
		}
		// All or nothing: a bulk assignment that does not fit is refused as
		// a whole instead of assigning the first children that fit.
		if err := s.ensureActivityParticipantLimit(ctx, targetGroup, incoming); err != nil {
			return err
		}
	}

	for _, studentID := range studentIDs {
		if !inTransit[studentID] {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
			continue
		}
		assigned, err := s.assignTransitStudent(ctx, targetGroup.ID, studentID)
		if err != nil {
			return err
		}
		if !assigned {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
			continue
		}
		result.Assigned = append(result.Assigned, studentID)
	}
	return nil
}

// assignTransitStudent opens the visit; false means the child got a visit
// concurrently and is no longer in transit.
func (s *service) assignTransitStudent(ctx context.Context, targetGroupID, studentID int64) (bool, error) {
	visit := &studentpresence.Visit{
		StudentID:     studentID,
		ActiveGroupID: targetGroupID,
		EntryTime:     time.Now(),
	}
	// The lock-free path, not CreateVisit: this transaction already holds
	// the group row lock, and CreateVisit locks the student row before the
	// group row — the opposite order. Re-acquiring the student lock here
	// would deadlock against a concurrent check-in of the same student.
	// Attendance is already open (checked above), so nothing is lost.
	if err := s.createVisitWithoutAttendanceMutation(ctx, visit); err != nil {
		if errors.Is(err, ErrStudentAlreadyActive) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func countTrue(values map[int64]bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
