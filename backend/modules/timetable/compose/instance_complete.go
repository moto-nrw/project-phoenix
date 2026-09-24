package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// reopenWindow is how long a completed block may be reopened.
const reopenWindow = 5 * time.Minute

func (s *InstanceLifecycleService) validateCompleteTime(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time) error {
	if !s.deps.EnforceTimePolicy || instance.IsSpontaneous {
		return nil
	}
	enforce, err := s.deps.Settings.EnforcePlannedEnd(ctx)
	if err != nil {
		return fmt.Errorf("%w: resolve planned end policy: %v", timetable.ErrLifecycleSettings, err)
	}
	availability := evaluateLifecycleAvailability(instance, now, 0, enforce)
	if !availability.CanComplete {
		return fmt.Errorf("%w: available at %s", timetable.ErrInstanceCompleteEarly,
			timetable.LifecycleBoundary(timezone.Date(instance.Date), instance.EndTime).Format(time.RFC3339))
	}
	return nil
}

// Complete implements active → completed. The session is ended the way a
// session end always is — open visits close, supervisors end, checkout
// events fire — and the live state is snapshotted first, so Reopen can
// restore exactly what the completion closed.
func (s *InstanceLifecycleService) Complete(ctx context.Context, instanceID int64) (*timetable.LifecycleInstance, error) {
	instance, err := s.complete(ctx, instanceID)
	return LifecycleInstanceOf(instance), err
}

func (s *InstanceLifecycleService) complete(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	if !s.hasTx(ctx) {
		var result *scheduleModel.ActivityInstance
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var completeErr error
			result, completeErr = s.complete(txCtx, instanceID)
			return completeErr
		})
		return result, err
	}
	instance, err := s.completableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.lockCompletionState(ctx, instance); err != nil {
		return nil, err
	}
	if err := s.snapshotCompletion(ctx, instance); err != nil {
		return nil, err
	}
	if err := s.stampRemainingAbsent(ctx, instance); err != nil {
		return nil, err
	}
	if err := s.deps.ActiveService.EndActivitySession(ctx, *instance.ActiveGroupID); err != nil {
		return nil, &ScheduleError{Op: "complete instance: end active.group", Err: err}
	}
	completedAt, err := s.deps.RecoveryRepo.CompletionTimestamp(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: read transaction timestamp", Err: err}
	}
	instance.MarkCompleted(completedAt, completedAt.Add(reopenWindow), timetable.LifecycleActor(ctx))
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at", "completed_by", "reopen_until", "completion_snapshot"); err != nil {
		return nil, &ScheduleError{Op: "complete instance: update", Err: err}
	}
	s.broadcastInstanceEvent(ctx, LifecycleEventInstanceCompleted, instance, nil, nil)
	return instance, nil
}

// completableInstance loads a running block under its day lock. The
// day-wide staffing saves take the same lock before they rewrite staff rows
// and open supervisors, so without it a deviation save could commit
// staffing onto an already completed block (#1840).
func (s *InstanceLifecycleService) completableInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, err = s.lockDayAndReload(ctx, instance, "complete instance")
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusActive {
		return nil, fmt.Errorf("%w: cannot complete instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	if err := s.validateCompleteTime(ctx, instance, s.now()); err != nil {
		return nil, err
	}
	if instance.ActiveGroupID == nil {
		// A running block without a session is data corruption; abort rather
		// than "complete" a block that never ran.
		return nil, &ScheduleError{Op: "complete instance", Err: fmt.Errorf("active instance %d has no active_group_id", instance.ID)}
	}
	return instance, nil
}

// lockCompletionState serializes check-in (which locks the group) and
// checkout (visit row locks) with the snapshot, so Reopen restores the rows
// the session end actually closes.
func (s *InstanceLifecycleService) lockCompletionState(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	lockedGroup, err := s.deps.ActiveGroupRepo.FindByIDForUpdate(ctx, *instance.ActiveGroupID)
	if err != nil {
		return &ScheduleError{Op: "complete instance: lock group", Err: err}
	}
	if lockedGroup == nil || lockedGroup.EndTime != nil {
		return fmt.Errorf("%w: active group is not open", timetable.ErrInvalidInstanceTransition)
	}
	if s.deps.RecoveryRepo == nil {
		return nil
	}
	if err := s.deps.RecoveryRepo.LockOpenVisits(ctx, *instance.ActiveGroupID); err != nil {
		return &ScheduleError{Op: "complete instance: lock visits", Err: err}
	}
	if err := s.deps.RecoveryRepo.LockOpenSupervisors(ctx, *instance.ActiveGroupID); err != nil {
		return &ScheduleError{Op: "complete instance: lock supervisors", Err: err}
	}
	if err := s.deps.RecoveryRepo.LockAttendance(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "complete instance: lock attendance", Err: err}
	}
	return nil
}

// snapshotCompletion records the open visits, the open supervisors and the
// attendance rows on the block before the session ends. A confirmed roster
// (WithCompletionConfirmation) must match the children still checked in.
func (s *InstanceLifecycleService) snapshotCompletion(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	activeGroupID := *instance.ActiveGroupID
	visitsBefore, err := s.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{activeGroupID}})
	if err != nil {
		return &ScheduleError{Op: "complete instance: snapshot visits", Err: err}
	}
	supervisorsBefore, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, activeGroupID, true)
	if err != nil {
		return &ScheduleError{Op: "complete instance: snapshot supervisors", Err: err}
	}
	attendanceBefore, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "complete instance: snapshot attendance", Err: err}
	}
	snapshot := scheduleModel.ActivityCompletionSnapshot{ActiveGroupID: activeGroupID}
	openStudentIDs := make([]int64, 0, len(visitsBefore))
	for _, visit := range visitsBefore {
		if visit.ExitTime == nil {
			snapshot.VisitIDs = append(snapshot.VisitIDs, visit.ID)
			openStudentIDs = append(openStudentIDs, visit.StudentID)
		}
	}
	if confirmed, required := timetable.CompletionConfirmation(ctx); required {
		slices.Sort(confirmed)
		slices.Sort(openStudentIDs)
		if !slices.Equal(confirmed, openStudentIDs) {
			return timetable.ErrCompletionConfirmationStale
		}
	}
	for _, supervisor := range supervisorsBefore {
		snapshot.SupervisorIDs = append(snapshot.SupervisorIDs, supervisor.ID)
	}
	for _, row := range attendanceBefore {
		snapshot.Attendance = append(snapshot.Attendance, completionAttendanceOf(row))
	}
	instance.CompletionSnapshot, err = json.Marshal(snapshot)
	if err != nil {
		return &ScheduleError{Op: "complete instance: encode snapshot", Err: err}
	}
	return nil
}

func completionAttendanceOf(row *scheduleModel.InstanceStudent) scheduleModel.CompletionAttendanceSnapshot {
	return scheduleModel.CompletionAttendanceSnapshot{
		RowID:              row.ID,
		Status:             row.Status,
		Substatus:          row.Substatus,
		Note:               row.Note,
		CheckedInAt:        row.CheckedInAt,
		CheckedOutAt:       row.CheckedOutAt,
		NotScheduled:       row.NotScheduled,
		StudentStatusDayID: row.StudentStatusDayID,
		PickupExceptionID:  row.PickupExceptionID,
	}
}

// stampRemainingAbsent marks the still-expected children absent before the
// session ends, in the caller's tenant tx. Children not booked into care
// that day are spared (#1747): "absent" would claim they missed care they
// were never booked for. WHY they are spared is persisted first, so later
// writers of the status cannot silently create or destroy that fact.
func (s *InstanceLifecycleService) stampRemainingAbsent(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	notScheduled, err := s.notScheduledStudentIDs(ctx, instance)
	if err != nil {
		return err
	}
	refs := make([]scheduleModel.StudentInstanceRef, 0, len(notScheduled))
	for _, studentID := range notScheduled {
		refs = append(refs, scheduleModel.StudentInstanceRef{StudentID: studentID, InstanceID: instance.ID})
	}
	if err := s.deps.InstanceStudents.MarkNotScheduled(ctx, refs); err != nil {
		return &ScheduleError{Op: "complete instance: mark not scheduled", Err: err}
	}
	if _, err := s.deps.InstanceStudents.BulkUpdateStatus(
		ctx, instance.ID, scheduleModel.AttendanceStatusExpected, scheduleModel.AttendanceStatusAbsent, notScheduled,
	); err != nil {
		return &ScheduleError{Op: "complete instance: mark absent", Err: err}
	}
	return nil
}

// notScheduledStudentIDs returns the block's children who are not booked
// into care at all on its date (#1747). A child whose day was cancelled is
// not among them — that is a reported absence and must be written — and
// neither is a child whose row somebody set by hand (ManualStatusAt): that
// decision outranks the derivation (#1747 review).
func (s *InstanceLifecycleService) notScheduledStudentIDs(ctx context.Context, instance *scheduleModel.ActivityInstance) ([]int64, error) {
	rows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: load attendance rows", Err: err}
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ManualStatusAt == nil {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	if len(studentIDs) == 0 {
		return nil, nil
	}
	careDay, err := s.deps.CareDays.ResolveForDate(ctx, studentIDs, timezone.Date(instance.Date))
	if err != nil {
		return nil, &ScheduleError{Op: "complete instance: resolve care day", Err: err}
	}
	notScheduled := make([]int64, 0)
	for _, studentID := range studentIDs {
		if s.deps.CareDays.ExemptFromAbsence(careDay[studentID]) {
			notScheduled = append(notScheduled, studentID)
		}
	}
	return notScheduled, nil
}
