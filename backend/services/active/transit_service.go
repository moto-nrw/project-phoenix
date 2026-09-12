package active

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	TransitSkipNotInTransit = "not_in_transit"
	TransitSkipCreateFailed = "create_failed"

	StudentMoveSkipNotPresent = "not_present"
	StudentMoveSkipConflict   = "conflict"
)

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
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, "AssignTransitStudentsToActiveGroup")
	if err != nil {
		return nil, err
	}
	if auth != nil && !auth.BypassResourceChecks {
		allowed, _, err := s.moveTargetAccess(ctx, *auth, targetGroup.ID, "AssignTransitStudentsToActiveGroup")
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, studentMoveForbidden("AssignTransitStudentsToActiveGroup")
		}
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	// Binary-mode tenants track no room visits, so there is no transit state
	// to resolve — mirror moveStudentsToActiveGroup's short-circuit.
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		result := &TransitAssignResult{
			Assigned:      []int64{},
			Skipped:       []TransitAssignSkipped{},
			ActiveGroupID: targetGroup.ID,
			RoomID:        targetGroup.RoomID,
		}
		for _, studentID := range uniqueIDs {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
		}
		return result, nil
	}

	openAttendance, err := s.lockOpenAttendance(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}
	currentVisits, err := s.currentPresenceVisits(ctx, uniqueIDs, false)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}

	incoming := 0
	for _, studentID := range uniqueIDs {
		_, hasAttendance := openAttendance[studentID]
		_, hasVisit := currentVisits[studentID]
		if hasAttendance && !hasVisit {
			incoming++
		}
	}
	if incoming > 0 {
		if err := s.ensureRoomCapacity(ctx, targetGroup.RoomID, incoming); err != nil {
			return nil, err
		}
	}

	result := &TransitAssignResult{
		Assigned:      []int64{},
		Skipped:       []TransitAssignSkipped{},
		ActiveGroupID: targetGroup.ID,
		RoomID:        targetGroup.RoomID,
	}

	for _, studentID := range uniqueIDs {
		_, hasAttendance := openAttendance[studentID]
		_, hasVisit := currentVisits[studentID]
		if !hasAttendance || hasVisit {
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
			continue
		}

		visit := &studentpresence.Visit{
			StudentID:     studentID,
			ActiveGroupID: targetGroup.ID,
			EntryTime:     time.Now(),
		}
		// The lock-free path, not CreateVisit: this transaction already holds
		// the group row lock, and CreateVisit locks the student row before the
		// group row — the opposite order. Re-acquiring the student lock here
		// would deadlock against a concurrent check-in of the same student.
		// Attendance is already open (checked above), so nothing is lost.
		if err := s.createVisitWithoutAttendanceMutation(ctx, visit); err != nil {
			if errors.Is(err, ErrStudentAlreadyActive) {
				result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipNotInTransit})
				continue
			}
			return nil, err
		}

		result.Assigned = append(result.Assigned, studentID)
	}

	if len(result.Assigned) > 0 {
		if err := s.UpdateSessionActivity(ctx, targetGroup.ID); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// MoveStudentsToActiveGroupAuthorized is the HTTP-facing bulk move path. It
// revalidates the caller's source/target access against the locked move state
// before any current visit is ended or recreated.
func (s *service) MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error) {
	return s.moveStudentsToActiveGroup(ctx, studentIDs, activeGroupID, &auth)
}

func (s *service) moveStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization) (*StudentMoveResult, error) {
	var result *StudentMoveResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var moveErr error
		result, moveErr = s.moveStudentsToActiveGroupLocked(txCtx, studentIDs, activeGroupID, auth)
		return moveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) moveStudentsToActiveGroupLocked(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization) (*StudentMoveResult, error) {
	const op = "MoveStudentsToActiveGroup"

	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}
	requestedIDs := uniqueIDs
	if s.StudentRepo != nil {
		lockedStudents, err := s.StudentRepo.FindByIDsForUpdate(ctx, uniqueIDs)
		if err != nil {
			return nil, &ActiveError{Op: op, Err: err}
		}
		today := timezone.TodayDate()
		for _, studentID := range uniqueIDs {
			student := lockedStudents[studentID]
			if student == nil {
				continue
			}
			if student.IsAlumnus() {
				return nil, &ActiveError{Op: op, Err: ErrStudentGraduated}
			}
			// Moving a departed child between rooms is a presence write like
			// any other (#2487).
			if student.CareEndedOn(today) {
				return nil, &ActiveError{Op: op, Err: ErrStudentCareEnded}
			}
		}
	}

	var lockedTargetRoomID int64
	staleVisitIDs := make(map[int64]struct{})
	if auth != nil && !auth.BypassResourceChecks {
		var err error
		lockedTargetRoomID, err = s.lockMoveTargetRoom(ctx, activeGroupID, op)
		if err != nil {
			return nil, err
		}
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, op)
	if err != nil {
		return nil, err
	}
	targetGroup, err := s.lockMoveGroups(ctx, uniqueIDs, currentVisits, activeGroupID, op)
	if err != nil {
		return nil, err
	}
	if lockedTargetRoomID > 0 && targetGroup.RoomID != lockedTargetRoomID {
		return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	refreshedVisits, err := s.currentPresenceVisits(ctx, uniqueIDs, true)
	if err != nil {
		return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if refreshedVisits == nil {
		refreshedVisits = map[int64]*studentpresence.Visit{}
	}
	if auth != nil && !auth.BypassResourceChecks {
		moveIDs := make([]int64, 0, len(uniqueIDs))
		for _, studentID := range uniqueIDs {
			if currentVisits[studentID] != nil && refreshedVisits[studentID] == nil {
				staleVisitIDs[studentID] = struct{}{}
				continue
			}
			moveIDs = append(moveIDs, studentID)
		}
		currentVisits = refreshedVisits
		if len(moveIDs) == 0 {
			result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
			for _, studentID := range requestedIDs {
				if _, stale := staleVisitIDs[studentID]; stale {
					result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
				}
			}
			return result, nil
		}
		if err := s.authorizeStudentMove(ctx, *auth, targetGroup, moveIDs, openAttendance, currentVisits, op); err != nil {
			return nil, err
		}
		uniqueIDs = moveIDs
	} else {
		currentVisits = refreshedVisits
	}
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: op, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
		for _, studentID := range requestedIDs {
			if _, stale := staleVisitIDs[studentID]; stale {
				result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
				continue
			}
			if currentVisits[studentID] != nil {
				result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipConflict})
				continue
			}
			result.Unchanged = append(result.Unchanged, studentID)
		}
		return result, nil
	}

	if err := s.ensureCapacityForStudentMove(ctx, targetGroup, uniqueIDs, openAttendance, currentVisits); err != nil {
		return nil, err
	}

	result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
	for _, studentID := range requestedIDs {
		if _, stale := staleVisitIDs[studentID]; stale {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
		}
	}
	for _, studentID := range uniqueIDs {
		if !studentHasOpenAttendance(openAttendance, studentID) {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
			continue
		}

		currentVisit := currentVisits[studentID]
		if currentVisit != nil && currentVisit.ActiveGroupID == targetGroup.ID {
			result.Unchanged = append(result.Unchanged, studentID)
			continue
		}

		var previousActiveGroupID int64
		if currentVisit != nil {
			previousActiveGroupID = currentVisit.ActiveGroupID
			if err := s.EndVisit(ctx, currentVisit.ID); err != nil {
				if !errors.Is(err, ErrVisitAlreadyEnded) && !errors.Is(err, ErrVisitNotFound) {
					return nil, err
				}
			}
		}

		visit := &studentpresence.Visit{
			StudentID:     studentID,
			ActiveGroupID: targetGroup.ID,
			EntryTime:     time.Now(),
		}
		if err := s.createVisitWithoutAttendanceMutation(ctx, visit); err != nil {
			if errors.Is(err, ErrStudentAlreadyActive) {
				result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipConflict})
				continue
			}
			return nil, err
		}
		result.Moved = append(result.Moved, studentID)
		if previousActiveGroupID > 0 {
			result.PreviousActiveGroupIDs[studentID] = previousActiveGroupID
		}
	}

	if len(result.Moved) > 0 {
		if err := s.UpdateSessionActivity(ctx, targetGroup.ID); err != nil {
			return nil, err
		}
	}
	if moveResultOnlySkippedNotPresent(result) {
		return nil, &ActiveError{Op: op, Err: ErrStudentsNotPresent}
	}

	return result, nil
}

func (s *service) ensureCapacityForStudentMove(
	ctx context.Context,
	targetGroup *active.Group,
	studentIDs []int64,
	openAttendance map[int64]studentpresence.Attendance,
	currentVisits map[int64]*studentpresence.Visit,
) error {
	groupIDs := make([]int64, 0, len(currentVisits))
	seenGroupIDs := make(map[int64]struct{}, len(currentVisits))
	for _, visit := range currentVisits {
		if visit == nil || visit.ActiveGroupID == targetGroup.ID {
			continue
		}
		if _, seen := seenGroupIDs[visit.ActiveGroupID]; seen {
			continue
		}
		seenGroupIDs[visit.ActiveGroupID] = struct{}{}
		groupIDs = append(groupIDs, visit.ActiveGroupID)
	}

	groups, err := s.GroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return &ActiveError{Op: "MoveStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}

	incoming := 0
	for _, studentID := range studentIDs {
		if !studentHasOpenAttendance(openAttendance, studentID) {
			continue
		}
		currentVisit := currentVisits[studentID]
		if currentVisit == nil {
			incoming++
			continue
		}
		if currentVisit.ActiveGroupID == targetGroup.ID {
			continue
		}
		currentGroup := groups[currentVisit.ActiveGroupID]
		if currentGroup == nil || currentGroup.RoomID != targetGroup.RoomID {
			incoming++
		}
	}

	if incoming == 0 {
		return nil
	}
	return s.ensureRoomCapacity(ctx, targetGroup.RoomID, incoming)
}

// MoveStudentsToTransitAuthorized is the HTTP-facing transit move path. It
// revalidates source-room access against the locked move state before ending
// any current visit.
func (s *service) MoveStudentsToTransitAuthorized(ctx context.Context, studentIDs []int64, auth StudentMoveAuthorization) (*StudentMoveResult, error) {
	return s.moveStudentsToTransit(ctx, studentIDs, &auth)
}

func (s *service) moveStudentsToTransit(ctx context.Context, studentIDs []int64, auth *StudentMoveAuthorization) (*StudentMoveResult, error) {
	var result *StudentMoveResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.moveStudentsToTransitLocked(txCtx, studentIDs, auth)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) moveStudentsToTransitLocked(ctx context.Context, studentIDs []int64, auth *StudentMoveAuthorization) (*StudentMoveResult, error) {
	const op = "MoveStudentsToTransit"

	if len(studentIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: op, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		result := newStudentMoveResult(nil, nil)
		result.Unchanged = append(result.Unchanged, uniqueIDs...)
		return result, nil
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, op)
	if err != nil {
		return nil, err
	}

	result := newStudentMoveResult(nil, nil)
	for _, studentID := range uniqueIDs {
		if !studentHasOpenAttendance(openAttendance, studentID) {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
			continue
		}

		currentVisit := currentVisits[studentID]
		if currentVisit == nil {
			result.Unchanged = append(result.Unchanged, studentID)
			continue
		}

		if err := s.EndVisit(ctx, currentVisit.ID); err != nil {
			if !errors.Is(err, ErrVisitAlreadyEnded) && !errors.Is(err, ErrVisitNotFound) {
				return nil, err
			}
		}
		result.Moved = append(result.Moved, studentID)
	}
	if moveResultOnlySkippedNotPresent(result) {
		return nil, &ActiveError{Op: op, Err: ErrStudentsNotPresent}
	}

	return result, nil
}

func moveResultOnlySkippedNotPresent(result *StudentMoveResult) bool {
	if result == nil || len(result.Moved) > 0 || len(result.Unchanged) > 0 || len(result.Skipped) == 0 {
		return false
	}
	for _, skipped := range result.Skipped {
		if skipped.Reason != StudentMoveSkipNotPresent {
			return false
		}
	}
	return true
}

func newStudentMoveResult(activeGroupID, roomID *int64) *StudentMoveResult {
	return &StudentMoveResult{
		Moved:                  []int64{},
		Unchanged:              []int64{},
		Skipped:                []StudentMoveSkipped{},
		ActiveGroupID:          activeGroupID,
		RoomID:                 roomID,
		PreviousActiveGroupIDs: map[int64]int64{},
	}
}

func (s *service) loadMoveState(ctx context.Context, studentIDs []int64, op string) (map[int64]studentpresence.Attendance, map[int64]*studentpresence.Visit, error) {
	openAttendance, err := s.lockOpenAttendance(ctx, studentIDs)
	if err != nil {
		return nil, nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}

	currentVisits, err := s.currentPresenceVisits(ctx, studentIDs, false)
	if err != nil {
		return nil, nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if currentVisits == nil {
		currentVisits = map[int64]*studentpresence.Visit{}
	}

	return openAttendance, currentVisits, nil
}

func studentHasOpenAttendance(attendances map[int64]studentpresence.Attendance, studentID int64) bool {
	attendance, ok := attendances[studentID]
	return ok && attendance.CheckOutTime == nil
}

// lockMoveGroups locks the target and every source session in ascending ID
// order. This prevents opposing room moves from waiting on each other's group
// row locks.
func (s *service) lockMoveGroups(ctx context.Context, studentIDs []int64, currentVisits map[int64]*studentpresence.Visit, targetGroupID int64, op string) (*active.Group, error) {
	groupIDs := make(map[int64]struct{})
	groupIDs[targetGroupID] = struct{}{}
	for _, studentID := range studentIDs {
		visit := currentVisits[studentID]
		if visit != nil && visit.ActiveGroupID != targetGroupID {
			groupIDs[visit.ActiveGroupID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(groupIDs))
	for id := range groupIDs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var targetGroup *active.Group
	for _, id := range ids {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
		}
		if id == targetGroupID {
			if group == nil {
				return nil, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
			}
			if !group.IsActive() {
				return nil, &ActiveError{Op: op, Err: ErrActiveGroupAlreadyEnded}
			}
			targetGroup = group
			continue
		}
		if group == nil || !group.IsActive() {
			return nil, studentMoveForbidden(op)
		}
	}
	return targetGroup, nil
}

// lockMoveTargetRoom serializes a push move with session changes in its target
// room. The target row is locked afterwards and checked again before visits are
// created, so a concurrent room reassignment cannot use a stale room lock.
func (s *service) lockMoveTargetRoom(ctx context.Context, activeGroupID int64, op string) (int64, error) {
	group, err := s.GroupRepo.FindByID(ctx, activeGroupID)
	if err != nil {
		return 0, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if group == nil || !group.IsActive() || group.RoomID <= 0 {
		return 0, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
	}
	if err := s.GroupRepo.LockRoomSessionWrites(ctx, group.RoomID); err != nil {
		return 0, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	return group.RoomID, nil
}

func (s *service) loadMoveSupervisedGroupIDs(ctx context.Context, staffID int64, op string) (map[int64]struct{}, error) {
	if staffID <= 0 {
		return nil, studentMoveForbidden(op)
	}

	supervisions, err := s.SupervisorRepo.FindActiveByStaffIDForUpdate(ctx, staffID)
	if err != nil {
		return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}

	ids := make(map[int64]struct{}, len(supervisions))
	for _, supervision := range supervisions {
		if supervision == nil || supervision.GroupID <= 0 || !IsSupervisorActive(supervision, time.Now()) {
			continue
		}
		ids[supervision.GroupID] = struct{}{}
	}
	return ids, nil
}

// authorizeStudentMove extends scope for eligible OGS staff only when the
// school enables it. Otherwise it preserves the push-or-pull rule (#2969):
// the caller may move the children when they supervise
// the TARGET group (pull, unchanged since #2329), or when they supervise the
// current group of every present child in the batch (push). On the push path
// the target must additionally be the only running session in its room and
// carry at least one running supervision, so no child is handed to a room
// without a responsible adult and the assignment stays unambiguous. Admin
// callers never reach this function (BypassResourceChecks).
func (s *service) authorizeStudentMove(
	ctx context.Context,
	auth StudentMoveAuthorization,
	targetGroup *active.Group,
	studentIDs []int64,
	openAttendance map[int64]studentpresence.Attendance,
	currentVisits map[int64]*studentpresence.Visit,
	op string,
) error {
	allowed, supervisedGroups, err := s.moveTargetAccess(ctx, auth, targetGroup.ID, op)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	for _, studentID := range studentIDs {
		if !studentHasOpenAttendance(openAttendance, studentID) {
			continue
		}
		currentVisit := currentVisits[studentID]
		if currentVisit == nil {
			return studentMoveForbidden(op)
		}
		if currentVisit.ActiveGroupID == targetGroup.ID {
			continue
		}
		if _, ok := supervisedGroups[currentVisit.ActiveGroupID]; !ok {
			return studentMoveForbidden(op)
		}
	}
	return s.ensureMoveTargetIsSupervised(ctx, targetGroup, op)
}

func (s *service) moveTargetAccess(ctx context.Context, auth StudentMoveAuthorization, targetGroupID int64, op string) (bool, map[int64]struct{}, error) {
	if auth.SchoolWideAttendanceEligible {
		allowed, err := s.schoolWideAttendanceMoveAllowed(ctx, auth.StaffID)
		if err != nil {
			return false, nil, &ActiveError{Op: op, Err: err}
		}
		if allowed {
			return true, nil, nil
		}
	}
	supervisedGroups, err := s.loadMoveSupervisedGroupIDs(ctx, auth.StaffID, op)
	if err != nil {
		return false, nil, err
	}
	_, allowed := supervisedGroups[targetGroupID]
	return allowed, supervisedGroups, nil
}

// This is a scope extension, not the admin bypass: target and child state
// remain locked and validated by the existing move workflow.
func (s *service) schoolWideAttendanceMoveAllowed(ctx context.Context, staffID int64) (bool, error) {
	if s.settings == nil {
		return false, errors.New("attendance edit settings unavailable")
	}
	scope, err := s.settings.ResolveString(ctx, configModel.KeyAttendanceEditScope)
	if err != nil {
		return false, fmt.Errorf("resolve attendance edit scope: %w", err)
	}
	if scope == configModel.AttendanceEditScopeOwn {
		return false, nil
	}
	if scope != configModel.AttendanceEditScopeAllStaff {
		return false, ErrStudentMoveForbidden
	}
	visibility, err := s.settings.ResolveString(ctx, configModel.KeyOperationalOverviewScope)
	if err != nil {
		return false, fmt.Errorf("resolve attendance visibility: %w", err)
	}
	if visibility != configModel.OverviewScopeAllStaff || staffID <= 0 || tenant.FromContext(ctx) <= 0 {
		return false, ErrStudentMoveForbidden
	}
	staff, err := s.StaffRepo.FindByID(ctx, staffID)
	if modelBase.IsNoRows(err) {
		return false, ErrStudentMoveForbidden
	}
	if err != nil {
		return false, err
	}
	if staff == nil || staff.TenantID != tenant.FromContext(ctx) {
		return false, ErrStudentMoveForbidden
	}
	return true, nil
}

// ensureMoveTargetIsSupervised rejects push moves into a room with several
// running sessions (ambiguous assignment) or into a session nobody supervises
// right now.
func (s *service) ensureMoveTargetIsSupervised(ctx context.Context, targetGroup *active.Group, op string) error {
	groupsInRoom, err := s.GroupRepo.FindActiveByRoomID(ctx, targetGroup.RoomID)
	if err != nil {
		return &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if len(groupsInRoom) != 1 {
		return studentMoveForbidden(op)
	}

	supervisors, err := s.SupervisorRepo.FindByActiveGroupIDForUpdate(ctx, targetGroup.ID)
	if err != nil {
		return &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	now := time.Now()
	for _, supervisor := range supervisors {
		if supervisor != nil && IsSupervisorActive(supervisor, now) {
			return nil
		}
	}
	return studentMoveForbidden(op)
}

func studentMoveForbidden(op string) error {
	return &ActiveError{Op: op, Err: ErrStudentMoveForbidden}
}

func (s *service) lockActiveGroupForMove(ctx context.Context, activeGroupID int64, op string) (*active.Group, error) {
	group, err := s.GroupRepo.FindByIDForUpdate(ctx, activeGroupID)
	if err != nil {
		return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if group == nil || !group.IsActive() {
		if group == nil {
			return nil, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
		}
		return nil, &ActiveError{Op: op, Err: ErrActiveGroupAlreadyEnded}
	}
	return group, nil
}

// createVisitWithoutAttendanceMutation inserts a visit for a student whose
// attendance is already open, without touching attendance rows. Callers hold
// the target group's row lock (lockActiveGroupForMove) and this helper takes
// no student row lock — CreateVisit locks student-then-group, so re-acquiring
// the student lock under the group lock would invert that order and deadlock
// against a concurrent check-in.
func (s *service) createVisitWithoutAttendanceMutation(ctx context.Context, visit *studentpresence.Visit) error {
	if !validPresenceVisit(visit) {
		return &ActiveError{Op: "CreateMoveVisit", Err: ErrInvalidData}
	}
	if err := s.validateStudentExists(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateMoveVisit", Err: err}
	}
	if err := s.validateActiveGroupExists(ctx, visit.ActiveGroupID); err != nil {
		return &ActiveError{Op: "CreateMoveVisit", Err: err}
	}

	visit.TenantID = tenant.FromContext(ctx)
	stored, err := s.SchoolPresence.RecordVisit(ctx, *visit)
	if err != nil {
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "CreateMoveVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "CreateMoveVisit", Err: ErrDatabaseOperation}
	}

	visit.ID, visit.CreatedAt, visit.UpdatedAt = stored.ID, stored.CreatedAt, stored.UpdatedAt

	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		snapshot, err = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, presenceVisitSnapshot(visit))
		if err != nil {
			return &ActiveError{Op: "CreateMoveVisit", Err: errors.Join(ErrDatabaseOperation, err)}
		}
	}
	s.broadcastVisitCreated(ctx, visit, snapshot)
	return nil
}
