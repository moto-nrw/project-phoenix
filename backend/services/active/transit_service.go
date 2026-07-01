package active

import (
	"context"
	"database/sql"
	"errors"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
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
	openAttendanceIDs, err := s.attendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
	if err != nil {
		return nil, &ActiveError{Op: "ListStudentsInTransit", Err: ErrDatabaseOperation}
	}
	if len(openAttendanceIDs) == 0 {
		return []int64{}, nil
	}

	currentVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, openAttendanceIDs)
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
	ids, err := s.attendanceRepo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())
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

	uniqueIDs := uniquePositiveInt64s(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrInvalidData}
	}

	openAttendance, err := s.attendanceRepo.GetOpenTodayByStudentIDsForUpdate(ctx, uniqueIDs)
	if err != nil {
		return nil, &ActiveError{Op: "AssignTransitStudentsToActiveGroup", Err: ErrDatabaseOperation}
	}
	currentVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, uniqueIDs)
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

// MoveStudentsToActiveGroup moves checked-in students into the target active
// group while preserving room history: any old open visit is ended and a new
// visit opens in the target group. It never creates or reopens daily
// attendance; not-present students are reported as skipped.
func (s *service) MoveStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*StudentMoveResult, error) {
	if activeGroupID <= 0 || len(studentIDs) == 0 {
		return nil, &ActiveError{Op: "MoveStudentsToActiveGroup", Err: ErrInvalidData}
	}

	targetGroup, err := s.lockActiveGroupForMove(ctx, activeGroupID, "MoveStudentsToActiveGroup")
	if err != nil {
		return nil, err
	}

	uniqueIDs := uniquePositiveInt64s(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "MoveStudentsToActiveGroup", Err: ErrInvalidData}
	}
	if s.GetPresenceMode(ctx) == "binary" {
		result := newStudentMoveResult(&targetGroup.ID, &targetGroup.RoomID)
		result.Unchanged = append(result.Unchanged, uniqueIDs...)
		return result, nil
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, "MoveStudentsToActiveGroup")
	if err != nil {
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
		return nil, &ActiveError{Op: "MoveStudentsToActiveGroup", Err: ErrStudentsNotPresent}
	}

	return result, nil
}

// MoveStudentsToTransit ends any current room visit for checked-in students
// while keeping their daily attendance open.
func (s *service) MoveStudentsToTransit(ctx context.Context, studentIDs []int64) (*StudentMoveResult, error) {
	if len(studentIDs) == 0 {
		return nil, &ActiveError{Op: "MoveStudentsToTransit", Err: ErrInvalidData}
	}

	uniqueIDs := uniquePositiveInt64s(studentIDs)
	if len(uniqueIDs) == 0 {
		return nil, &ActiveError{Op: "MoveStudentsToTransit", Err: ErrInvalidData}
	}
	if s.GetPresenceMode(ctx) == "binary" {
		result := newStudentMoveResult(nil, nil)
		result.Unchanged = append(result.Unchanged, uniqueIDs...)
		return result, nil
	}

	openAttendance, currentVisits, err := s.loadMoveState(ctx, uniqueIDs, "MoveStudentsToTransit")
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
		return nil, &ActiveError{Op: "MoveStudentsToTransit", Err: ErrStudentsNotPresent}
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
	openAttendance, err := s.attendanceRepo.GetOpenTodayByStudentIDsForUpdate(ctx, studentIDs)
	if err != nil {
		return nil, nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}

	currentVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, studentIDs)
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

func (s *service) lockActiveGroupForMove(ctx context.Context, activeGroupID int64, op string) (*active.Group, error) {
	group := new(active.Group)
	query := repoBase.GetDB(ctx, s.db).NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, activeGroupID).
		For("UPDATE")

	if where, val, ok := repoBase.TenantWhere(ctx, "group"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActiveError{Op: op, Err: ErrActiveGroupNotFound}
		}
		return nil, &ActiveError{Op: op, Err: ErrDatabaseOperation}
	}
	if group == nil || !group.IsActive() {
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
	if err := s.visitRepo.Create(ctx, visit); err != nil {
		if isDuplicateActiveVisitViolation(err) {
			return &ActiveError{Op: "CreateMoveVisit", Err: ErrStudentAlreadyActive}
		}
		return &ActiveError{Op: "CreateMoveVisit", Err: ErrDatabaseOperation}
	}

	var snapshot *AttendanceSnapshot
	if s.attendanceSyncer != nil {
		snapshot = s.attendanceSyncer.MirrorCheckInForVisit(ctx, visit)
	}
	s.broadcastVisitCreated(ctx, visit, snapshot)
	return nil
}

func uniquePositiveInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
