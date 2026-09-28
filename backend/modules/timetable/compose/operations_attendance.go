package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *operations) CheckInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	roster, err := s.checkInStudent(ctx, accountID, isAdmin, instanceID, studentID)
	return s.rosterWithActionAccess(ctx, accountID, isAdmin, instanceID, roster, err)
}

func (s *operations) checkInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	staffID, err := s.requireScopedAction(ctx, accountID, isAdmin, instanceID, ScopedAttendance)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, timetable.ErrNoStaffProfile
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.Status != scheduleModels.InstanceStatusActive || inst.ActiveGroupID == nil {
		return nil, fmt.Errorf("%w: instance is not active", timetable.ErrTimetableOperationConflict)
	}
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
	}
	current, err := s.currentVisit(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return s.checkInStudentWithCurrentVisit(ctx, staffID, inst, instanceID, studentID, current)
	}
	now := s.now()
	if roster, handled, err := s.createCheckInVisit(ctx, staffID, inst, instanceID, studentID, now); handled {
		return roster, err
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	if err := s.deps.Sessions.UpdateLastActivity(ctx, *inst.ActiveGroupID, now); err != nil {
		s.logger().WarnContext(ctx, "failed to update active group activity after timetable check-in",
			slog.Int64("active_group_id", *inst.ActiveGroupID),
			slog.String("error", err.Error()))
	}
	return s.buildRoster(ctx, instanceID)
}

// createCheckInVisit opens the child's visit in the block's session,
// attributed to the acting staff member. Inside a request transaction it
// runs in a savepoint, so a child that meanwhile checked in elsewhere is
// resolved without aborting the transaction. handled reports that the check-in
// already produced its answer (the concurrent visit path or a failure).
func (s *operations) createCheckInVisit(ctx context.Context, staffID int64, inst *scheduleModels.ActivityInstance, instanceID, studentID int64, now time.Time) (*timetable.OperationRoster, bool, error) {
	visit := &studentpresence.Visit{StudentID: studentID, ActiveGroupID: *inst.ActiveGroupID, EntryTime: now}
	visit.TenantID = tenant.FromContext(ctx)
	var createErr error
	if _, inTx := tenant.TransactionFromContext(ctx); inTx {
		createErr = tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
			return s.deps.Attendance.CreateVisitAs(savepointCtx, staffID, visit)
		})
	} else {
		createErr = s.deps.Attendance.CreateVisitAs(ctx, staffID, visit)
	}
	if createErr == nil {
		return nil, false, nil
	}
	if errors.Is(createErr, tenant.ErrSavepointControl) || !errors.Is(createErr, studentpresence.ErrStudentAlreadyActive) {
		return nil, true, createErr
	}
	current, err := s.currentVisit(ctx, studentID)
	if err != nil {
		return nil, true, err
	}
	if current == nil {
		return nil, true, createErr
	}
	roster, err := s.checkInStudentWithCurrentVisit(ctx, staffID, inst, instanceID, studentID, current)
	return roster, true, err
}

func (s *operations) checkInStudentWithCurrentVisit(ctx context.Context, staffID int64, inst *scheduleModels.ActivityInstance, instanceID, studentID int64, current *studentpresence.Visit) (*timetable.OperationRoster, error) {
	if current.ActiveGroupID != *inst.ActiveGroupID {
		return s.moveStudentFromOtherSession(ctx, staffID, inst, instanceID, studentID)
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

// moveStudentFromOtherSession resolves the "child is still present in
// another running session" check-in by moving the child (#2386). The shared
// bulk move owns checkout semantics, attendance mirroring and broadcasts;
// the target was authorized already, so the move's own check is bypassed.
func (s *operations) moveStudentFromOtherSession(ctx context.Context, staffID int64, inst *scheduleModels.ActivityInstance, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	result, err := s.deps.Attendance.MoveStudentsToActiveGroupAuthorized(ctx, []int64{studentID}, *inst.ActiveGroupID, studentpresence.StudentMoveAuthorization{
		StaffID:              staffID,
		BypassResourceChecks: true,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Moved) == 0 && len(result.Unchanged) == 0 {
		tenant.MarkRollback(ctx)
		return nil, fmt.Errorf("%w: student could not be moved from other session", timetable.ErrTimetableOperationConflict)
	}
	if err := s.markPlannedStudentPresent(ctx, instanceID, studentID); err != nil {
		return nil, err
	}
	roster, err := s.buildRoster(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if len(result.Moved) > 0 {
		movedFrom := ""
		if previousActiveGroupID := result.PreviousActiveGroupIDs[studentID]; previousActiveGroupID > 0 {
			movedFrom = s.resolveActiveGroupLabel(ctx, previousActiveGroupID)
		}
		roster.MovedFrom = &movedFrom
	}
	return roster, nil
}

// resolveActiveGroupLabel names a running session for the move notice: the
// block's title, else the session's activity name, else its room. Purely
// cosmetic — every failure degrades to an empty label.
func (s *operations) resolveActiveGroupLabel(ctx context.Context, activeGroupID int64) string {
	if inst, err := s.deps.Instances.FindByActiveGroupID(ctx, activeGroupID); err == nil && inst != nil {
		return inst.Title
	}
	group, err := s.deps.Sessions.FindSession(ctx, activeGroupID)
	if err != nil || group == nil {
		return ""
	}
	if group.ActivityGroupID != nil {
		if activityGroup, err := s.deps.Templates.FindByID(ctx, *group.ActivityGroupID); err == nil && activityGroup != nil {
			return activityGroup.Name
		}
	}
	if name, ok, err := s.deps.Rooms.RoomName(ctx, group.RoomID); err == nil && ok {
		return name
	}
	return ""
}

func (s *operations) markPlannedStudentPresent(ctx context.Context, instanceID, studentID int64) error {
	row, err := s.deps.Participants.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil || row == nil {
		return err
	}
	status := scheduleModels.AttendanceStatusPresent
	return s.deps.Participants.UpdateAttendanceFields(ctx, row.ID, scheduleModels.AttendanceFieldPatch{
		Status:         &status,
		SubstatusClear: true,
		NoteClear:      true,
	})
}

func (s *operations) CheckOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	roster, err := s.checkOutStudent(ctx, accountID, isAdmin, instanceID, studentID)
	return s.rosterWithActionAccess(ctx, accountID, isAdmin, instanceID, roster, err)
}

func (s *operations) checkOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	staffID, err := s.requireScopedAction(ctx, accountID, isAdmin, instanceID, ScopedAttendance)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, timetable.ErrNoStaffProfile
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.ActiveGroupID == nil {
		return nil, fmt.Errorf("%w: instance has no active group", timetable.ErrTimetableOperationConflict)
	}
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
	}
	visit, err := s.findActiveVisitForInstanceStudent(ctx, *inst.ActiveGroupID, studentID)
	if err != nil {
		return nil, err
	}
	if visit == nil {
		return nil, timetable.ErrTimetableOperationNotFound
	}
	if err := s.deps.Attendance.EndVisitAs(ctx, staffID, visit.TenantID, visit.ID); err != nil && !errors.Is(err, studentpresence.ErrVisitAlreadyEnded) {
		return nil, err
	}
	return s.buildRoster(ctx, instanceID)
}

// requireRosterStudent bounds a per-child write of an assignment-bound
// portal to the children the block holds (#2527): a planned row or a child
// currently present in the running session, minus the ones the roster
// excludes. An OGS supervisor may pull a walk-in off the tenant directory; a
// Lehrkraft's whole reach is the block she was planned into.
func (s *operations) requireRosterStudent(ctx context.Context, inst *scheduleModels.ActivityInstance, instanceID, studentID int64) error {
	if !isAssignmentBoundPortal(ctx) {
		return nil
	}
	onRoster, err := s.blockHoldsStudent(ctx, inst, instanceID, studentID)
	if err != nil {
		return err
	}
	if !onRoster {
		return timetable.ErrTimetableOperationForbidden
	}
	excluded, err := s.rosterStudentExcluded(ctx, inst, studentID)
	if err != nil {
		return err
	}
	if excluded {
		return timetable.ErrTimetableOperationForbidden
	}
	return nil
}

// blockHoldsStudent is the same union the roster renders: a planned row, or
// a visit in the running session.
func (s *operations) blockHoldsStudent(ctx context.Context, inst *scheduleModels.ActivityInstance, instanceID, studentID int64) (bool, error) {
	planned, err := s.deps.Participants.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return false, err
	}
	if _, ok := findPlanned(planned, studentID); ok {
		return true, nil
	}
	if inst == nil || inst.ActiveGroupID == nil {
		return false, nil
	}
	visits, err := s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{*inst.ActiveGroupID}})
	if err != nil {
		return false, err
	}
	for _, visit := range visits {
		if visit.StudentID == studentID {
			return true, nil
		}
	}
	return false, nil
}

// rosterStudentExcluded applies the roster's current exclusion before an
// assignment-bound portal writes a child: a planned row kept as history after
// graduation or care end authorizes nothing any more.
func (s *operations) rosterStudentExcluded(ctx context.Context, inst *scheduleModels.ActivityInstance, studentID int64) (bool, error) {
	students, err := s.deps.Students.FindByIDs(ctx, []int64{studentID})
	if err != nil {
		return false, err
	}
	return rosterExcludedAlumni(inst, students, s.today())[studentID], nil
}

func (s *operations) currentVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error) {
	visits, err := s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{studentID}, OpenOnly: true, NewestFirst: true, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(visits) == 0 {
		return nil, nil
	}
	return &visits[0], nil
}

func (s *operations) findActiveVisitForInstanceStudent(ctx context.Context, activeGroupID, studentID int64) (*studentpresence.Visit, error) {
	visits, err := s.deps.Visits.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{activeGroupID}})
	if err != nil {
		return nil, err
	}
	for _, visit := range visits {
		if visit.StudentID == studentID && visit.ExitTime == nil {
			return &visit, nil
		}
	}
	return nil, nil
}
