package compose

import (
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/ports"
)

const (
	refreshReasonStudentMoved  = "student_moved"
	refreshReasonActivityEnded = "activity_ended"
)

// notifier announces a committed session end over SSE and wakes the guardians
// of every checked-out child. It runs from an after-commit hook: there is no
// tenant transaction and no tenant role, so it must not touch the database
// (#2951). Everything it needs arrives in the notification.
//
// The event shapes are the ones the kiosks, dashboards, and the parents'
// app already consume: one bulk_student_checkout per session on the
// active-group topic and one per affected education group, a tenant-wide
// dashboard refresh scoped to those groups, activity_end on the active-group
// topic with a broad refresh, and instance_completed for the mirrored block.
type notifier struct {
	broadcaster realtime.Broadcaster
	guardians   ports.GuardianWaker
	logger      *slog.Logger
}

func (n *notifier) log() *slog.Logger {
	if n.logger == nil {
		return slog.Default()
	}
	return n.logger
}

func (n *notifier) SessionEnded(note ports.Notification) {
	activeGroupID := strconv.FormatInt(note.ActiveGroupID, 10)
	n.announceCheckouts(note, activeGroupID)
	n.announceActivityEnd(note, activeGroupID)
	n.announceInstanceCompleted(note, activeGroupID)
	n.wakeGuardians(note)
}

func (n *notifier) announceCheckouts(note ports.Notification, activeGroupID string) {
	if len(note.Students) == 0 {
		return
	}
	studentIDs := make([]string, 0, len(note.Students))
	byGroup := make(map[int64][]string)
	groupOrder := make([]int64, 0)
	for _, student := range note.Students {
		id := strconv.FormatInt(student.StudentID, 10)
		studentIDs = append(studentIDs, id)
		if student.EducationGroupID == nil {
			continue
		}
		gid := *student.EducationGroupID
		if _, seen := byGroup[gid]; !seen {
			groupOrder = append(groupOrder, gid)
		}
		byGroup[gid] = append(byGroup[gid], id)
	}
	// Group ids only, never student identity, on the tenant-wide refresh.
	// Absence of the field means "refresh broadly": emit nil, not an empty
	// list.
	var groupIDs []string
	for _, gid := range groupOrder {
		groupIDs = append(groupIDs, strconv.FormatInt(gid, 10))
	}

	data := realtime.EventData{StudentIDs: &studentIDs}
	if len(groupIDs) > 0 {
		data.GroupIDs = &groupIDs
	}
	n.toGroup(note.TenantID, activeGroupID, realtime.NewEvent(realtime.EventBulkStudentCheckOut, activeGroupID, data))

	for _, gid := range groupOrder {
		ids := byGroup[gid]
		scope := []string{strconv.FormatInt(gid, 10)}
		event := realtime.NewEvent(realtime.EventBulkStudentCheckOut, activeGroupID, realtime.EventData{StudentIDs: &ids, GroupIDs: &scope})
		n.toGroup(note.TenantID, "edu:"+scope[0], event)
	}
	n.refreshDashboards(note.TenantID, activeGroupID, refreshReasonStudentMoved, groupIDs)
}

func (n *notifier) announceActivityEnd(note ports.Notification, activeGroupID string) {
	roomID := strconv.FormatInt(note.RoomID, 10)
	activityName := note.ActivityName
	roomName := note.RoomName
	event := realtime.NewEvent(realtime.EventActivityEnd, activeGroupID, realtime.EventData{
		ActivityName: &activityName,
		RoomID:       &roomID,
		RoomName:     &roomName,
	})
	n.toGroup(note.TenantID, activeGroupID, event)
	// A session end changes room occupancy across groups: no group scope.
	n.refreshDashboards(note.TenantID, activeGroupID, refreshReasonActivityEnded, nil)
}

func (n *notifier) announceInstanceCompleted(note ports.Notification, activeGroupID string) {
	if note.Instance == nil {
		return
	}
	instanceID := strconv.FormatInt(note.Instance.ID, 10)
	date := note.Instance.Date
	start := note.Instance.StartTime
	data := realtime.EventData{InstanceID: &instanceID, InstanceDate: &date, InstanceStartTime: &start}
	if note.Instance.RoomID > 0 {
		roomID := strconv.FormatInt(note.Instance.RoomID, 10)
		data.RoomID = &roomID
	}
	if err := n.broadcaster.BroadcastToTenant(note.TenantID, realtime.NewEvent(realtime.EventInstanceCompleted, activeGroupID, data)); err != nil {
		n.log().Warn("session end: instance completed broadcast failed",
			slog.String("error", err.Error()),
			slog.Int64("instance_id", note.Instance.ID),
			slog.String("active_group_id", activeGroupID),
		)
	}
}

func (n *notifier) wakeGuardians(note ports.Notification) {
	if n.guardians == nil || note.TenantID <= 0 {
		return
	}
	for _, student := range note.Students {
		n.guardians.BroadcastChildUpdateToGuardians(note.TenantID, student.StudentID)
	}
}

func (n *notifier) refreshDashboards(tenantID int64, activeGroupID, reason string, groupIDs []string) {
	data := realtime.EventData{Reason: &reason}
	if len(groupIDs) > 0 {
		data.GroupIDs = &groupIDs
	}
	if err := n.broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventDashboardCountsChanged, activeGroupID, data)); err != nil {
		n.log().Warn("session end: dashboard refresh broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
		)
	}
}

func (n *notifier) toGroup(tenantID int64, topic string, event realtime.Event) {
	if err := n.broadcaster.BroadcastToGroup(tenantID, topic, event); err != nil {
		n.log().Warn("session end: SSE broadcast failed",
			slog.String("error", err.Error()),
			slog.String("event_type", string(event.Type)),
			slog.String("topic", topic),
		)
	}
}
