package compose

import (
	"context"
	"fmt"
	"log/slog"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PatchAttendance applies a manual attendance decision to one child of a
// block that has not ended. Reporting sick or excused needs the absence
// grant, every other decision the block's operation rights.
func (s *operations) PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch timetable.AttendancePatch) (*timetable.OperationRosterRow, error) {
	inst, err := s.loadOpenBlock(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRosterStudent(ctx, inst, instanceID, studentID); err != nil {
		return nil, err
	}
	if err := s.lockOpenBlockAttendance(ctx, instanceID); err != nil {
		return nil, err
	}
	row, err := s.deps.Participants.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, timetable.ErrTimetableOperationNotFound
	}
	if err := s.authorizeAttendancePatch(ctx, accountID, isAdmin, instanceID, patch, row); err != nil {
		return nil, err
	}
	current := timetable.SlotAttendance{Status: row.Status, Substatus: row.Substatus}
	if verrs := timetable.ValidateAttendancePatch(patch, current); len(verrs) > 0 {
		return nil, &timetable.AttendanceValidationError{Fields: verrs}
	}
	if err := s.deps.Participants.UpdateAttendanceFields(ctx, row.ID, attendanceFieldPatch(patch)); err != nil {
		return nil, err
	}
	s.broadcastAttendanceChanged(ctx, instanceID)
	return s.rosterRowOf(ctx, instanceID, studentID)
}

// lockOpenBlockAttendance serializes a write with a concurrent completion:
// it re-reads the block under the attendance lock. A composition without
// locks skips it.
func (s *operations) lockOpenBlockAttendance(ctx context.Context, instanceID int64) error {
	if s.deps.Locks == nil {
		return nil
	}
	if err := s.deps.Locks.LockAttendance(ctx, instanceID); err != nil {
		return err
	}
	_, err := s.loadOpenBlock(ctx, instanceID)
	return err
}

// rosterRowOf rebuilds the block's roster and returns the child's row.
func (s *operations) rosterRowOf(ctx context.Context, instanceID, studentID int64) (*timetable.OperationRosterRow, error) {
	roster, err := s.buildRoster(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	for i := range roster.Rows {
		if roster.Rows[i].StudentID == studentID {
			return &roster.Rows[i], nil
		}
	}
	return nil, timetable.ErrTimetableOperationNotFound
}

// loadOpenBlock loads a block whose attendance may still change: completed
// and cancelled blocks are frozen.
func (s *operations) loadOpenBlock(ctx context.Context, instanceID int64) (*scheduleModels.ActivityInstance, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if inst.Status == scheduleModels.InstanceStatusCompleted || inst.Status == scheduleModels.InstanceStatusCancelled {
		return nil, fmt.Errorf("%w: attendance is frozen after completion", timetable.ErrTimetableOperationConflict)
	}
	return inst, nil
}

// authorizeAttendancePatch picks the grant a patch needs. Only reporting
// sick/excused and returning such a row to expected use the absence grant;
// present, unexplained absence and other substatuses keep the block's
// operation rights, even when they replace an excusal. Touching a sick or
// excused marker also needs the direct-absence scope.
func (s *operations) authorizeAttendancePatch(ctx context.Context, accountID int64, isAdmin bool, instanceID int64, patch timetable.AttendancePatch, row *scheduleModels.InstanceStudent) error {
	if isAbsenceOnlyAttendancePatch(patch, row) {
		if err := s.requireCanReportAbsence(ctx, accountID, isAdmin, instanceID); err != nil {
			return err
		}
	} else if _, err := s.requireCanOperate(ctx, accountID, isAdmin, instanceID); err != nil {
		return err
	}
	if isReportableAbsenceSubstatus(patch.Substatus) || isReportableAbsenceSubstatus(row.Substatus) {
		return s.requireDirectAbsenceScope(ctx, isAdmin)
	}
	return nil
}

func isReportableAbsenceSubstatus(substatus *string) bool {
	return substatus != nil && (*substatus == scheduleModels.AttendanceSubstatusSick || *substatus == scheduleModels.AttendanceSubstatusExcused)
}

func isAbsenceOnlyAttendancePatch(patch timetable.AttendancePatch, row *scheduleModels.InstanceStudent) bool {
	status, substatus := row.Status, row.Substatus
	if patch.Status != nil {
		status = *patch.Status
	}
	if patch.SubstatusClear {
		substatus = nil
	} else if patch.Substatus != nil {
		substatus = patch.Substatus
	}
	return (status == scheduleModels.AttendanceStatusAbsent && isReportableAbsenceSubstatus(substatus)) ||
		(status == scheduleModels.AttendanceStatusExpected && substatus == nil && isReportableAbsenceSubstatus(row.Substatus))
}

// attendanceFieldPatch is the retained row patch of an attendance decision.
func attendanceFieldPatch(patch timetable.AttendancePatch) scheduleModels.AttendanceFieldPatch {
	return scheduleModels.AttendanceFieldPatch{
		Status: patch.Status, Substatus: patch.Substatus, SubstatusClear: patch.SubstatusClear,
		Note: patch.Note, NoteClear: patch.NoteClear,
	}
}

// broadcastAttendanceChanged wakes the tenant's clients after the commit of
// an attendance patch; the event names the session, never the child
// (#2085). Clients refetch the views their permissions allow.
func (s *operations) broadcastAttendanceChanged(ctx context.Context, instanceID int64) {
	if s.deps.Announcer == nil {
		return
	}
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		s.logger().WarnContext(ctx, "failed to load instance for timetable attendance broadcast",
			slog.Int64("instance_id", instanceID),
			slog.String("error", err.Error()))
		return
	}
	if inst.ActiveGroupID == nil {
		return
	}
	activeGroupID := *inst.ActiveGroupID
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if err := s.deps.Announcer.AnnounceAttendanceChanged(tenantID, activeGroupID, instanceID); err != nil {
			s.logger().WarnContext(ctx, "SSE timetable attendance broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("active_group_id", fmt.Sprintf("%d", activeGroupID)),
				slog.String("error", err.Error()))
		}
	})
}
