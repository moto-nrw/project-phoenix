package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

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
	openRoomTarget bool,
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
	if openRoomTarget {
		// A released room is a shared destination (#3066): taking supervision
		// there is not a prerequisite, and its room session legitimately runs
		// beside activity sessions, so the push rule's "one supervised session"
		// check does not apply. Every source-side right above still does.
		return nil
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
	scope, err := s.settings.AttendanceEditScope(ctx)
	if err != nil {
		return false, fmt.Errorf("resolve attendance edit scope: %w", err)
	}
	if scope == AttendanceEditScopeOwn {
		return false, nil
	}
	if scope != AttendanceEditScopeAllStaff {
		return false, ErrStudentMoveForbidden
	}
	visibility, err := s.settings.OperationalOverviewScope(ctx)
	if err != nil {
		return false, fmt.Errorf("resolve attendance visibility: %w", err)
	}
	if visibility != OverviewScopeAllStaff || staffID <= 0 || tenant.FromContext(ctx) <= 0 {
		return false, ErrStudentMoveForbidden
	}
	staffTenantID, err := s.StaffRepo.StaffTenantID(ctx, staffID)
	if modelBase.IsNoRows(err) {
		return false, ErrStudentMoveForbidden
	}
	if err != nil {
		return false, err
	}
	if staffTenantID == nil || *staffTenantID != tenant.FromContext(ctx) {
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
