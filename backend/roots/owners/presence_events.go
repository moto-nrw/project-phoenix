package owners

import (
	"context"
	"fmt"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PresenceEvents binds Student Presence's event port to the realtime
// transport: it shapes each announcement into the transport's events and fans
// them out to the session, educational group and tenant topics of the
// caller's school. A failed fan-out is logged, never returned.
type PresenceEvents struct {
	publisher deliveryCompose.RealtimePublisher
	logger    *slog.Logger
}

// NewPresenceEvents binds the announcements to publisher. A nil logger logs
// to the default logger.
func NewPresenceEvents(publisher deliveryCompose.RealtimePublisher, logger *slog.Logger) PresenceEvents {
	return PresenceEvents{publisher: publisher, logger: logger}
}

const presenceBroadcastFailed = "SSE broadcast failed"

func (e PresenceEvents) log() *slog.Logger {
	if e.logger != nil {
		return e.logger
	}
	return slog.Default()
}

func educationGroupIDs(educationGroupID *int64) []string {
	if educationGroupID == nil {
		return nil
	}
	return []string{fmt.Sprintf("%d", *educationGroupID)}
}

func visitEvent(eventType realtime.EventType, change studentpresence.LiveVisitChange) realtime.Event {
	studentID := change.StudentID
	data := realtime.EventData{StudentID: &studentID}
	if change.Source != "" {
		source := change.Source
		data.Source = &source
	}
	if ids := educationGroupIDs(change.EducationGroupID); len(ids) > 0 {
		data.GroupIDs = &ids
	}
	if change.Attendance != nil {
		status := change.Attendance.Status
		data.AttendanceStatus = &status
		data.AttendanceSubstatus = change.Attendance.Substatus
		data.AttendanceNote = change.Attendance.Note
	}
	return realtime.NewEvent(eventType, change.ActiveGroupID, data)
}

// VisitCheckedIn announces a room check-in to the session topic and, when
// known, the child's educational group topic.
func (e PresenceEvents) VisitCheckedIn(ctx context.Context, change studentpresence.LiveVisitChange) {
	e.publishVisitChange(ctx, realtime.EventStudentCheckIn, change)
}

// VisitCheckedOut announces a room check-out to the session topic and, when
// known, the child's educational group topic.
func (e PresenceEvents) VisitCheckedOut(ctx context.Context, change studentpresence.LiveVisitChange) {
	e.publishVisitChange(ctx, realtime.EventStudentCheckOut, change)
}

func (e PresenceEvents) publishVisitChange(ctx context.Context, eventType realtime.EventType, change studentpresence.LiveVisitChange) {
	if e.publisher == nil {
		return
	}
	topics := []string{change.ActiveGroupID}
	if change.EducationGroupID != nil {
		topics = append(topics, realtime.EducationGroupTopic(*change.EducationGroupID))
	}
	if err := e.publisher.BroadcastToGroups(tenant.FromContext(ctx), topics, visitEvent(eventType, change)); err != nil {
		e.log().Error(presenceBroadcastFailed,
			slog.String("error", err.Error()),
			slog.String("event_type", string(eventType)),
			slog.String("active_group_id", change.ActiveGroupID),
			slog.String("student_id", change.StudentID),
		)
	}
}

// RoomlessAttendanceChanged announces an attendance change that has no room
// context to the child's educational group topic only. Without an
// educational group there is nobody to notify.
func (e PresenceEvents) RoomlessAttendanceChanged(ctx context.Context, checkIn bool, change studentpresence.LiveVisitChange) {
	eventType := realtime.EventStudentCheckOut
	if checkIn {
		eventType = realtime.EventStudentCheckIn
	}
	change.ActiveGroupID = ""
	e.publishToEducationGroup(ctx, change.EducationGroupID, visitEvent(eventType, change))
}

func (e PresenceEvents) publishToEducationGroup(ctx context.Context, educationGroupID *int64, event realtime.Event) {
	if e.publisher == nil || educationGroupID == nil {
		return
	}
	topic := realtime.EducationGroupTopic(*educationGroupID)
	if err := e.publisher.BroadcastToGroup(tenant.FromContext(ctx), topic, event); err != nil {
		studentID := ""
		if event.Data.StudentID != nil {
			studentID = *event.Data.StudentID
		}
		e.log().Error(presenceBroadcastFailed+" for educational topic",
			slog.String("error", err.Error()),
			slog.String("event_type", string(event.Type)),
			slog.String("education_group_topic", topic),
			slog.String("student_id", studentID),
		)
	}
}

func bulkEventType(checkIn bool) realtime.EventType {
	if checkIn {
		return realtime.EventBulkStudentCheckIn
	}
	return realtime.EventBulkStudentCheckOut
}

// BulkStudentChange announces one batched check-in or check-out to a session
// topic. groupIDs are the affected educational groups; nil keeps the field
// absent so clients refresh broadly.
func (e PresenceEvents) BulkStudentChange(ctx context.Context, checkIn bool, activeGroupID string, studentIDs, groupIDs []string) {
	if e.publisher == nil {
		return
	}
	data := realtime.EventData{StudentIDs: &studentIDs}
	if len(groupIDs) > 0 {
		ids := groupIDs
		data.GroupIDs = &ids
	}
	e.publishToActiveGroup(ctx, activeGroupID, realtime.NewEvent(bulkEventType(checkIn), activeGroupID, data))
}

// BulkStudentChangeToEducationGroup announces the subset of a batched change
// that belongs to one educational group on that group's topic.
func (e PresenceEvents) BulkStudentChangeToEducationGroup(ctx context.Context, checkIn bool, activeGroupID string, educationGroupID int64, studentIDs []string) {
	ids := []string{fmt.Sprintf("%d", educationGroupID)}
	event := realtime.NewEvent(bulkEventType(checkIn), activeGroupID, realtime.EventData{StudentIDs: &studentIDs, GroupIDs: &ids})
	e.publishToEducationGroup(ctx, &educationGroupID, event)
}

// SessionStarted announces a started session to its topic.
func (e PresenceEvents) SessionStarted(ctx context.Context, session studentpresence.LiveSessionChange) {
	if e.publisher == nil {
		return
	}
	activityName, roomID, roomName := session.ActivityName, session.RoomID, session.RoomName
	supervisorIDs := session.SupervisorIDs
	event := realtime.NewEvent(realtime.EventActivityStart, session.ActiveGroupID, realtime.EventData{
		ActivityName: &activityName, RoomID: &roomID, RoomName: &roomName, SupervisorIDs: &supervisorIDs,
	})
	e.publishToActiveGroup(ctx, session.ActiveGroupID, event)
}

// SessionEnded announces an ended session to its topic.
func (e PresenceEvents) SessionEnded(ctx context.Context, session studentpresence.LiveSessionChange) {
	if e.publisher == nil {
		return
	}
	activityName, roomID, roomName := session.ActivityName, session.RoomID, session.RoomName
	event := realtime.NewEvent(realtime.EventActivityEnd, session.ActiveGroupID, realtime.EventData{
		ActivityName: &activityName, RoomID: &roomID, RoomName: &roomName,
	})
	e.publishToActiveGroup(ctx, session.ActiveGroupID, event)
}

func (e PresenceEvents) publishToActiveGroup(ctx context.Context, activeGroupID string, event realtime.Event) {
	if err := e.publisher.BroadcastToGroup(tenant.FromContext(ctx), activeGroupID, event); err != nil {
		e.log().Error(presenceBroadcastFailed,
			slog.String("error", err.Error()),
			slog.String("event_type", string(event.Type)),
			slog.String("active_group_id", activeGroupID),
		)
	}
}

// SupervisionRefresh sends the tenant-wide refresh used by attendance and
// activity changes. The event carries the union of the former dashboard-count
// and active-supervision invalidation scopes and no child identity (#2085).
// eduGroupIDs scope the client refresh; nil refreshes broadly.
func (e PresenceEvents) SupervisionRefresh(ctx context.Context, activeGroupID, reason string, eduGroupIDs []string) {
	if e.publisher == nil {
		return
	}
	data := realtime.EventData{Reason: &reason}
	if len(eduGroupIDs) > 0 {
		ids := eduGroupIDs
		data.GroupIDs = &ids
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, activeGroupID, data)
	if err := e.publisher.BroadcastToTenant(tenantID, event); err != nil {
		e.log().Warn("SSE combined supervision broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
			slog.Int64("tenant_id", tenantID),
		)
	}
}

// SupervisionChanged queues active_supervision_changed after the commit through
// Delivery's existing after-commit queue (#5017).
func (e PresenceEvents) SupervisionChanged(ctx context.Context, activeGroupID int64, reason string) {
	if e.publisher == nil {
		return
	}
	deliveryCompose.NewGroupEvents(e.publisher, e.log()).QueueActiveSupervisionChanged(ctx, activeGroupID, reason)
}

// DashboardCountsChanged sends the tenant-wide dashboard refresh signal
// (#2057). eduGroupIDs are the affected educational group ids, never student
// identity; nil omits the field so clients refresh broadly.
func (e PresenceEvents) DashboardCountsChanged(ctx context.Context, eduGroupIDs []string) {
	if e.publisher == nil {
		return
	}
	data := realtime.EventData{}
	if len(eduGroupIDs) > 0 {
		ids := eduGroupIDs
		data.GroupIDs = &ids
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, "", data)
	if err := e.publisher.BroadcastToTenant(tenantID, event); err != nil {
		e.log().Warn("SSE dashboard counts broadcast failed",
			slog.String("error", err.Error()),
			slog.Int64("tenant_id", tenantID),
		)
	}
}
