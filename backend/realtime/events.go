// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
// This package is dependency-neutral to avoid circular imports between api and services layers.
package realtime

import "time"

// EventType represents the type of SSE event
type EventType string

// Event type constants
const (
	// Student movement events
	EventStudentCheckIn  EventType = "student_checkin"
	EventStudentCheckOut EventType = "student_checkout"
	EventStudentUpdated  EventType = "student_updated"

	// Activity session lifecycle events
	EventActivityStart  EventType = "activity_start"
	EventActivityEnd    EventType = "activity_end"
	EventActivityUpdate EventType = "activity_update"

	// Instance lifecycle events (WP-B9). Instances are the "planned" layer
	// between template (activities.*) and live (active.*). These fire on the
	// lifecycle transitions driven by admin/staff via the planner — distinct
	// from activity_start/end which are IoT/NFC driven and emit per active.group.
	EventInstanceStarted   EventType = "instance_started"
	EventInstanceCompleted EventType = "instance_completed"
	EventInstanceCancelled EventType = "instance_cancelled"
	EventInstanceOverdue   EventType = "instance_overdue"

	// Global refresh event — tells all clients to re-fetch dashboard counts
	EventDashboardCountsChanged EventType = "dashboard_counts_changed"

	// Active supervision refresh event — tenant-wide signal that the active
	// supervision view is stale regardless of whether the cause was IoT, NFC,
	// timetable operations, or another lifecycle action.
	EventActiveSupervisionChanged EventType = "active_supervision_changed"

	// Arrival schedule events affect derived "not arriving today" badges and
	// bulk arrival-time lookups across student list/detail pages.
	EventArrivalScheduleChanged EventType = "arrival_schedule_changed"

	// Tenant-scoped settings change. Fires when a setting whose value travels
	// through /auth/tenant/resolve (currently operations.student_photos_enabled)
	// is written or reset. Already-open tabs at the affected tenant pick this
	// up via SSE and re-resolve their TenantContext so feature gates flip
	// without a manual reload.
	//
	// This is the cross-origin counterpart to the in-browser BroadcastChannel
	// in lib/settings-broadcast.ts: BroadcastChannel only reaches tabs on the
	// same origin, so an operator-side write at operator.<domain> never
	// reaches tenant tabs at <slug>.<domain>. SSE crosses that boundary
	// because every authenticated tab holds an open connection. The Source
	// field carries the setting key so log review (and future selective
	// invalidation) can distinguish writers without parsing the payload.
	EventTenantSettingsChanged EventType = "tenant_settings_changed"
)

// Event represents a Server-Sent Event that will be broadcast to clients
type Event struct {
	Type          EventType `json:"type"`
	ActiveGroupID string    `json:"active_group_id"`
	Data          EventData `json:"data"`
	Timestamp     time.Time `json:"timestamp"`
}

// EventData contains the payload for an SSE event
// Only includes display-level data for GDPR compliance (no sensitive info)
type EventData struct {
	// Student-related fields (for check-in/check-out events)
	StudentID   *string `json:"student_id,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
	SchoolClass *string `json:"school_class,omitempty"`
	GroupName   *string `json:"group_name,omitempty"` // Student's OGS group, not active group

	// Activity session fields (for activity_start/end/update events)
	ActivityName  *string   `json:"activity_name,omitempty"`
	RoomID        *string   `json:"room_id,omitempty"`
	RoomName      *string   `json:"room_name,omitempty"`
	SupervisorIDs *[]string `json:"supervisor_ids,omitempty"`

	// Instance lifecycle fields (for instance_* events, WP-B9). String-typed
	// so the Event envelope's ActiveGroupID convention stays consistent and
	// so empty values serialize cleanly via omitempty.
	InstanceID        *string `json:"instance_id,omitempty"`
	InstanceDate      *string `json:"instance_date,omitempty"`       // YYYY-MM-DD
	InstanceStartTime *string `json:"instance_start_time,omitempty"` // HH:MM:SS

	// Attendance fields (WP-B10). Populated on student_checkin / student_checkout
	// when the visit corresponds to a schedule.instance_students row — null
	// otherwise (walk-ins, check-ins against non-instance active groups).
	AttendanceStatus    *string `json:"attendance_status,omitempty"`
	AttendanceSubstatus *string `json:"attendance_substatus,omitempty"`
	AttendanceNote      *string `json:"attendance_note,omitempty"`

	// Source tracking
	Source *string `json:"source,omitempty"` // "rfid", "manual", "automated"

	// Reason tracking for generic refresh events.
	Reason *string `json:"reason,omitempty"`
}

// NewEvent creates a new event with current timestamp
func NewEvent(eventType EventType, activeGroupID string, data EventData) Event {
	return Event{
		Type:          eventType,
		ActiveGroupID: activeGroupID,
		Data:          data,
		Timestamp:     time.Now(),
	}
}
