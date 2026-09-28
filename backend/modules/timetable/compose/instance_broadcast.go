package compose

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The realtime event types the lifecycle emits. They match the Delivery
// Platform's event vocabulary, which the composition root's binding passes
// through unchanged.
const (
	LifecycleEventInstanceStarted          = "instance_started"
	LifecycleEventInstanceCompleted        = "instance_completed"
	LifecycleEventInstanceCancelled        = "instance_cancelled"
	LifecycleEventActiveSupervisionChanged = "active_supervision_changed"
	LifecycleEventBulkStudentCheckIn       = "bulk_student_checkin"
	LifecycleEventDashboardCountsChanged   = "dashboard_counts_changed"
	LifecycleEventStaffingDeviationChanged = "staffing_deviation_changed"
	LifecycleEventActivityUpdate           = "activity_update"
)

// LifecycleEvent is one realtime notification of the lifecycle. Events are
// refetch triggers, not payloads; the optional fields mirror the event data
// the clients read.
type LifecycleEvent struct {
	Type              string
	ActiveGroupID     string
	InstanceID        *string
	InstanceDate      *string
	InstanceStartTime *string
	RoomID            *string
	RoomName          *string
	ActivityName      *string
	SupervisorIDs     *[]string
	StudentIDs        *[]string
	GroupIDs          *[]string
	Reason            *string
	Source            *string
}

// LifecycleBroadcaster delivers the lifecycle's events; the composition root
// binds the realtime hub.
type LifecycleBroadcaster interface {
	BroadcastToGroup(tenantID int64, topic string, event LifecycleEvent) error
	BroadcastToTenant(tenantID int64, event LifecycleEvent) error
}

// broadcastPlannedInstanceChanged queues one tenant-wide
// staffing_deviation_changed event after the surrounding tenant transaction
// commits. CRUD on planned blocks fires no lifecycle event, yet it changes
// who is planned where (#1844). source names the emitting flow.
func (s *InstanceLifecycleService) broadcastPlannedInstanceChanged(ctx context.Context, source string) {
	if s.deps.Broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	event := LifecycleEvent{Type: LifecycleEventStaffingDeviationChanged, Source: &source}
	tenant.RegisterAfterCommit(ctx, func() {
		if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.getLogger().Warn("SSE planned instance broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// broadcastInstanceEvent queues a lifecycle event after commit. Events with
// a live session route to its subscribers as well; every event reaches the
// tenant, so the admin dashboard sees a cancel-from-planned too.
func (s *InstanceLifecycleService) broadcastInstanceEvent(
	ctx context.Context,
	eventType string,
	instance *scheduleModel.ActivityInstance,
	activeGroup *studentpresence.LiveGroup,
	staffRows []*scheduleModel.InstanceStaff,
) {
	if s.deps.Broadcaster == nil || instance == nil {
		return
	}
	event := instanceEventBase(eventType, instance)
	if activeGroup != nil {
		s.describeStartedSession(ctx, &event, instance, activeGroup, staffRows)
	} else if instance.ActiveGroupID != nil {
		event.ActiveGroupID = strconv.FormatInt(*instance.ActiveGroupID, 10)
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		s.deliverInstanceEvent(tenantID, event)
	})
}

func instanceEventBase(eventType string, instance *scheduleModel.ActivityInstance) LifecycleEvent {
	instanceID := strconv.FormatInt(instance.ID, 10)
	instanceDate := instance.Date.String()
	instanceStart := instance.StartTime.Format("15:04:05")
	return LifecycleEvent{
		Type:              eventType,
		InstanceID:        &instanceID,
		InstanceDate:      &instanceDate,
		InstanceStartTime: &instanceStart,
	}
}

// describeStartedSession names the fresh session's room, activity and
// supervisors on a start event.
func (s *InstanceLifecycleService) describeStartedSession(
	ctx context.Context,
	event *LifecycleEvent,
	instance *scheduleModel.ActivityInstance,
	activeGroup *studentpresence.LiveGroup,
	staffRows []*scheduleModel.InstanceStaff,
) {
	event.ActiveGroupID = strconv.FormatInt(activeGroup.ID, 10)
	roomID := strconv.FormatInt(activeGroup.RoomID, 10)
	event.RoomID = &roomID
	if s.deps.Rooms != nil {
		if name, ok, err := s.deps.Rooms.RoomName(ctx, activeGroup.RoomID); err == nil && ok {
			event.RoomName = &name
		}
	}
	if instance.ActivityGroupID != nil && s.deps.ActivityGroupRepo != nil {
		if group, err := s.deps.ActivityGroupRepo.FindByID(ctx, *instance.ActivityGroupID); err == nil && group != nil {
			name := group.Name
			event.ActivityName = &name
		}
	}
	if len(staffRows) > 0 {
		ids := make([]string, 0, len(staffRows))
		for _, row := range staffRows {
			ids = append(ids, strconv.FormatInt(row.StaffID, 10))
		}
		event.SupervisorIDs = &ids
	}
}

func (s *InstanceLifecycleService) deliverInstanceEvent(tenantID int64, event LifecycleEvent) {
	if event.ActiveGroupID != "" {
		if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, event.ActiveGroupID, event); err != nil {
			s.getLogger().Warn("SSE broadcast failed",
				slog.String("event_type", event.Type),
				slog.String("active_group_id", event.ActiveGroupID),
				slog.String("error", err.Error()),
			)
		}
	}
	if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE broadcast-to-tenant failed",
			slog.String("event_type", event.Type),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}
	s.broadcastActiveSupervisionChanged(tenantID, event.ActiveGroupID, *event.InstanceID, event.Type)
}

func (s *InstanceLifecycleService) broadcastActiveSupervisionChanged(tenantID int64, activeGroupID, instanceID, sourceEventType string) {
	reason := instanceRefreshReason(sourceEventType)
	event := LifecycleEvent{
		Type:          LifecycleEventActiveSupervisionChanged,
		ActiveGroupID: activeGroupID,
		InstanceID:    &instanceID,
		Reason:        &reason,
	}
	if err := s.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE active supervision broadcast failed",
			slog.String("event_type", sourceEventType),
			slog.String("active_group_id", activeGroupID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}
}

func instanceRefreshReason(eventType string) string {
	switch eventType {
	case LifecycleEventInstanceStarted, LifecycleEventInstanceCompleted, LifecycleEventInstanceCancelled:
		return eventType
	default:
		return "instance_changed"
	}
}

// QueueActivityUpdates queues one activity_update per touched running block
// until the surrounding tenant transaction commits. The update is a refetch
// trigger and carries only the slot identity.
func (s *InstanceLifecycleService) QueueActivityUpdates(ctx context.Context, touched timetable.TouchedActivities) {
	if s.deps.Broadcaster == nil || len(touched) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	for activeGroupID, update := range touched {
		topic := strconv.FormatInt(activeGroupID, 10)
		instanceID := strconv.FormatInt(update.InstanceID, 10)
		instanceDate := update.Date.String()
		instanceStart := update.StartTime.Format("15:04:05")
		event := LifecycleEvent{
			Type:              LifecycleEventActivityUpdate,
			ActiveGroupID:     topic,
			InstanceID:        &instanceID,
			InstanceDate:      &instanceDate,
			InstanceStartTime: &instanceStart,
		}
		tenant.RegisterAfterCommit(ctx, func() {
			if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, topic, event); err != nil {
				s.getLogger().Warn("SSE activity update broadcast failed",
					slog.String("active_group_id", topic),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}

// touchedActivitiesOf maps the running blocks a staff move touched onto
// their slot identities.
func touchedActivitiesOf(touched map[int64]*scheduleModel.ActivityInstance) timetable.TouchedActivities {
	out := make(timetable.TouchedActivities, len(touched))
	for activeGroupID, instance := range touched {
		if instance != nil {
			out[activeGroupID] = timetable.TouchedActivity{
				InstanceID: instance.ID, Date: timezone.Date(instance.Date), StartTime: instance.StartTime,
			}
		}
	}
	return out
}
