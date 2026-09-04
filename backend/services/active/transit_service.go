package active

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
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
	openAttendanceIDs, err := s.AttendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}
	if len(openAttendanceIDs) == 0 {
		return []int64{}, nil
	}

	currentVisits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, openAttendanceIDs)
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
	ids, err := s.AttendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsPresentToday", Err: ErrDatabaseOperation}
	}
	if ids == nil {
		return []int64{}, nil
	}
	return ids, nil
}

// AssignTransitStudentsToActiveGroup assigns checked-in students without an
// active room visit to an existing active group/session.
func (s *service) AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*TransitAssignResult, error) {
	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, "AssignTransitStudentsToActiveGroup")
	if err != nil {
		return nil, err
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	// Binary-mode tenants track no room visits, so there is no transit state
	// to resolve — mirror moveStudentsToActiveGroup's short-circuit.
	if s.GetPresenceMode(ctx) == "binary" {
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

	openAttendance, err := s.AttendanceRepo.GetOpenTodayByStudentIDsForUpdate(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}
	currentVisits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, uniqueIDs)
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

		visit := &active.Visit{
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
			result.Skipped = append(result.Skipped, TransitAssignSkipped{StudentID: studentID, Reason: TransitSkipCreateFailed})
			continue
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

	if auth != nil && !auth.BypassResourceChecks {
		// Take the table-level write gate before any active-group row lock. This
		// order matches PostgreSQL's own UPDATE lock acquisition and prevents a
		// session writer from waiting on this move while this move waits on it.
		if err := s.GroupRepo.LockActiveGroupWrites(ctx); err != nil {
			return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
		}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, op)
	if err != nil {
		return nil, err
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, op)
	if err != nil {
		return nil, err
	}
	if auth != nil && !auth.BypassResourceChecks {
		if err := s.lockMoveSourceGroups(ctx, uniqueIDs, currentVisits, targetGroup.ID, op); err != nil {
			return nil, err
		}
		if err := s.authorizeStudentMove(ctx, auth.StaffID, targetGroup, uniqueIDs, openAttendance, currentVisits, op); err != nil {
			return nil, err
		}
	}
	if s.GetPresenceMode(ctx) == "binary" {
		result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
		for _, studentID := range uniqueIDs {
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

		visit := &active.Visit{
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
	openAttendance map[int64]*active.Attendance,
	currentVisits map[int64]*active.Visit,
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
	const op = "MoveStudentsToTransit"

	if len(studentIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	if s.GetPresenceMode(ctx) == "binary" {
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

func (s *service) loadMoveState(ctx context.Context, studentIDs []int64, op string) (map[int64]*active.Attendance, map[int64]*active.Visit, error) {
	openAttendance, err := s.AttendanceRepo.GetOpenTodayByStudentIDsForUpdate(ctx, studentIDs)
	if err != nil {
		return nil, nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}

	currentVisits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if currentVisits == nil {
		currentVisits = map[int64]*active.Visit{}
	}

	return openAttendance, currentVisits, nil
}

func studentHasOpenAttendance(attendances map[int64]*active.Attendance, studentID int64) bool {
	attendance := attendances[studentID]
	return attendance != nil && attendance.CheckOutTime == nil
}

// lockMoveSourceGroups locks every source session that can authorize a push.
// The active-groups write gate is already held, so these locks recheck the
// source state against the same transaction that will create the new visits.
func (s *service) lockMoveSourceGroups(ctx context.Context, studentIDs []int64, currentVisits map[int64]*active.Visit, targetGroupID int64, op string) error {
	groupIDs := make(map[int64]struct{})
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
	for _, id := range ids {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return &ActiveError{Op: op, Err: ErrDatabaseOperation}
		}
		if group == nil || !group.IsActive() {
			return studentMoveForbidden(op)
		}
	}
	return nil
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

// authorizeStudentMove implements the push-or-pull rule for staff-initiated
// room changes (#2969). The caller may move the children when they supervise
// the TARGET group (pull, unchanged since #2329), or when they supervise the
// current group of every present child in the batch (push). On the push path
// the target must additionally be the only running session in its room and
// carry at least one running supervision, so no child is handed to a room
// without a responsible adult and the assignment stays unambiguous. Admin
// callers never reach this function (BypassResourceChecks).
func (s *service) authorizeStudentMove(
	ctx context.Context,
	staffID int64,
	targetGroup *active.Group,
	studentIDs []int64,
	openAttendance map[int64]*active.Attendance,
	currentVisits map[int64]*active.Visit,
	op string,
) error {
	supervisedGroups, err := s.loadMoveSupervisedGroupIDs(ctx, staffID, op)
	if err != nil {
		return err
	}
	if _, ok := supervisedGroups[targetGroup.ID]; ok {
		return nil
	}

	for _, studentID := range studentIDs {
		if !studentHasOpenAttendance(openAttendance, studentID) {
			// Reported as not_present further down; nothing to authorize.
			continue
		}
		currentVisit := currentVisits[studentID]
		if currentVisit == nil {
			// A child in transit has no source room the caller could supervise.
			return studentMoveForbidden(op)
		}
		if currentVisit.ActiveGroupID == targetGroup.ID {
			// Already there; reported as unchanged further down.
			continue
		}
		if _, ok := supervisedGroups[currentVisit.ActiveGroupID]; !ok {
			return studentMoveForbidden(op)
		}
	}

	return s.ensureMoveTargetIsSupervised(ctx, targetGroup, op)
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
func (s *service) createVisitWithoutAttendanceMutation(ctx context.Context, visit *active.Visit) error {
	if visit == nil || visit.Validate() != nil {
		return &ActiveError{Op: "CreateMoveVisit", Err: ErrInvalidData}
	}
	if err := s.validateStudentExists(ctx, visit.StudentID); err != nil {
		return &ActiveError{Op: "CreateMoveVisit", Err: err}
	}
	if err := s.validateActiveGroupExists(ctx, visit.ActiveGroupID); err != nil {
		return &ActiveError{Op: "CreateMoveVisit", Err: err}
	}

	visit.SetTenantID(tenant.FromContext(ctx))
	if err := s.VisitRepo.Create(ctx, visit); err != nil {
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "CreateMoveVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "CreateMoveVisit", Err: ErrDatabaseOperation}
	}

	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		snapshot = s.AttendanceSyncer.MirrorCheckInForVisit(ctx, visit)
	}
	s.broadcastVisitCreated(ctx, visit, snapshot)
	return nil
}
