package compose

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// movedSlotReason marks the exception a move leaves on the vacated slot, so
// admins can tell it apart from manual cancellations.
const movedSlotReason = "Einzeltermin verschoben"

// UpdatePlanned replaces the planning fields and the roster of a planned
// block. It may move the block to another date.
func (s *InstanceLifecycleService) UpdatePlanned(ctx context.Context, instanceID int64, req timetable.UpdateInstanceInput, actorAccountID *int64) (*timetable.LifecycleInstance, error) {
	if !s.hasTx(ctx) {
		var updated *timetable.LifecycleInstance
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			var err error
			updated, err = s.UpdatePlanned(txCtx, instanceID, req, actorAccountID)
			return err
		})
		return updated, err
	}
	// Both tenant gates BEFORE the day locks: the edit rewrites the roster,
	// so it must not interleave with a grade transition, and the
	// recurrence-first order keeps the acquisition acyclic.
	if err := s.lockRecurrenceThenGradeTransitions(ctx, "update instance"); err != nil {
		return nil, err
	}
	instance, err := s.editableInstance(ctx, instanceID, req)
	if err != nil {
		return nil, err
	}
	// The slot the block occupied before the edit: a moved template-backed
	// occurrence vacates it, and consumeMovedSlot keeps the materializer
	// from recreating it next to the moved one.
	origSlot := capturedSlot{
		ActivityGroupID: instance.ActivityGroupID,
		Date:            timezone.Date(instance.Date),
		StartHHMMSS:     formatTimeOfDay(instance.StartTime),
	}
	if err := s.updateLifecycleColumns(ctx, instance, applyPlannedEdit(instance, req)...); err != nil {
		return nil, &ScheduleError{Op: "update instance", Err: duplicateTemplateInstance(err)}
	}
	if err := s.consumeMovedSlot(ctx, origSlot, req); err != nil {
		return nil, err
	}
	if err := s.replaceInstanceAssignments(ctx, instance, req.StaffIDs, req.StudentIDs, actorAccountID); err != nil {
		return nil, err
	}
	// A lingering acknowledgement is cleared only when the edit left the
	// block fully staffed; an unrelated edit must not reopen an
	// acknowledged gap (#1840).
	if err := s.clearStaleAckIfStaffed(ctx, instance, actorAccountID); err != nil {
		return nil, err
	}
	s.broadcastPlannedInstanceChanged(ctx, "instance_update")
	return LifecycleInstanceOf(instance), nil
}

// editableInstance loads a planned block and takes the day locks of its
// current and (when moved) new date in ascending order: the edit rewrites
// the assignments the day-wide staffing saves write (#1840). The block is
// re-read and re-validated under the locks, because a concurrent
// transition or move may have committed while this call waited.
func (s *InstanceLifecycleService) editableInstance(ctx context.Context, instanceID int64, req timetable.UpdateInstanceInput) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot update instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	lockedDate := instance.Date
	if err := s.acquireSubstituteDayLockPair(ctx, tenant.FromContext(ctx), timezone.Date(lockedDate), req.Date); err != nil {
		return nil, &ScheduleError{Op: "update instance: lock day", Err: err}
	}
	instance, err = s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Date != lockedDate {
		return nil, timetable.ErrInstanceMoved
	}
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return nil, fmt.Errorf("%w: cannot update instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	if err := validateLegacyWeekendInstanceDate(timezone.Date(instance.Date), req.Date); err != nil {
		return nil, err
	}
	if req.CalendarPeriodID != nil || (timezone.Date(instance.Date) != req.Date && !instance.IsSpontaneous) {
		if err := s.validateInstanceDateInActiveCalendarPeriod(ctx, req.Date); err != nil {
			return nil, &ScheduleError{Op: "update instance: validate calendar period", Err: err}
		}
	}
	if err := s.validateInstanceReferences(ctx, req.Date, instanceReferences{
		roomID: req.RoomID, activityGroupID: req.ActivityGroupID, staffIDs: req.StaffIDs, studentIDs: req.StudentIDs,
	}); err != nil {
		return nil, &ScheduleError{Op: "update instance: validate references", Err: err}
	}
	return instance, nil
}

// applyPlannedEdit writes the request onto the row and names the columns
// to persist. is_spontaneous records the creation origin (#2299) and never
// follows the offering link; only the series conversion (the one caller
// with a CalendarPeriodID) stamps the full materializer marker.
func applyPlannedEdit(instance *scheduleModel.ActivityInstance, req timetable.UpdateInstanceInput) []string {
	instance.Date = scheduleModel.Date(req.Date)
	instance.StartTime = req.StartTime
	instance.EndTime = req.EndTime
	instance.Title = req.Title
	instance.Description = req.Description
	instance.Notes = req.Notes
	instance.RoomID = req.RoomID
	instance.ActivityGroupID = req.ActivityGroupID
	instance.RequiredStaff = req.RequiredStaff
	instance.ListKind = req.ListKind
	columns := []string{
		"date", "start_time", "end_time", "title", "description", "notes",
		"room_id", "activity_group_id", "required_staff", "list_kind",
	}
	if req.CalendarPeriodID != nil {
		instance.CalendarPeriodID = req.CalendarPeriodID
		instance.IsSpontaneous = false
		columns = append(columns, "calendar_period_id", "is_spontaneous")
	}
	return columns
}

func validateLegacyWeekendInstanceDate(existing, requested timezone.Date) error {
	if requested.Weekday() != time.Saturday && requested.Weekday() != time.Sunday {
		return nil
	}
	if existing == requested {
		return nil
	}
	return timetable.ErrInstanceWeekend
}

func (s *InstanceLifecycleService) validateInstanceDateInActiveCalendarPeriod(ctx context.Context, date timezone.Date) error {
	periods, err := s.deps.CalendarPeriodRepo.FindActiveByTenantID(ctx)
	if err != nil {
		return fmt.Errorf("find active calendar periods: %w", err)
	}
	for _, period := range periods {
		if period.ContainsDay(scheduleModel.Date(date)) {
			return nil
		}
	}
	return timetable.ErrInstanceOutsideActiveCalendarPeriod
}

// replaceInstanceAssignments wipes and re-creates the block's staff and
// student rows from the request lists (#1840). The editor sends plain
// staff ids, so two kinds of row metadata are carried differently:
//
//   - is_primary and the room override are PLANNED attributes the editor
//     cannot re-express; they always follow a staff member who stays.
//   - the DEVIATION state (is_substitute, is_absent, absence_reason) follows
//     only when the roster MEMBERSHIP is unchanged. A changed staff set
//     re-plans the base roster, so the old deviations no longer describe
//     it and every row is recreated as plain planned staff; the dropped
//     deviations are recorded in the Änderungsprotokoll (#1886).
func (s *InstanceLifecycleService) replaceInstanceAssignments(ctx context.Context, instance *scheduleModel.ActivityInstance, staffIDs, studentIDs []int64, actorAccountID *int64) error {
	prior, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "update instance: load existing staff", Err: err}
	}
	priorByStaff := make(map[int64]*scheduleModel.InstanceStaff, len(prior))
	for _, row := range prior {
		priorByStaff[row.StaffID] = row
	}
	newStaffIDs := sliceutil.UniquePositive(staffIDs)
	rosterUnchanged := sameStaffMembership(newStaffIDs, priorByStaff)
	if !rosterUnchanged {
		if err := s.logDroppedDeviations(ctx, instance, prior, actorAccountID); err != nil {
			return err
		}
	}
	if err := s.lockRosterCareDays(ctx, instance, studentIDs); err != nil {
		return err
	}
	if err := s.deps.InstanceStaffRepo.DeleteByInstanceID(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "update instance: clear staff", Err: err}
	}
	if err := s.deps.InstanceStudents.DeleteByInstanceID(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "update instance: clear students", Err: err}
	}
	tenantID := tenant.FromContext(ctx)
	for _, staffID := range newStaffIDs {
		row := recreatedStaffRow(instance.ID, staffID, priorByStaff[staffID], rosterUnchanged)
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStaffRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "update instance: assign staff", Err: err}
		}
	}
	return s.assignInstanceStudents(ctx, instance, sliceutil.UniquePositive(studentIDs), tenantID, "update instance")
}

// sameStaffMembership: staff_id is unique per block, so a length and
// membership check decides set equality.
func sameStaffMembership(staffIDs []int64, prior map[int64]*scheduleModel.InstanceStaff) bool {
	if len(staffIDs) != len(prior) {
		return false
	}
	for _, staffID := range staffIDs {
		if _, ok := prior[staffID]; !ok {
			return false
		}
	}
	return true
}

func recreatedStaffRow(instanceID, staffID int64, prior *scheduleModel.InstanceStaff, keepDeviation bool) *scheduleModel.InstanceStaff {
	row := &scheduleModel.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	if prior == nil {
		return row
	}
	row.IsPrimary = prior.IsPrimary
	row.RoomID = prior.RoomID
	if keepDeviation {
		row.IsSubstitute = prior.IsSubstitute
		row.IsAbsent = prior.IsAbsent
		row.AbsenceReason = prior.AbsenceReason
	}
	return row
}

// logDroppedDeviations records one protocol entry per prior row that
// carried a deviation, so "warum ist die Vertretung weg?" stays answerable
// after a roster edit (#1886).
func (s *InstanceLifecycleService) logDroppedDeviations(ctx context.Context, instance *scheduleModel.ActivityInstance, prior []*scheduleModel.InstanceStaff, actorAccountID *int64) error {
	for _, row := range prior {
		if !row.IsSubstitute && !row.IsAbsent {
			continue
		}
		dropped := map[string]any{
			"is_substitute": row.IsSubstitute,
			"is_absent":     row.IsAbsent,
		}
		if row.AbsenceReason != nil {
			dropped["reason"] = *row.AbsenceReason
		}
		if err := s.logDeviationEvent(ctx, deviationEventInput{
			instance:       instance,
			eventType:      DeviationEventDroppedByEdit,
			subjectStaffID: &row.StaffID,
			oldValue:       dropped,
			actorAccountID: actorAccountID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// lockRosterCareDays takes the care-day locks of the old and new children
// before any attendance row changes. Partial-absence writers lock student →
// care day and then update rows; rewriting rows first while waiting on a
// care day would deadlock against that order.
func (s *InstanceLifecycleService) lockRosterCareDays(ctx context.Context, instance *scheduleModel.ActivityInstance, studentIDs []int64) error {
	priorStudents, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "update instance: load existing students", Err: err}
	}
	lockStudentIDs := make([]int64, 0, len(priorStudents)+len(studentIDs))
	for _, row := range priorStudents {
		if row != nil && row.StudentID > 0 {
			lockStudentIDs = append(lockStudentIDs, row.StudentID)
		}
	}
	lockStudentIDs = append(lockStudentIDs, studentIDs...)
	return s.lockCareExceptionDaysForStudents(ctx, sliceutil.UniquePositive(lockStudentIDs), timezone.Date(instance.Date))
}

// lockCareExceptionDaysForStudents takes student → care-day locks for every
// child on the date, sorted, matching the partial-absence writers.
func (s *InstanceLifecycleService) lockCareExceptionDaysForStudents(ctx context.Context, studentIDs []int64, date timezone.Date) error {
	if len(studentIDs) == 0 || s.deps.DB == nil {
		return nil
	}
	sorted := slices.Clone(studentIDs)
	slices.Sort(sorted)
	for _, studentID := range sorted {
		if err := s.deps.CareDayLocks.LockStudentAndExceptionDay(ctx, studentID, date.String()); err != nil {
			return &ScheduleError{Op: "lock care exception day for roster rewrite", Err: err}
		}
	}
	return nil
}

// capturedSlot is the (template, date, start) key a block occupied before an
// edit. StartHHMMSS is compared as text, independent of the year anchor bun
// picks when scanning TIME columns.
type capturedSlot struct {
	ActivityGroupID *int64
	Date            timezone.Date
	StartHHMMSS     string
}

// consumeMovedSlot writes a cancelled exception for the original (template,
// date) when an edit moved a template-backed block to another date or start
// time; otherwise the next materialization recreates the vacated slot.
// Exceptions are unique per (template, date): an existing one already
// consumes the slot or was authored deliberately, so it is left untouched.
func (s *InstanceLifecycleService) consumeMovedSlot(ctx context.Context, orig capturedSlot, req timetable.UpdateInstanceInput) error {
	if orig.ActivityGroupID == nil {
		return nil // spontaneous before the edit — materialization never recreates it
	}
	if orig.Date == req.Date && orig.StartHHMMSS == formatTimeOfDay(req.StartTime) {
		return nil // slot key unchanged — nothing vacated
	}
	existing, err := s.deps.ExceptionRepo.FindByActivityGroupAndDate(ctx, *orig.ActivityGroupID, scheduleModel.Date(orig.Date))
	if err != nil {
		return &ScheduleError{Op: "update instance: check slot exception", Err: err}
	}
	if existing != nil {
		s.getLogger().Warn("moved instance: exception already exists for original date, leaving it untouched",
			slog.Int64("activity_group_id", *orig.ActivityGroupID),
			slog.String("original_date", orig.Date.String()),
			slog.String("exception_type", existing.ExceptionType),
		)
		return nil
	}
	reason := movedSlotReason
	exc := &scheduleModel.ActivityException{
		ActivityGroupID: *orig.ActivityGroupID,
		ExceptionDate:   scheduleModel.Date(orig.Date),
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exc.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.ExceptionRepo.Create(ctx, exc); err != nil {
		return &ScheduleError{Op: "update instance: consume moved slot", Err: err}
	}
	s.getLogger().Info("moved instance: original slot consumed via cancelled exception",
		slog.Int64("activity_group_id", *orig.ActivityGroupID),
		slog.String("original_date", orig.Date.String()),
		slog.String("original_start", orig.StartHHMMSS),
		slog.String("new_date", req.Date.String()),
	)
	return nil
}
