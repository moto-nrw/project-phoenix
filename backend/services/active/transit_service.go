package active

import (
	"context"
	"errors"
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

	openAttendance, err := s.AttendanceRepo.GetOpenTodayByStudentIDsForUpdate(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}
	currentVisits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
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
		if err := s.CreateVisit(ctx, visit); err != nil {
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
	const op = "MoveStudentsToActiveGroup"

	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, op)
	if err != nil {
		return nil, err
	}

	uniqueIDs := sliceutil.UniquePositive(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: op, Err: ErrInvalidData}
	}

	var supervisedGroups map[int64]struct{}
	if auth != nil && !auth.BypassResourceChecks {
		supervisedGroups, err = s.loadMoveSupervisedGroupIDs(ctx, auth.StaffID, op)
		if err != nil {
			return nil, err
		}
		if _, ok := supervisedGroups[targetGroup.ID]; !ok {
			return nil, studentMoveForbidden(op)
		}
	}
	if s.GetPresenceMode(ctx) == "binary" {
		result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
		result.Unchanged = append(result.Unchanged, uniqueIDs...)
		return result, nil
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, op)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeMoveStudents(ctx, op, auth, supervisedGroups, uniqueIDs, openAttendance, currentVisits, true); err != nil {
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

		if currentVisit != nil {
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

	var supervisedGroups map[int64]struct{}
	var err error
	if auth != nil && !auth.BypassResourceChecks {
		supervisedGroups, err = s.loadMoveSupervisedGroupIDs(ctx, auth.StaffID, op)
		if err != nil {
			return nil, err
		}
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
	if err := s.authorizeMoveStudents(ctx, op, auth, supervisedGroups, uniqueIDs, openAttendance, currentVisits, false); err != nil {
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
		Moved:         []int64{},
		Unchanged:     []int64{},
		Skipped:       []StudentMoveSkipped{},
		ActiveGroupID: activeGroupID,
		RoomID:        roomID,
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

func (s *service) loadMoveSupervisedGroupIDs(ctx context.Context, staffID int64, op string) (map[int64]struct{}, error) {
	if staffID <= 0 {
		return nil, studentMoveForbidden(op)
	}

	supervisions, err := s.GetStaffActiveSupervisions(ctx, staffID)
	if err != nil {
		return nil, err
	}

	ids := make(map[int64]struct{}, len(supervisions))
	for _, supervision := range supervisions {
		if supervision == nil || supervision.GroupID <= 0 {
			continue
		}
		ids[supervision.GroupID] = struct{}{}
	}
	return ids, nil
}

func (s *service) authorizeMoveStudents(
	ctx context.Context,
	op string,
	auth *StudentMoveAuthorization,
	supervisedGroups map[int64]struct{},
	studentIDs []int64,
	openAttendance map[int64]*active.Attendance,
	currentVisits map[int64]*active.Visit,
	allowOpenTransit bool,
) error {
	if auth == nil || auth.BypassResourceChecks {
		return nil
	}

	for _, studentID := range studentIDs {
		hasTeacherAccess, err := s.CheckTeacherStudentAccess(ctx, auth.StaffID, studentID)
		if err != nil {
			return err
		}
		if hasTeacherAccess {
			continue
		}

		currentVisit := currentVisits[studentID]
		if currentVisit != nil {
			if _, ok := supervisedGroups[currentVisit.ActiveGroupID]; ok {
				continue
			}
			return studentMoveForbidden(op)
		}

		if allowOpenTransit && studentHasOpenAttendance(openAttendance, studentID) {
			continue
		}

		return studentMoveForbidden(op)
	}

	return nil
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
