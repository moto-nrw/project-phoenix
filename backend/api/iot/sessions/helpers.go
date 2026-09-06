package sessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// startSession starts an activity session with proper validation and logging
func (rs *Resource) startSession(ctx context.Context, req *SessionStartRequest, deviceCtx *iot.Device) (*active.Group, error) {
	slog.Default().InfoContext(ctx, "session start request",
		slog.Int64("activity_id", req.ActivityID),
		slog.Int("supervisor_count", len(req.SupervisorIDs)),
		slog.Bool("force", req.Force),
	)

	if len(req.SupervisorIDs) == 0 {
		slog.Default().WarnContext(ctx, "no supervisor IDs provided")
		return nil, errors.New("at least one supervisor ID is required")
	}

	slog.Default().DebugContext(ctx, "using multi-supervisor session methods",
		slog.Int("supervisor_count", len(req.SupervisorIDs)),
	)

	if req.Force {
		slog.Default().InfoContext(ctx, "force starting activity session",
			slog.Int("supervisor_count", len(req.SupervisorIDs)),
		)
		return rs.ActiveService.ForceStartActivitySessionWithSupervisors(ctx, req.ActivityID, deviceCtx.ID, req.SupervisorIDs, req.RoomID)
	}

	slog.Default().InfoContext(ctx, "starting activity session",
		slog.Int("supervisor_count", len(req.SupervisorIDs)),
	)
	return rs.ActiveService.StartActivitySessionWithSupervisors(ctx, req.ActivityID, deviceCtx.ID, req.SupervisorIDs, req.RoomID)
}

// handleSessionConflictError handles session conflict errors and returns true if error was handled
func (rs *Resource) handleSessionConflictError(w http.ResponseWriter, r *http.Request, err error, activityID, deviceID int64) bool {
	if !errors.Is(err, activeSvc.ErrSessionConflict) && !errors.Is(err, activeSvc.ErrDeviceAlreadyActive) {
		return false
	}

	conflictInfo, conflictErr := rs.ActiveService.CheckActivityConflict(r.Context(), activityID, deviceID)
	if conflictErr != nil || !conflictInfo.HasConflict {
		return false
	}

	response := SessionStartResponse{
		Status:  "conflict",
		Message: conflictInfo.ConflictMessage,
		ConflictInfo: &ConflictInfoResponse{
			HasConflict:     conflictInfo.HasConflict,
			ConflictMessage: conflictInfo.ConflictMessage,
			CanOverride:     conflictInfo.CanOverride,
		},
	}

	if conflictInfo.ConflictingDevice != nil {
		if deviceID, parseErr := strconv.ParseInt(*conflictInfo.ConflictingDevice, 10, 64); parseErr == nil {
			response.ConflictInfo.ConflictingDevice = &deviceID
		}
	}

	common.Respond(w, r, http.StatusConflict, response, "Session conflict detected")
	return true
}

// buildSessionStartResponse builds the success response with supervisor information.
// ActivityID is *int64 after WP-B6 — for today's IoT session start, the
// activeGroup always carries a template id, but we propagate the pointer as-is
// so spontaneous sessions (WP-B9+) surface cleanly as null.
func (rs *Resource) buildSessionStartResponse(ctx context.Context, activeGroup *active.Group, deviceCtx *iot.Device) SessionStartResponse {
	response := SessionStartResponse{
		ActiveGroupID: activeGroup.ID,
		ActivityID:    activeGroup.GroupID,
		DeviceID:      deviceCtx.ID,
		StartTime:     activeGroup.StartTime,
		Status:        "started",
		Message:       "Activity session started successfully",
	}

	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)

	if err == nil && len(supervisors) > 0 {
		response.Supervisors = rs.buildSupervisorInfos(ctx, supervisors)
	}

	return response
}

// buildSupervisorInfos builds supervisor information from supervisor list.
// Uses a single batch query to fetch all staff+person data instead of N individual queries.
func (rs *Resource) buildSupervisorInfos(ctx context.Context, supervisors []*active.GroupSupervisor) []SupervisorInfo {
	if len(supervisors) == 0 {
		return []SupervisorInfo{}
	}

	// Collect unique staff IDs
	staffIDs := make([]int64, 0, len(supervisors))
	seen := make(map[int64]bool, len(supervisors))
	for _, sup := range supervisors {
		if !seen[sup.StaffID] {
			staffIDs = append(staffIDs, sup.StaffID)
			seen[sup.StaffID] = true
		}
	}

	// Batch fetch all staff with person data in a single query
	staffMap, err := rs.UsersService.GetStaffWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		slog.Default().WarnContext(ctx, "batch staff lookup failed, falling back to empty list",
			slog.String("error", err.Error()),
		)
		return []SupervisorInfo{}
	}

	supervisorInfos := make([]SupervisorInfo, 0, len(supervisors))
	for _, supervisor := range supervisors {
		staff, ok := staffMap[supervisor.StaffID]
		if ok && staff != nil && staff.Person != nil {
			supervisorInfos = append(supervisorInfos, SupervisorInfo{
				StaffID:     supervisor.StaffID,
				FirstName:   staff.Person.FirstName,
				LastName:    staff.Person.LastName,
				DisplayName: fmt.Sprintf("%s %s", staff.Person.FirstName, staff.Person.LastName),
				Role:        supervisor.Role,
			})
		}
		// Skip supervisors where staff or person data is missing
	}

	return supervisorInfos
}

// filterActiveSupervisors filters for active supervisors (no end date)
func (rs *Resource) filterActiveSupervisors(supervisors []*active.GroupSupervisor) []*active.GroupSupervisor {
	active := make([]*active.GroupSupervisor, 0, len(supervisors))
	for _, sup := range supervisors {
		if sup.EndDate == nil && sup.StaffID > 0 {
			active = append(active, sup)
		}
	}
	return active
}

// countActiveStudents counts visits without an exit time (active students in session)
func countActiveStudents(visits []*active.Visit) int {
	count := 0
	for _, visit := range visits {
		if visit.ExitTime == nil {
			count++
		}
	}
	return count
}

func (rs *Resource) mirrorSessionToTimetable(ctx context.Context, activeGroup *active.Group, supervisorIDs []int64) {
	if activeGroup == nil || rs.TimetableData == nil {
		return
	}
	if existing, err := rs.TimetableData.GetInstanceByActiveGroupID(ctx, activeGroup.ID); err == nil && existing != nil {
		return
	} else if err != nil {
		slog.Default().WarnContext(ctx, "failed to check mirrored timetable instance",
			slog.Int64("active_group_id", activeGroup.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	startedAt := activeGroup.StartTime
	inBerlin := startedAt.In(timezone.Berlin)
	startMinutes := inBerlin.Hour()*60 + inBerlin.Minute()
	if startMinutes > 23*60+30 {
		startMinutes = 23*60 + 30
	}
	endMinutes := startMinutes + 60
	if endMinutes > 23*60+59 {
		endMinutes = 23*60 + 59
	}

	startedBy := firstPositiveID(supervisorIDs)
	inst := &scheduleModel.ActivityInstance{
		Date:            scheduleModel.DateFromTime(startedAt),
		ActivityGroupID: activeGroup.GroupID,
		Title:           rs.timetableTitleForActivity(ctx, activeGroup.GroupID),
		StartTime:       clockFromMinutes(startMinutes),
		EndTime:         clockFromMinutes(endMinutes),
		RoomID:          activeGroup.RoomID,
		Status:          scheduleModel.InstanceStatusActive,
		ActiveGroupID:   &activeGroup.ID,
		IsSpontaneous:   true,
		CreatedBy:       startedBy,
		StartedBy:       startedBy,
		StartedAt:       &startedAt,
	}
	inst.SetTenantID(tenant.FromContext(ctx))
	if err := rs.TimetableData.CreateActivityInstance(ctx, inst); err != nil {
		slog.Default().WarnContext(ctx, "failed to mirror IoT session to timetable",
			slog.Int64("active_group_id", activeGroup.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	for _, staffID := range sliceutil.UniquePositive(supervisorIDs) {
		row := &scheduleModel.InstanceStaff{
			InstanceID: inst.ID,
			StaffID:    staffID,
		}
		row.SetTenantID(tenant.FromContext(ctx))
		if err := rs.TimetableData.CreateInstanceStaff(ctx, row); err != nil {
			slog.Default().WarnContext(ctx, "failed to mirror IoT session staff to timetable",
				slog.Int64("instance_id", inst.ID),
				slog.Int64("staff_id", staffID),
				slog.String("error", err.Error()),
			)
		}
	}
	rs.broadcastMirroredInstance(ctx, realtime.EventInstanceStarted, inst, activeGroup)
}

// completeMirroredTimetableInstance ends the timetable side of an RFID session
// the kiosk just closed.
//
// It goes through the bridge rather than stamping the instance completed
// directly (#1747 review). Completion is not a status flip: the bridge
// finalizes the attendance first — genuinely expected children flip to
// 'absent', children the care plan does not book that day keep their expected
// row and are stamped as non-bookings. Marking the instance completed on its
// own would leave every expected row behind, which readers show as an
// unfinished day and which no later path ever repairs.
//
// Every failure is returned, not logged and swallowed (#1747 review). The
// caller runs inside the request's tenant transaction, so a returned error
// rolls the RFID session close back with it: either both sides of "Sitzung
// beenden" happen or neither does. Acknowledging the close while the mirrored
// instance stays active is the one outcome nothing later repairs — the kiosk
// has moved on, the block still counts as running, and its attendance is never
// finalized.
func (rs *Resource) completeMirroredTimetableInstance(ctx context.Context, activeGroupID int64) error {
	if activeGroupID <= 0 || rs.TimetableData == nil {
		// No mirroring wired at all: this deployment never created the
		// instance either, so there is nothing to complete.
		return nil
	}
	if rs.TimetableBridge == nil {
		// Mirroring is wired but its completion half is not. Sessions started
		// here DO create instances, so staying quiet would leak a permanently
		// active block per kiosk session. A wiring gap must fail visibly.
		return fmt.Errorf("timetable mirroring is wired without a completion bridge (active group %d)", activeGroupID)
	}
	inst, err := rs.TimetableData.GetInstanceByActiveGroupID(ctx, activeGroupID)
	if err != nil {
		return fmt.Errorf("find mirrored timetable instance for completion (active group %d): %w", activeGroupID, err)
	}
	if inst == nil {
		// The session was never mirrored (started before mirroring existed, or
		// by a path that does not mirror). Nothing to complete.
		return nil
	}
	now := time.Now()
	completed, err := rs.TimetableBridge.CompleteActiveByActiveGroupIDs(ctx, []int64{activeGroupID}, now)
	if err != nil {
		return fmt.Errorf("complete mirrored timetable instance %d: %w", inst.ID, err)
	}
	if completed == 0 {
		// Already completed by another path — nothing was finalized here, so
		// re-announcing it would be a phantom event.
		return nil
	}
	inst.Status = scheduleModel.InstanceStatusCompleted
	inst.CompletedAt = &now
	rs.broadcastMirroredInstance(ctx, realtime.EventInstanceCompleted, inst, &active.Group{RoomID: inst.RoomID})
	return nil
}

func (rs *Resource) timetableTitleForActivity(ctx context.Context, activityGroupID *int64) string {
	if activityGroupID == nil || rs.ActivitiesService == nil {
		return "RFID-Aktivität"
	}
	group, err := rs.ActivitiesService.GetGroup(ctx, *activityGroupID)
	if err != nil || group == nil || group.Name == "" {
		return "RFID-Aktivität"
	}
	return group.Name
}

func (rs *Resource) broadcastMirroredInstance(ctx context.Context, eventType realtime.EventType, inst *scheduleModel.ActivityInstance, activeGroup *active.Group) {
	if rs.Broadcaster == nil || inst == nil {
		return
	}
	instanceID := fmt.Sprintf("%d", inst.ID)
	instanceDate := inst.Date.Format("2006-01-02")
	instanceStart := inst.StartTime.Format("15:04:05")
	activeGroupID := ""
	if inst.ActiveGroupID != nil {
		activeGroupID = fmt.Sprintf("%d", *inst.ActiveGroupID)
	}
	data := realtime.EventData{
		InstanceID:        &instanceID,
		InstanceDate:      &instanceDate,
		InstanceStartTime: &instanceStart,
	}
	if activeGroup != nil && activeGroup.RoomID > 0 {
		roomID := fmt.Sprintf("%d", activeGroup.RoomID)
		data.RoomID = &roomID
	}
	event := realtime.NewEvent(eventType, activeGroupID, data)
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			slog.Default().WarnContext(ctx, "failed to broadcast mirrored timetable instance",
				slog.String("event_type", string(eventType)),
				slog.Int64("instance_id", inst.ID),
				slog.String("error", err.Error()),
			)
		}
	})
}

func clockFromMinutes(minutes int) time.Time {
	return time.Date(2000, 1, 1, minutes/60, minutes%60, 0, 0, time.UTC)
}

func firstPositiveID(ids []int64) *int64 {
	for _, id := range ids {
		if id > 0 {
			value := id
			return &value
		}
	}
	return nil
}
