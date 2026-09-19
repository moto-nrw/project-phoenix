package realtimeevents

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Publisher is the realtime fan-out the presence producers write to. It is
// satisfied by the realtime hub and by the recording double in test support.
type Publisher interface {
	BroadcastToGroup(tenantID int64, activeGroupID string, event realtime.Event) error
	BroadcastToGroups(tenantID int64, groupIDs []string, event realtime.Event) error
	BroadcastToTenant(tenantID int64, event realtime.Event) error
}

const presenceBroadcastFailed = "SSE broadcast failed"

// AttendanceDetail carries the timetable attendance fields a visit event may
// carry (WP-B10). Substatus and Note stay absent when nil.
type AttendanceDetail struct {
	Status    string
	Substatus *string
	Note      *string
}

// VisitChange describes one child's room check-in or check-out. An empty
// ActiveGroupID marks a roomless attendance change. EducationGroupID scopes
// the educational group topic and the client-side invalidation; Source names
// the emitting flow and is omitted when empty.
type VisitChange struct {
	ActiveGroupID    string
	StudentID        string
	EducationGroupID *int64
	Source           string
	Attendance       *AttendanceDetail
}

// ActivitySession describes a session start or end for the room roster.
type ActivitySession struct {
	ActiveGroupID string
	ActivityName  string
	RoomID        string
	RoomName      string
	SupervisorIDs []string
}

func educationTopic(educationGroupID int64) string {
	return fmt.Sprintf("edu:%d", educationGroupID)
}

func educationGroupIDs(educationGroupID *int64) []string {
	if educationGroupID == nil {
		return nil
	}
	return []string{fmt.Sprintf("%d", *educationGroupID)}
}

func visitEvent(eventType realtime.EventType, change VisitChange) realtime.Event {
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

// PublishVisitCheckIn announces a room check-in to the session topic and, when
// known, the child's educational group topic.
func PublishVisitCheckIn(ctx context.Context, publisher Publisher, logger *slog.Logger, change VisitChange) {
	publishVisitChange(ctx, publisher, logger, realtime.EventStudentCheckIn, change)
}

// PublishVisitCheckOut announces a room check-out to the session topic and,
// when known, the child's educational group topic.
func PublishVisitCheckOut(ctx context.Context, publisher Publisher, logger *slog.Logger, change VisitChange) {
	publishVisitChange(ctx, publisher, logger, realtime.EventStudentCheckOut, change)
}

func publishVisitChange(ctx context.Context, publisher Publisher, logger *slog.Logger, eventType realtime.EventType, change VisitChange) {
	if publisher == nil {
		return
	}
	topics := []string{change.ActiveGroupID}
	if change.EducationGroupID != nil {
		topics = append(topics, educationTopic(*change.EducationGroupID))
	}
	if err := publisher.BroadcastToGroups(tenant.FromContext(ctx), topics, visitEvent(eventType, change)); err != nil {
		loggerOrDefault(logger).Error(presenceBroadcastFailed,
			slog.String("error", err.Error()),
			slog.String("event_type", string(eventType)),
			slog.String("active_group_id", change.ActiveGroupID),
			slog.String("student_id", change.StudentID),
		)
	}
}

// PublishRoomlessAttendanceChange announces an attendance change that has no
// room context to the child's educational group topic only. Without an
// educational group there is nobody to notify.
func PublishRoomlessAttendanceChange(ctx context.Context, publisher Publisher, logger *slog.Logger, checkIn bool, change VisitChange) {
	eventType := realtime.EventStudentCheckOut
	if checkIn {
		eventType = realtime.EventStudentCheckIn
	}
	change.ActiveGroupID = ""
	publishToEducationGroup(ctx, publisher, logger, change.EducationGroupID, visitEvent(eventType, change))
}

func publishToEducationGroup(ctx context.Context, publisher Publisher, logger *slog.Logger, educationGroupID *int64, event realtime.Event) {
	if publisher == nil || educationGroupID == nil {
		return
	}
	topic := educationTopic(*educationGroupID)
	if err := publisher.BroadcastToGroup(tenant.FromContext(ctx), topic, event); err != nil {
		studentID := ""
		if event.Data.StudentID != nil {
			studentID = *event.Data.StudentID
		}
		loggerOrDefault(logger).Error(presenceBroadcastFailed+" for educational topic",
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

// PublishBulkStudentChange announces one batched check-in or check-out to a
// session topic. groupIDs are the affected educational groups; nil keeps the
// field absent so clients refresh broadly.
func PublishBulkStudentChange(ctx context.Context, publisher Publisher, logger *slog.Logger, checkIn bool, activeGroupID string, studentIDs, groupIDs []string) {
	if publisher == nil {
		return
	}
	data := realtime.EventData{StudentIDs: &studentIDs}
	if len(groupIDs) > 0 {
		ids := groupIDs
		data.GroupIDs = &ids
	}
	eventType := bulkEventType(checkIn)
	publishToActiveGroup(ctx, publisher, logger, activeGroupID, realtime.NewEvent(eventType, activeGroupID, data))
}

// PublishBulkStudentChangeToEducationGroup announces the subset of a batched
// change that belongs to one educational group on that group's topic.
func PublishBulkStudentChangeToEducationGroup(ctx context.Context, publisher Publisher, logger *slog.Logger, checkIn bool, activeGroupID string, educationGroupID int64, studentIDs []string) {
	ids := []string{fmt.Sprintf("%d", educationGroupID)}
	event := realtime.NewEvent(bulkEventType(checkIn), activeGroupID, realtime.EventData{StudentIDs: &studentIDs, GroupIDs: &ids})
	publishToEducationGroup(ctx, publisher, logger, &educationGroupID, event)
}

// PublishActivityStart announces a started session to its topic.
func PublishActivityStart(ctx context.Context, publisher Publisher, logger *slog.Logger, session ActivitySession) {
	if publisher == nil {
		return
	}
	activityName, roomID, roomName := session.ActivityName, session.RoomID, session.RoomName
	supervisorIDs := session.SupervisorIDs
	event := realtime.NewEvent(realtime.EventActivityStart, session.ActiveGroupID, realtime.EventData{
		ActivityName: &activityName, RoomID: &roomID, RoomName: &roomName, SupervisorIDs: &supervisorIDs,
	})
	publishToActiveGroup(ctx, publisher, logger, session.ActiveGroupID, event)
}

// PublishActivityEnd announces an ended session to its topic.
func PublishActivityEnd(ctx context.Context, publisher Publisher, logger *slog.Logger, session ActivitySession) {
	if publisher == nil {
		return
	}
	activityName, roomID, roomName := session.ActivityName, session.RoomID, session.RoomName
	event := realtime.NewEvent(realtime.EventActivityEnd, session.ActiveGroupID, realtime.EventData{
		ActivityName: &activityName, RoomID: &roomID, RoomName: &roomName,
	})
	publishToActiveGroup(ctx, publisher, logger, session.ActiveGroupID, event)
}

func publishToActiveGroup(ctx context.Context, publisher Publisher, logger *slog.Logger, activeGroupID string, event realtime.Event) {
	if err := publisher.BroadcastToGroup(tenant.FromContext(ctx), activeGroupID, event); err != nil {
		loggerOrDefault(logger).Error(presenceBroadcastFailed,
			slog.String("error", err.Error()),
			slog.String("event_type", string(event.Type)),
			slog.String("active_group_id", activeGroupID),
		)
	}
}

// PublishSupervisionRefresh sends the tenant-wide refresh used by attendance
// and activity changes. The event carries the union of the former
// dashboard-count and active-supervision invalidation scopes and no child
// identity (#2085). eduGroupIDs scope the client refresh; nil refreshes
// broadly.
func PublishSupervisionRefresh(ctx context.Context, publisher Publisher, logger *slog.Logger, activeGroupID, reason string, eduGroupIDs []string) {
	if publisher == nil {
		return
	}
	data := realtime.EventData{Reason: &reason}
	if len(eduGroupIDs) > 0 {
		ids := eduGroupIDs
		data.GroupIDs = &ids
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, activeGroupID, data)
	if err := publisher.BroadcastToTenant(tenantID, event); err != nil {
		loggerOrDefault(logger).Warn("SSE combined supervision broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
			slog.Int64("tenant_id", tenantID),
		)
	}
}

// PublishDashboardCountsChanged sends the tenant-wide dashboard refresh signal
// (#2057). eduGroupIDs are the affected educational group ids, never student
// identity; nil omits the field so clients refresh broadly.
func PublishDashboardCountsChanged(ctx context.Context, publisher Publisher, logger *slog.Logger, eduGroupIDs []string) {
	if publisher == nil {
		return
	}
	data := realtime.EventData{}
	if len(eduGroupIDs) > 0 {
		ids := eduGroupIDs
		data.GroupIDs = &ids
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventDashboardCountsChanged, "", data)
	if err := publisher.BroadcastToTenant(tenantID, event); err != nil {
		loggerOrDefault(logger).Warn("SSE dashboard counts broadcast failed",
			slog.String("error", err.Error()),
			slog.Int64("tenant_id", tenantID),
		)
	}
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
