// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
// This package stays independent from api and services layers to avoid circular imports.
package realtime

import (
	"strconv"
	"time"
)

// EventType represents the type of SSE event
type EventType string

// Event type constants
const (
	// Student movement events
	EventStudentCheckIn  EventType = "student_checkin"
	EventStudentCheckOut EventType = "student_checkout"
	EventStudentUpdated  EventType = "student_updated"

	// Bulk checkout event — emitted when a whole session ends (manual end or
	// scheduler timeout) and every active visit is closed at once. One event
	// per affected topic carrying all student IDs, instead of one
	// student_checkout per student, so a single client receives one trigger
	// rather than N (issue #848: per-student bursts overflowed the SSE channel
	// buffer and dropped events). Single-student checkouts keep emitting
	// EventStudentCheckOut. Like all SSE events this is a trigger, not a
	// payload — the client refetches via bulk endpoints; StudentIDs only drives
	// per-student detail-cache invalidation.
	EventBulkStudentCheckOut EventType = "bulk_student_checkout"

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

	// EventParentMessage signals that a parent sent a new message to the OGS
	// (or staff replied), so staff inbox / parent thread views refetch their
	// list and unread badge. Trigger only — clients refetch via the API.
	EventParentMessage EventType = "parent_message"
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

	// StudentIDs carries the affected students on a bulk_student_checkout event
	// (whole-session end). The client adds each to its per-student
	// detail-cache invalidation set; the refetch itself is topic-driven.
	StudentIDs *[]string `json:"student_ids,omitempty"`

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

	// Parent-messaging field (parent_message events). With StudentID, it lets an
	// open chat/conversation skip refetching when the event is about a different
	// thread/child — instead of every staff member and multi-child guardian
	// refetching the heavy inbox projection on every message tenant-wide. Trigger
	// only; the client still refetches the affected thread via the API.
	//
	// NOTE: ThreadID is an opaque conversation id (no child/guardian identity) and
	// is broadcast to staff; StudentID and Source (guardian account id) are NOT —
	// they are cleared for the staff fan-out by staffSafeParentMessage so an
	// unauthorized staffer cannot observe which child/guardian a thread concerns
	// from raw SSE traffic (gdpr.student_data_scope = group_supervisors_only). The
	// addressed guardian's own clients still receive the full event.
	ThreadID *string `json:"thread_id,omitempty"`

	// Source tracking
	Source *string `json:"source,omitempty"` // "rfid", "manual", "automated"

	// Reason tracking for generic refresh events.
	Reason *string `json:"reason,omitempty"`
}

// staffSafeParentMessage returns a copy of a parent-message event with the
// guardian-identifying fields cleared, for fan-out to staff who may not be
// authorized to read the thread. The guardian's OWN clients receive the full
// event (their own data); staff only need a refresh trigger for their
// access-filtered inbox. StudentID and Source (the guardian account id) are
// removed so an unauthorized staffer cannot observe which child/guardian a
// thread concerns from raw SSE traffic. ThreadID is retained — an opaque
// conversation id carrying no child/guardian identity — so an open staff thread
// can still refetch only the affected thread.
//
// The receiver is a value copy: reassigning the copy's pointer fields to nil
// leaves the original event (delivered to the guardian) untouched.
//
// Accepted consequence: with StudentID cleared, the per-student
// useMessagesActivity filter on the staff ParentMessagesCard cannot match, so
// every mounted card refetches its projection on any tenant-wide parent message
// (the studentId prop is intentionally INERT for staff — it only narrows the
// guardian's own clients). This is a deliberate privacy-over-efficiency trade:
// leaking which child a thread concerns to an unauthorized staffer is worse than
// an extra refetch. The 500 ms debounce on the staff side collapses a sick-note
// rush into far fewer fetches; ThreadID (opaque, no identity) is retained so an
// OPEN staff thread still refetches only itself.
func staffSafeParentMessage(event Event) Event {
	safe := event
	safe.Data.StudentID = nil
	safe.Data.Source = nil
	return safe
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

// NewParentMessageEvent builds the EventParentMessage SSE event shared by both
// portals' messaging services (staff in services/messaging and parents in
// services/parent). The guardian account is stamped as Source so the guardian's
// own tabs can skip a redundant refetch; threadID/studentID (when positive) let
// an open chat refetch only the affected thread. Centralizing the payload shape
// here means adding a field can never drift between the two emitters.
func NewParentMessageEvent(guardianAccountID, threadID, studentID int64) Event {
	gid := strconv.FormatInt(guardianAccountID, 10)
	data := EventData{Source: &gid}
	if threadID > 0 {
		tid := strconv.FormatInt(threadID, 10)
		data.ThreadID = &tid
	}
	if studentID > 0 {
		sid := strconv.FormatInt(studentID, 10)
		data.StudentID = &sid
	}
	return NewEvent(EventParentMessage, "", data)
}
