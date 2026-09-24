package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Reopen restores the exact live session, visits, supervisors and
// attendance captured immediately before completion. The day lock
// serializes it with lifecycle and staffing writes; any conflict aborts the
// tenant transaction.
func (s *InstanceLifecycleService) Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*timetable.StartInstanceResult, error) {
	if !s.hasTx(ctx) {
		var result *timetable.StartInstanceResult
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var reopenErr error
			result, reopenErr = s.Reopen(txCtx, instanceID, accountID, isAdmin)
			return reopenErr
		})
		return result, err
	}
	instance, snapshot, err := s.reopenableInstance(ctx, instanceID, accountID, isAdmin)
	if err != nil {
		return nil, err
	}
	// Check-in locks student then room. Lock the snapshot's children first
	// so a concurrent check-in of one of them into another group in the
	// same room cannot deadlock.
	studentIDs, err := s.lockReopenSnapshotStudents(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	if err := s.validateReopenState(ctx, instance, snapshot); err != nil {
		return nil, err
	}
	if err := s.deps.RecoveryRepo.Restore(ctx, instance.ID, snapshot, s.now()); err != nil {
		if modelBase.IsUniqueViolation(err) {
			return nil, fmt.Errorf("%w: concurrent check-in", timetable.ErrTimetableOperationConflict)
		}
		return nil, &ScheduleError{Op: "reopen instance: restore snapshot", Err: err}
	}
	instance.Status = scheduleModel.InstanceStatusActive
	instance.ActiveGroupID = ptrTo(snapshot.ActiveGroupID)
	instance.CompletedAt, instance.CompletedBy, instance.ReopenUntil = nil, nil, nil
	instance.CompletionSnapshot = nil
	s.broadcastInstanceEvent(ctx, LifecycleEventInstanceStarted, instance, nil, nil)
	s.broadcastRestoredVisits(ctx, snapshot.ActiveGroupID, studentIDs)
	return &timetable.StartInstanceResult{
		Instance: LifecycleInstanceOf(instance), ActiveGroupID: snapshot.ActiveGroupID, Warnings: []timetable.InstanceConflictWarning{},
	}, nil
}

// reopenableInstance loads a completed block within its reopen window under
// the day lock, applies the actor gate and decodes its completion snapshot.
func (s *InstanceLifecycleService) reopenableInstance(
	ctx context.Context, instanceID, accountID int64, isAdmin bool,
) (*scheduleModel.ActivityInstance, scheduleModel.ActivityCompletionSnapshot, error) {
	var snapshot scheduleModel.ActivityCompletionSnapshot
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, snapshot, err
	}
	instance, err = s.lockDayAndReload(ctx, instance, "reopen instance")
	if err != nil {
		return nil, snapshot, err
	}
	if instance.Status != scheduleModel.InstanceStatusCompleted || instance.ReopenUntil == nil || s.now().After(*instance.ReopenUntil) {
		return nil, snapshot, fmt.Errorf("%w: reopen window expired", timetable.ErrInvalidInstanceTransition)
	}
	if !isAdmin && (instance.CompletedBy == nil || *instance.CompletedBy != accountID) {
		return nil, snapshot, timetable.ErrTimetableOperationForbidden
	}
	if len(instance.CompletionSnapshot) == 0 || json.Unmarshal(instance.CompletionSnapshot, &snapshot) != nil {
		return nil, snapshot, fmt.Errorf("%w: completion snapshot missing", timetable.ErrInvalidInstanceTransition)
	}
	if s.deps.RecoveryRepo == nil {
		return nil, snapshot, &ScheduleError{Op: "reopen instance: restore snapshot", Err: errors.New("recovery repository not wired")}
	}
	return instance, snapshot, nil
}

func (s *InstanceLifecycleService) validateReopenState(ctx context.Context, instance *scheduleModel.ActivityInstance, snapshot scheduleModel.ActivityCompletionSnapshot) error {
	if err := s.validateReopenOccupancy(ctx, instance, snapshot); err != nil {
		return err
	}
	if err := s.validateReopenAttendanceUnchanged(ctx, instance); err != nil {
		return err
	}
	return s.validateReopenSupervisorsUnchanged(ctx, instance, snapshot)
}

func (s *InstanceLifecycleService) lockReopenSnapshotStudents(ctx context.Context, snapshot scheduleModel.ActivityCompletionSnapshot) ([]int64, error) {
	if len(snapshot.VisitIDs) == 0 {
		return nil, nil
	}
	closedVisits, err := s.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{snapshot.ActiveGroupID}})
	if err != nil {
		return nil, &ScheduleError{Op: "reopen instance: load visits", Err: err}
	}
	studentByVisit := make(map[int64]int64, len(closedVisits))
	for _, visit := range closedVisits {
		studentByVisit[visit.ID] = visit.StudentID
	}
	studentIDs := make([]int64, 0, len(snapshot.VisitIDs))
	for _, visitID := range snapshot.VisitIDs {
		studentID := studentByVisit[visitID]
		if studentID <= 0 {
			return nil, fmt.Errorf("%w: snapshot visit missing", timetable.ErrInvalidInstanceTransition)
		}
		studentIDs = append(studentIDs, studentID)
	}
	slices.Sort(studentIDs)
	studentIDs = slices.Compact(studentIDs)
	locked, err := s.deps.StudentRepo.FindByIDsForUpdate(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "reopen instance: lock students", Err: err}
	}
	for _, studentID := range studentIDs {
		if locked[studentID] == nil {
			return nil, &ScheduleError{Op: "reopen instance: lock student", Err: modelBase.ErrNotFound}
		}
	}
	currentVisits, err := s.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true, NewestFirst: true, StudentOrder: true})
	if err != nil {
		return nil, err
	}
	if len(currentVisits) > 0 {
		return nil, fmt.Errorf("%w: student %d already has an active visit", timetable.ErrTimetableOperationConflict, currentVisits[0].StudentID)
	}
	return studentIDs, nil
}

func (s *InstanceLifecycleService) validateReopenOccupancy(ctx context.Context, instance *scheduleModel.ActivityInstance, snapshot scheduleModel.ActivityCompletionSnapshot) error {
	capacity, ok, err := s.deps.Rooms.LockRoom(ctx, instance.RoomID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: lock room", Err: err}
	}
	if !ok {
		return &ScheduleError{Op: "reopen instance: lock room", Err: fmt.Errorf("room %d not found", instance.RoomID)}
	}
	hasConflict, _, err := s.deps.ActiveGroupRepo.CheckRoomConflict(ctx, instance.RoomID, snapshot.ActiveGroupID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: check room", Err: err}
	}
	if hasConflict {
		return studentpresence.ErrRoomConflict
	}
	if len(snapshot.VisitIDs) == 0 || capacity == nil || *capacity <= 0 {
		return nil
	}
	currentOccupancy, err := s.deps.Presence.CountOpenVisitsInRoom(ctx, instance.RoomID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: count room occupancy", Err: err}
	}
	if currentOccupancy+len(snapshot.VisitIDs) > *capacity {
		return studentpresence.ErrRoomCapacityExceeded
	}
	return nil
}

func (s *InstanceLifecycleService) validateReopenAttendanceUnchanged(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	if s.deps.RecoveryRepo != nil {
		if err := s.deps.RecoveryRepo.LockAttendance(ctx, instance.ID); err != nil {
			return &ScheduleError{Op: "reopen instance: lock attendance", Err: err}
		}
	}
	if instance.CompletedAt == nil {
		return nil
	}
	rows, err := s.deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: load attendance", Err: err}
	}
	for _, row := range rows {
		if row.UpdatedAt.After(*instance.CompletedAt) {
			return fmt.Errorf("%w: attendance changed after completion", timetable.ErrTimetableOperationConflict)
		}
	}
	return nil
}

// validateReopenSupervisorsUnchanged refuses a reopen when a snapshotted
// supervisor row is gone, changed after completion, or its person now
// supervises another group.
func (s *InstanceLifecycleService) validateReopenSupervisorsUnchanged(ctx context.Context, instance *scheduleModel.ActivityInstance, snapshot scheduleModel.ActivityCompletionSnapshot) error {
	if len(snapshot.SupervisorIDs) == 0 {
		return nil
	}
	if err := s.deps.RecoveryRepo.LockSupervisors(ctx, snapshot.SupervisorIDs); err != nil {
		return &ScheduleError{Op: "reopen instance: lock supervisors", Err: err}
	}
	rows, err := s.deps.SupervisorRepo.FindByActiveGroupID(ctx, snapshot.ActiveGroupID, false)
	if err != nil {
		return &ScheduleError{Op: "reopen instance: load supervisors", Err: err}
	}
	byID := make(map[int64]*studentpresence.StaffedSupervision, len(rows))
	staffIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
		if !slices.Contains(staffIDs, row.StaffID) {
			staffIDs = append(staffIDs, row.StaffID)
		}
	}
	activeByStaff, err := s.activeSupervisionsByStaff(ctx, staffIDs)
	if err != nil {
		return err
	}
	for _, supervisorID := range snapshot.SupervisorIDs {
		if err := reopenSupervisorConflict(instance, byID[supervisorID], activeByStaff); err != nil {
			return err
		}
	}
	return nil
}

func (s *InstanceLifecycleService) activeSupervisionsByStaff(ctx context.Context, staffIDs []int64) (map[int64][]studentpresence.GroupSupervision, error) {
	activeByStaff := make(map[int64][]studentpresence.GroupSupervision, len(staffIDs))
	if len(staffIDs) == 0 {
		return activeByStaff, nil
	}
	day := timezone.TodayDate().String()
	activeRows, err := s.deps.Presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{ActiveOn: &day, StaffIDs: staffIDs})
	if err != nil {
		return nil, &ScheduleError{Op: "reopen instance: load staff supervisions", Err: err}
	}
	for _, row := range activeRows {
		activeByStaff[row.StaffID] = append(activeByStaff[row.StaffID], row)
	}
	return activeByStaff, nil
}

func reopenSupervisorConflict(
	instance *scheduleModel.ActivityInstance, row *studentpresence.StaffedSupervision, activeByStaff map[int64][]studentpresence.GroupSupervision,
) error {
	if row == nil {
		return fmt.Errorf("%w: supervisor snapshot missing", timetable.ErrTimetableOperationConflict)
	}
	if instance.CompletedAt != nil && row.UpdatedAt.After(*instance.CompletedAt) {
		return fmt.Errorf("%w: supervisor changed after completion", timetable.ErrTimetableOperationConflict)
	}
	for _, other := range activeByStaff[row.StaffID] {
		if other.ID != row.ID {
			return fmt.Errorf("%w: staff %d now supervises another group", timetable.ErrTimetableOperationConflict, row.StaffID)
		}
	}
	return nil
}

// broadcastRestoredVisits emits the check-in-equivalent invalidation for the
// visits Reopen brought back. The start and supervision events refresh the
// timetable and supervision caches, not the student-list and location
// caches the completion's bulk checkout invalidated. Student ids stay on
// group-scoped topics only.
func (s *InstanceLifecycleService) broadcastRestoredVisits(ctx context.Context, activeGroupID int64, studentIDs []int64) {
	if s.deps.Broadcaster == nil || activeGroupID == 0 || len(studentIDs) == 0 {
		return
	}
	allStudentIDs := make([]string, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		allStudentIDs = append(allStudentIDs, strconv.FormatInt(studentID, 10))
	}
	eduGroups := s.restoredEducationGroups(ctx, studentIDs)
	allEduGroupIDs := make([]string, 0, len(eduGroups))
	for gid := range eduGroups {
		allEduGroupIDs = append(allEduGroupIDs, strconv.FormatInt(gid, 10))
	}
	sessionID := strconv.FormatInt(activeGroupID, 10)
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		activeEvent := LifecycleEvent{Type: LifecycleEventBulkStudentCheckIn, ActiveGroupID: sessionID, StudentIDs: &allStudentIDs}
		if len(allEduGroupIDs) > 0 {
			activeEvent.GroupIDs = &allEduGroupIDs
		}
		s.broadcastRestoredToGroup(tenantID, sessionID, activeEvent, "active_group_id")
		for gid, ids := range eduGroups {
			groupStudentIDs := ids
			eduGroupID := []string{strconv.FormatInt(gid, 10)}
			eduEvent := LifecycleEvent{Type: LifecycleEventBulkStudentCheckIn, ActiveGroupID: sessionID, StudentIDs: &groupStudentIDs, GroupIDs: &eduGroupID}
			s.broadcastRestoredToGroup(tenantID, fmt.Sprintf("edu:%d", gid), eduEvent, "education_group_topic")
		}
		dashEvent := LifecycleEvent{Type: LifecycleEventDashboardCountsChanged}
		if len(allEduGroupIDs) > 0 {
			dashEvent.GroupIDs = &allEduGroupIDs
		}
		if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, dashEvent); err != nil {
			s.getLogger().Warn("SSE dashboard counts broadcast failed",
				slog.String("error", err.Error()),
				slog.Int64("tenant_id", tenantID),
			)
		}
	})
}

// restoredEducationGroups groups the restored children by their education
// group. A failed lookup only narrows the invalidation.
func (s *InstanceLifecycleService) restoredEducationGroups(ctx context.Context, studentIDs []int64) map[int64][]string {
	eduGroups := make(map[int64][]string)
	if s.deps.StudentRepo == nil {
		return eduGroups
	}
	students, err := s.deps.StudentRepo.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		s.getLogger().Warn("reopen visit invalidation: student scope lookup failed",
			slog.String("error", err.Error()),
		)
		return eduGroups
	}
	for _, studentID := range studentIDs {
		student := students[studentID]
		if student == nil || student.GroupID == nil {
			continue
		}
		gid := *student.GroupID
		eduGroups[gid] = append(eduGroups[gid], strconv.FormatInt(studentID, 10))
	}
	return eduGroups
}

func (s *InstanceLifecycleService) broadcastRestoredToGroup(tenantID int64, topic string, event LifecycleEvent, topicKey string) {
	if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, topic, event); err != nil {
		s.getLogger().Warn("SSE broadcast failed",
			slog.String("event_type", event.Type),
			slog.String(topicKey, topic),
			slog.String("error", err.Error()),
		)
	}
}
