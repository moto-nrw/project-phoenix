package compose

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// evaluateLifecycleAvailability applies the owner's clock policy to a
// retained row.
func evaluateLifecycleAvailability(instance *scheduleModel.ActivityInstance, now time.Time, startLeadMinutes int, enforcePlannedEnd bool) timetable.LifecycleAvailability {
	window := timetable.LifecycleWindow{Date: timezone.Date(instance.Date), StartTime: instance.StartTime, EndTime: instance.EndTime, IsSpontaneous: instance.IsSpontaneous}
	return timetable.EvaluateLifecycleAvailability(window, now, startLeadMinutes, enforcePlannedEnd)
}

func (s *InstanceLifecycleService) validateStartTime(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time) error {
	if !s.deps.EnforceTimePolicy || instance.IsSpontaneous {
		return nil
	}
	lead, err := s.deps.Settings.StartLeadMinutes(ctx)
	if err != nil {
		return fmt.Errorf("%w: resolve start lead: %v", timetable.ErrLifecycleSettings, err)
	}
	availability := evaluateLifecycleAvailability(instance, now, lead, true)
	if now.Before(availability.StartAvailableAt) {
		return fmt.Errorf("%w: available at %s", timetable.ErrInstanceStartTooEarly, availability.StartAvailableAt.Format(time.RFC3339))
	}
	if !availability.CanStart {
		return timetable.ErrInstanceStartExpired
	}
	return nil
}

func validateSpontaneousStartWorkday(instance *scheduleModel.ActivityInstance, now time.Time) error {
	if !instance.IsSpontaneous {
		return nil
	}
	switch now.In(timezone.Berlin).Weekday() {
	case time.Saturday, time.Sunday:
		return timetable.ErrInstanceWeekend
	default:
		return nil
	}
}

// Start implements planned → active inside the caller's tenant tx: it opens
// the Student Presence session, copies the present staff into its
// supervisors and absorbs unsupervised sessions in the room. Any failure
// rolls back the whole transition.
func (s *InstanceLifecycleService) Start(ctx context.Context, instanceID, startedByStaffID int64) (*timetable.StartInstanceResult, error) {
	if !s.hasTx(ctx) {
		var result *timetable.StartInstanceResult
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var startErr error
			result, startErr = s.Start(txCtx, instanceID, startedByStaffID)
			return startErr
		})
		return result, err
	}
	instance, err := s.startableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// Conflicts are advisory; failed presence reads abort before any writes.
	warnings, err := detectStartConflicts(ctx, s.deps.StartConflicts, instance)
	if err != nil {
		return nil, err
	}
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return nil, &ScheduleError{Op: "start instance: load instance_staff", Err: err}
	}
	if _, _, err := s.deps.Rooms.LockRoom(ctx, instance.RoomID); err != nil {
		return nil, &ScheduleError{Op: "start instance: lock room", Err: err}
	}
	now := s.now()
	if timetable.SpontaneousStartWorkdayGuarded(ctx) && instance.IsSpontaneous {
		if err := validateSpontaneousStartWorkday(instance, now); err != nil {
			return nil, err
		}
	}
	newGroup, activeStaffRows, err := s.openStartedSession(ctx, instance, staffRows, now)
	if err != nil {
		return nil, err
	}
	if err := s.markStarted(ctx, instance, newGroup.ID, startedByStaffID, now); err != nil {
		return nil, err
	}
	s.broadcastInstanceEvent(ctx, LifecycleEventInstanceStarted, instance, newGroup, activeStaffRows)
	return &timetable.StartInstanceResult{
		Instance:      LifecycleInstanceOf(instance),
		ActiveGroupID: newGroup.ID,
		Warnings:      warnings,
	}, nil
}

// startableInstance loads a planned block within its start window, then
// takes its day lock before the roster is read: the day-wide staffing saves
// hold the same lock while they flip is_absent and insert substitutes, so a
// start without it could copy a stale roster into the session (#1840). The
// block is re-validated under the lock.
func (s *InstanceLifecycleService) startableInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.checkStartable(ctx, instance); err != nil {
		return nil, err
	}
	instance, err = s.lockDayAndReload(ctx, instance, "start instance")
	if err != nil {
		return nil, err
	}
	if err := s.checkStartable(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func (s *InstanceLifecycleService) checkStartable(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	if instance.Status != scheduleModel.InstanceStatusPlanned {
		return fmt.Errorf("%w: cannot start instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	return s.validateStartTime(ctx, instance, s.now())
}

// openStartedSession creates the session, its supervisors and absorbs the
// room's unsupervised sessions. Absent staff rows are skipped: those people
// are marked out for the block, and supervising would contradict that. A
// failed supervisor aborts the start, because missing supervisors would
// undermine the conflict model of the next start elsewhere.
func (s *InstanceLifecycleService) openStartedSession(
	ctx context.Context, instance *scheduleModel.ActivityInstance, staffRows []*scheduleModel.InstanceStaff, now time.Time,
) (*studentpresence.LiveGroup, []*scheduleModel.InstanceStaff, error) {
	newGroup := &studentpresence.LiveGroup{
		StartTime:       now,
		LastActivity:    now,
		TimeoutMinutes:  30,
		ActivityGroupID: instance.ActivityGroupID,
		DeviceID:        nil,
		RoomID:          instance.RoomID,
	}
	newGroup.TenantID = tenant.FromContext(ctx)
	if err := s.deps.ActiveGroupRepo.CreateSession(ctx, newGroup); err != nil {
		return nil, nil, &ScheduleError{Op: "start instance: create active.group", Err: err}
	}
	activeStaffRows := make([]*scheduleModel.InstanceStaff, 0, len(staffRows))
	for _, row := range staffRows {
		if row.IsAbsent {
			continue
		}
		activeStaffRows = append(activeStaffRows, row)
		sup := &studentpresence.GroupSupervision{
			StaffID:   row.StaffID,
			GroupID:   newGroup.ID,
			Role:      "supervisor",
			StartDate: timezone.DateFromTime(now).String(),
		}
		sup.TenantID = tenant.FromContext(ctx)
		if err := s.deps.SupervisorRepo.CreateSupervision(ctx, sup); err != nil {
			return nil, nil, &ScheduleError{Op: "start instance: create supervisor", Err: err}
		}
	}
	// Fold open, supervisor-less sessions in this room into the fresh session
	// before announcing it; same tenant tx, so a failure rolls back the start.
	if err := s.absorbUnsupervisedOpenGroups(ctx, instance.ID, instance.RoomID, newGroup.ID); err != nil {
		return nil, nil, &ScheduleError{Op: "start instance: absorb unsupervised sessions", Err: err}
	}
	return newGroup, activeStaffRows, nil
}

// markStarted writes only the columns the transition changes.
func (s *InstanceLifecycleService) markStarted(ctx context.Context, instance *scheduleModel.ActivityInstance, activeGroupID, startedByStaffID int64, now time.Time) error {
	instance.Status = scheduleModel.InstanceStatusActive
	instance.ActiveGroupID = &activeGroupID
	if startedByStaffID > 0 {
		startedBy := startedByStaffID
		instance.StartedBy = &startedBy
	}
	instance.StartedAt = &now
	if err := s.updateLifecycleColumns(ctx, instance, "status", "active_group_id", "started_at", "started_by"); err != nil {
		return &ScheduleError{Op: "start instance: update instance", Err: err}
	}
	return nil
}

// absorbUnsupervisedOpenGroups folds today's unbridged sessions without an
// active supervisor in the room into the freshly started session: their open
// visits move over, then the orphan session ends. Typical case: a kiosk scan
// auto-created a fallback session before the planned block started (#2161).
// Independent room stays of a system activity (#3066), sessions bridged to
// any block and supervised sessions (#2139) are left alone. Absorbed visits
// are mirrored into the block's attendance rows before commit; the start
// event that follows makes clients refetch.
func (s *InstanceLifecycleService) absorbUnsupervisedOpenGroups(ctx context.Context, instanceID, roomID, newGroupID int64) error {
	openGroups, err := s.deps.ActiveGroupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return fmt.Errorf("find open groups in room %d: %w", roomID, err)
	}
	// Lock candidates in ascending ID order, so a later second multi-row
	// locker cannot form a wait cycle with this loop.
	slices.SortFunc(openGroups, func(a, b *studentpresence.LiveGroup) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	systemByActivity, err := s.systemActivitiesByID(ctx, openGroups)
	if err != nil {
		return fmt.Errorf("load system activities for room %d: %w", roomID, err)
	}
	today := timezone.TodayDate()
	movedTotal := int64(0)
	for _, group := range openGroups {
		if !absorptionCandidate(group, newGroupID, today, systemByActivity) {
			continue
		}
		moved, err := s.absorbGroup(ctx, group.ID, roomID, newGroupID, today)
		if err != nil {
			return err
		}
		movedTotal += moved
	}
	if movedTotal > 0 {
		return s.syncAbsorbedVisitAttendance(ctx, instanceID, newGroupID)
	}
	return nil
}

func absorptionCandidate(group *studentpresence.LiveGroup, newGroupID int64, today timezone.Date, systemByActivity map[int64]bool) bool {
	if group.ID == newGroupID || timezone.DateFromTime(group.StartTime) != today {
		return false
	}
	templateID, ok := group.TemplateID()
	return !ok || !group.IsIndependentRoomSession(systemByActivity[templateID])
}

// absorbGroup moves one candidate's open visits and ends it. The row lock
// serializes the "still open and unsupervised" decision with supervision
// claims, which lock the same row before they insert a supervisor.
func (s *InstanceLifecycleService) absorbGroup(ctx context.Context, groupID, roomID, newGroupID int64, today timezone.Date) (int64, error) {
	absorbable, err := s.absorbableGroup(ctx, groupID, roomID, today)
	if err != nil || !absorbable {
		return 0, err
	}
	moved, err := s.deps.Presence.TransferOpenVisits(ctx, groupID, newGroupID)
	if err != nil {
		return 0, fmt.Errorf("move open visits from group %d to group %d: %w", groupID, newGroupID, err)
	}
	if err := s.deps.Presence.EndGroup(ctx, groupID, s.now()); err != nil {
		return 0, fmt.Errorf("end absorbed group %d: %w", groupID, err)
	}
	s.getLogger().Info("absorbed unsupervised session into started instance",
		slog.Int64("absorbed_group_id", groupID),
		slog.Int64("new_group_id", newGroupID),
		slog.Int64("moved_visits", moved),
	)
	return moved, nil
}

// absorbableGroup re-checks a candidate under its row lock: still open, in
// the room, started today, bridged to no block and without a supervisor.
func (s *InstanceLifecycleService) absorbableGroup(ctx context.Context, groupID, roomID int64, today timezone.Date) (bool, error) {
	lockedGroup, err := s.deps.ActiveGroupRepo.FindByIDForUpdate(ctx, groupID)
	if err != nil {
		return false, fmt.Errorf("lock open group %d: %w", groupID, err)
	}
	if lockedGroup == nil || lockedGroup.EndTime != nil || lockedGroup.RoomID != roomID {
		return false, nil
	}
	if timezone.DateFromTime(lockedGroup.StartTime) != today {
		return false, nil
	}
	instance, err := s.deps.InstanceRepo.FindByActiveGroupID(ctx, groupID)
	if err != nil {
		return false, fmt.Errorf("load timetable bridge of group %d: %w", groupID, err)
	}
	if instance != nil {
		return false, nil
	}
	supervisors, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, groupID, true)
	if err != nil {
		return false, fmt.Errorf("load supervisors of group %d: %w", groupID, err)
	}
	return len(supervisors) == 0, nil
}

func (s *InstanceLifecycleService) systemActivitiesByID(ctx context.Context, groups []*studentpresence.LiveGroup) (map[int64]bool, error) {
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group.DeviceID != nil {
			continue
		}
		templateID, ok := group.TemplateID()
		if !ok {
			continue
		}
		if _, dup := seen[templateID]; dup {
			continue
		}
		seen[templateID] = struct{}{}
		ids = append(ids, templateID)
	}
	result := make(map[int64]bool, len(ids))
	if len(ids) == 0 || s.deps.ActivityGroupRepo == nil {
		return result, nil
	}
	activities, err := s.deps.ActivityGroupRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, activity := range activities {
		if activity != nil {
			result[activity.ID] = activity.IsSystem
		}
	}
	return result, nil
}

func (s *InstanceLifecycleService) syncAbsorbedVisitAttendance(ctx context.Context, instanceID, activeGroupID int64) error {
	visits, err := s.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{activeGroupID}})
	if err != nil {
		return fmt.Errorf("load absorbed visits from group %d: %w", activeGroupID, err)
	}
	existingRows, err := s.deps.InstanceStudents.FindByInstanceIDs(ctx, []int64{instanceID})
	if err != nil {
		return fmt.Errorf("load absorbed attendance for instance %d: %w", instanceID, err)
	}
	existingByStudent := make(map[int64]*scheduleModel.InstanceStudent, len(existingRows))
	for _, row := range existingRows {
		existingByStudent[row.StudentID] = row
	}
	for _, visit := range visits {
		if visit.ExitTime != nil {
			continue
		}
		updated, err := s.deps.InstanceStudents.UpdateAttendanceFromCheckin(ctx, instanceID, visit.StudentID, visit.EntryTime)
		if err != nil {
			return fmt.Errorf("mark absorbed student %d present: %w", visit.StudentID, err)
		}
		if updated || existingByStudent[visit.StudentID] != nil {
			continue
		}
		if _, err := s.deps.InstanceStudents.CreateUnplannedPresentIfAbsent(ctx, instanceID, visit.StudentID, visit.EntryTime); err != nil {
			return fmt.Errorf("create absorbed student %d attendance: %w", visit.StudentID, err)
		}
	}
	return nil
}
