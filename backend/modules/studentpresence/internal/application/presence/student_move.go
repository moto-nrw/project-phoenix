package presence

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	opMoveStudents          = "MoveStudentsToActiveGroup"
	opMoveStudentsToTransit = "MoveStudentsToTransit"
)

// MoveStudentsToActiveGroupAuthorized is the HTTP-facing bulk move path. It
// revalidates the caller's source/target access against the locked move state
// before any current visit is ended or recreated.
func (s *service) MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error) {
	return s.moveStudentsToActiveGroup(ctx, studentIDs, activeGroupID, &auth, false)
}

// MoveStudentsToOpenRoomSessionAuthorized moves children into the room session
// of a released room (#3066). Everything the ordinary move locks and validates
// still applies, including the caller's rights over each child's current
// place; only the destination no longer has to be supervised, because a
// released room is a shared destination rather than someone's supervision.
func (s *service) MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error) {
	return s.moveStudentsToActiveGroup(ctx, studentIDs, roomSessionID, &auth, true)
}

func (s *service) moveStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization, openRoomTarget bool) (*StudentMoveResult, error) {
	var result *StudentMoveResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var moveErr error
		result, moveErr = s.moveStudentsToActiveGroupLocked(txCtx, studentIDs, activeGroupID, auth, openRoomTarget)
		return moveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// studentMovePlan is the locked state a move into one session works on.
// studentIDs are the children still to move; stale are requested children
// whose visit ended while the move waited for its locks.
type studentMovePlan struct {
	target         *ports.ActiveGroup
	requestedIDs   []int64
	studentIDs     []int64
	stale          map[int64]struct{}
	openAttendance map[int64]studentpresence.Attendance
	currentVisits  map[int64]*studentpresence.Visit
}

func (s *service) moveStudentsToActiveGroupLocked(ctx context.Context, studentIDs []int64, activeGroupID int64, auth *StudentMoveAuthorization, openRoomTarget bool) (*StudentMoveResult, error) {
	if activeGroupID <= 0 {
		return nil, &ActiveError{Op: opMoveStudents, Err: ErrInvalidData}
	}
	uniqueIDs, err := normalizeMoveStudentIDs(studentIDs, opMoveStudents)
	if err != nil {
		return nil, err
	}
	if err := s.rejectInactiveMoveStudents(ctx, uniqueIDs); err != nil {
		return nil, err
	}

	restricted := auth != nil && !auth.BypassResourceChecks
	plan, err := s.lockStudentMovePlan(ctx, uniqueIDs, activeGroupID, restricted)
	if err != nil {
		return nil, err
	}
	if restricted {
		if len(plan.studentIDs) == 0 {
			return plan.staleResult(), nil
		}
		if err := s.authorizeStudentMove(ctx, *auth, plan.target, plan.studentIDs, plan.openAttendance, plan.currentVisits, opMoveStudents, openRoomTarget); err != nil {
			return nil, err
		}
	}

	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: opMoveStudents, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		return plan.binaryResult(), nil
	}
	return s.applyStudentMovePlan(ctx, plan)
}

// normalizeMoveStudentIDs drops duplicates and nonpositive IDs; a request
// without a usable ID is invalid.
func normalizeMoveStudentIDs(studentIDs []int64, op string) ([]int64, error) {
	if len(studentIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}
	uniqueIDs := slices.DeleteFunc(dedupeStudentIDs(studentIDs), func(id int64) bool { return id <= 0 })
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}
	return uniqueIDs, nil
}

// rejectInactiveMoveStudents locks the children and rejects the move of a
// graduate or of a child whose care has ended. Moving a departed child between
// rooms is a presence write like any other (#2487).
func (s *service) rejectInactiveMoveStudents(ctx context.Context, studentIDs []int64) error {
	if s.StudentRepo == nil {
		return nil
	}
	lockedStudents, err := s.StudentRepo.FindByIDsForUpdate(ctx, studentIDs)
	if err != nil {
		return &ActiveError{Op: opMoveStudents, Err: err}
	}
	today := timezone.TodayDate()
	for _, studentID := range studentIDs {
		student := lockedStudents[studentID]
		if student == nil {
			continue
		}
		if student.IsAlumnus() {
			return &ActiveError{Op: opMoveStudents, Err: ErrStudentGraduated}
		}
		if student.CareEndedOn(today) {
			return &ActiveError{Op: opMoveStudents, Err: ErrStudentCareEnded}
		}
	}
	return nil
}

// lockStudentMovePlan locks the move state: for a restricted caller the target
// room first, then attendance, the source and target sessions, and a locked
// re-read of the current visits. A restricted move drops the children whose
// visit ended meanwhile, so it never acts on access it checked for a place
// the child already left.
func (s *service) lockStudentMovePlan(ctx context.Context, studentIDs []int64, activeGroupID int64, restricted bool) (*studentMovePlan, error) {
	var lockedTargetRoomID int64
	if restricted {
		var err error
		lockedTargetRoomID, err = s.lockMoveTargetRoom(ctx, activeGroupID, opMoveStudents)
		if err != nil {
			return nil, err
		}
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, studentIDs, opMoveStudents)
	if err != nil {
		return nil, err
	}
	targetGroup, err := s.lockMoveGroups(ctx, studentIDs, currentVisits, activeGroupID, opMoveStudents)
	if err != nil {
		return nil, err
	}
	if lockedTargetRoomID > 0 && targetGroup.RoomID != lockedTargetRoomID {
		return nil, &ActiveError{Op: opMoveStudents, Err: ErrDatabaseOperation}
	}
	refreshedVisits, err := s.currentPresenceVisits(ctx, studentIDs, true)
	if err != nil {
		return nil, &ActiveError{Op: opMoveStudents, Err: ErrDatabaseOperation}
	}
	if refreshedVisits == nil {
		refreshedVisits = map[int64]*studentpresence.Visit{}
	}

	plan := &studentMovePlan{
		target:         targetGroup,
		requestedIDs:   studentIDs,
		studentIDs:     studentIDs,
		stale:          make(map[int64]struct{}),
		openAttendance: openAttendance,
		currentVisits:  refreshedVisits,
	}
	if restricted {
		plan.dropStaleVisits(currentVisits)
	}
	return plan, nil
}

// dropStaleVisits removes the children whose visit was open before the locks
// and is gone after them.
func (p *studentMovePlan) dropStaleVisits(visitsBeforeLock map[int64]*studentpresence.Visit) {
	moveIDs := make([]int64, 0, len(p.studentIDs))
	for _, studentID := range p.studentIDs {
		if visitsBeforeLock[studentID] != nil && p.currentVisits[studentID] == nil {
			p.stale[studentID] = struct{}{}
			continue
		}
		moveIDs = append(moveIDs, studentID)
	}
	p.studentIDs = moveIDs
}

// staleResult starts the move result with the stale children skipped as not
// present, in request order.
func (p *studentMovePlan) staleResult() *StudentMoveResult {
	result := newStudentMoveResult(&p.target.ID, &p.target.RoomID)
	for _, studentID := range p.requestedIDs {
		if _, stale := p.stale[studentID]; stale {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
		}
	}
	return result
}

// binaryResult answers a move in binary presence mode, which tracks no room
// visits: nothing moves, and a child with a visit is reported as a conflict.
func (p *studentMovePlan) binaryResult() *StudentMoveResult {
	result := newStudentMoveResult(&p.target.ID, &p.target.RoomID)
	for _, studentID := range p.requestedIDs {
		if _, stale := p.stale[studentID]; stale {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
			continue
		}
		if p.currentVisits[studentID] != nil {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipConflict})
			continue
		}
		result.Unchanged = append(result.Unchanged, studentID)
	}
	return result
}

func (s *service) applyStudentMovePlan(ctx context.Context, plan *studentMovePlan) (*StudentMoveResult, error) {
	if err := s.ensureCapacityForStudentMove(ctx, plan.target, plan.studentIDs, plan.openAttendance, plan.currentVisits); err != nil {
		return nil, err
	}

	result := plan.staleResult()
	for _, studentID := range plan.studentIDs {
		if err := s.moveStudentIntoGroup(ctx, plan, studentID, result); err != nil {
			return nil, err
		}
	}

	if len(result.Moved) > 0 {
		if err := s.UpdateSessionActivity(ctx, plan.target.ID); err != nil {
			return nil, err
		}
	}
	if moveResultOnlySkippedNotPresent(result) {
		return nil, &ActiveError{Op: opMoveStudents, Err: ErrStudentsNotPresent}
	}

	return result, nil
}

// moveStudentIntoGroup ends the child's current visit and opens one in the
// target session, recording the outcome in result.
func (s *service) moveStudentIntoGroup(ctx context.Context, plan *studentMovePlan, studentID int64, result *StudentMoveResult) error {
	if !studentHasOpenAttendance(plan.openAttendance, studentID) {
		result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipNotPresent})
		return nil
	}

	currentVisit := plan.currentVisits[studentID]
	if currentVisit != nil && currentVisit.ActiveGroupID == plan.target.ID {
		result.Unchanged = append(result.Unchanged, studentID)
		return nil
	}

	var previousActiveGroupID int64
	if currentVisit != nil {
		previousActiveGroupID = currentVisit.ActiveGroupID
		if err := s.endVisitForMove(ctx, currentVisit.ID); err != nil {
			return err
		}
	}

	visit := &studentpresence.Visit{
		StudentID:     studentID,
		ActiveGroupID: plan.target.ID,
		EntryTime:     time.Now(),
	}
	if err := s.createVisitWithoutAttendanceMutation(ctx, visit); err != nil {
		if errors.Is(err, ErrStudentAlreadyActive) {
			result.Skipped = append(result.Skipped, StudentMoveSkipped{StudentID: studentID, Reason: StudentMoveSkipConflict})
			return nil
		}
		return err
	}
	result.Moved = append(result.Moved, studentID)
	if previousActiveGroupID > 0 {
		result.PreviousActiveGroupIDs[studentID] = previousActiveGroupID
	}
	return nil
}

// endVisitForMove ends the visit a move replaces; a visit that already ended
// or vanished needs no ending.
func (s *service) endVisitForMove(ctx context.Context, visitID int64) error {
	if err := s.EndVisit(ctx, visitID); err != nil {
		if !errors.Is(err, ErrVisitAlreadyEnded) && !errors.Is(err, ErrVisitNotFound) {
			return err
		}
	}
	return nil
}

func (s *service) ensureCapacityForStudentMove(
	ctx context.Context,
	targetGroup *ports.ActiveGroup,
	studentIDs []int64,
	openAttendance map[int64]studentpresence.Attendance,
	currentVisits map[int64]*studentpresence.Visit,
) error {
	groups, err := s.GroupRepo.FindByIDs(ctx, moveSourceGroupIDs(currentVisits, targetGroup.ID))
	if err != nil {
		return &ActiveError{Op: opMoveStudents, Err: ErrDatabaseOperation}
	}

	incoming := 0
	for _, studentID := range studentIDs {
		if studentEntersTargetRoom(targetGroup, groups, openAttendance, currentVisits, studentID) {
			incoming++
		}
	}

	if incoming == 0 {
		return nil
	}
	return s.ensureRoomCapacity(ctx, targetGroup.RoomID, incoming)
}

// moveSourceGroupIDs lists every session other than the target that a child
// is currently in, once each.
func moveSourceGroupIDs(currentVisits map[int64]*studentpresence.Visit, targetGroupID int64) []int64 {
	groupIDs := make([]int64, 0, len(currentVisits))
	seenGroupIDs := make(map[int64]struct{}, len(currentVisits))
	for _, visit := range currentVisits {
		if visit == nil || visit.ActiveGroupID == targetGroupID {
			continue
		}
		if _, seen := seenGroupIDs[visit.ActiveGroupID]; seen {
			continue
		}
		seenGroupIDs[visit.ActiveGroupID] = struct{}{}
		groupIDs = append(groupIDs, visit.ActiveGroupID)
	}
	return groupIDs
}

// studentEntersTargetRoom reports whether moving the present child adds a
// person to the target room: a child without a visit, or one whose current
// session is in another (or an unknown) room.
func studentEntersTargetRoom(
	targetGroup *ports.ActiveGroup,
	groups map[int64]*ports.ActiveGroup,
	openAttendance map[int64]studentpresence.Attendance,
	currentVisits map[int64]*studentpresence.Visit,
	studentID int64,
) bool {
	if !studentHasOpenAttendance(openAttendance, studentID) {
		return false
	}
	currentVisit := currentVisits[studentID]
	if currentVisit == nil {
		return true
	}
	if currentVisit.ActiveGroupID == targetGroup.ID {
		return false
	}
	currentGroup := groups[currentVisit.ActiveGroupID]
	return currentGroup == nil || currentGroup.RoomID != targetGroup.RoomID
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
	uniqueIDs, err := normalizeMoveStudentIDs(studentIDs, opMoveStudentsToTransit)
	if err != nil {
		return nil, err
	}

	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: opMoveStudentsToTransit, Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary {
		result := newStudentMoveResult(nil, nil)
		result.Unchanged = append(result.Unchanged, uniqueIDs...)
		return result, nil
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, opMoveStudentsToTransit)
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

		if err := s.endVisitForMove(ctx, currentVisit.ID); err != nil {
			return nil, err
		}
		result.Moved = append(result.Moved, studentID)
	}
	if moveResultOnlySkippedNotPresent(result) {
		return nil, &ActiveError{Op: opMoveStudentsToTransit, Err: ErrStudentsNotPresent}
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
func (s *service) lockMoveGroups(ctx context.Context, studentIDs []int64, currentVisits map[int64]*studentpresence.Visit, targetGroupID int64, op string) (*ports.ActiveGroup, error) {
	var targetGroup *ports.ActiveGroup
	for _, id := range moveGroupLockOrder(studentIDs, currentVisits, targetGroupID) {
		group, err := s.GroupRepo.FindByIDForUpdate(ctx, id)
		if err != nil {
			return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
		}
		if id != targetGroupID {
			if group == nil || !group.IsActive() {
				return nil, studentMoveForbidden(op)
			}
			continue
		}
		if group == nil {
			return nil, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
		}
		if !group.IsActive() {
			return nil, &ActiveError{Op: op, Err: ErrActiveGroupAlreadyEnded}
		}
		targetGroup = group
	}
	return targetGroup, nil
}

// moveGroupLockOrder returns the target and the children's source sessions in
// ascending ID order.
func moveGroupLockOrder(studentIDs []int64, currentVisits map[int64]*studentpresence.Visit, targetGroupID int64) []int64 {
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
	return ids
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
	if err := s.SchoolPresence.LockRoomSessionWrites(ctx, group.RoomID); err != nil {
		return 0, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	return group.RoomID, nil
}

func (s *service) lockActiveGroupForMove(ctx context.Context, activeGroupID int64, op string) (*ports.ActiveGroup, error) {
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
