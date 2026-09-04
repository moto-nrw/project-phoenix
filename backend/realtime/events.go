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

	// EventStudentCompanionsChanged fires when a write may have changed a
	// child's Laufgemeinschaft ("läuft mit", users.student_companions): a
	// submitted companion list, or a departure-plan write, which trims the links
	// on a weekday the new plan no longer allows.
	//
	// It exists BECAUSE student_updated is far too broad for this: the links are
	// symmetric and the editing forms hold a draft they must not overwrite, so a
	// client that treats every student_updated as a companion write blocks an
	// in-progress edit whenever anyone renames a child, uploads a photo or marks
	// someone sick. Emitted IN ADDITION to student_updated (never instead of
	// it), tenant-scoped like its sibling.
	EventStudentCompanionsChanged EventType = "student_companions_changed"

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

	// Bulk check-in counterpart of EventBulkStudentCheckOut. Used when many
	// visits become live at once (activity reopen restoring a snapshot) so
	// clients get one location-cache invalidation instead of N student_checkin
	// events. Same payload contract as the checkout batch: StudentIDs on
	// group-scoped topics only, GroupIDs for OGS list scoping.
	EventBulkStudentCheckIn EventType = "bulk_student_checkin"

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

	// EventStaffingDeviationChanged is a tenant-wide signal that planned
	// staffing state changed on an instance — an absence was set or cleared, a
	// substitute assigned, the deliberately-unstaffed acknowledgement toggled,
	// or a still-planned instance was created, edited (title/time/room/staff,
	// incl. a date move), or deleted (#1840/#1844). The instance_* lifecycle
	// events do NOT cover these writes: they change no lifecycle status, and on
	// an active block only a group-scoped activity_update fires, which the
	// affected staff member may not be subscribed to. Clients treat it like an
	// instance lifecycle trigger and refetch timetable + own-assignment caches.
	// The Source field names the emitting flow (e.g. "deviations",
	// "understaffed_ack", "instance_create", "instance_update",
	// "instance_delete").
	EventStaffingDeviationChanged EventType = "staffing_deviation_changed"

	// Tenant-wide refresh event — tells every client of the tenant to re-fetch
	// dashboard counts. Attendance/activity emitters use this wire type for their
	// combined dashboard and supervision refresh; those events carry
	// ActiveGroupID and Reason in addition to GroupIDs (#2115).
	// Broadcast via BroadcastToTenant (issue #2057; it was
	// BroadcastToAll before, which fanned every school's check-in traffic out to
	// every OTHER school's clients). Carries GroupIDs (educational group ids,
	// never student identity) when the emitting site knows them, so clients can
	// scope their ogs-students-{gid} revalidation; without GroupIDs clients fall
	// back to a broad refresh.
	EventDashboardCountsChanged EventType = "dashboard_counts_changed"

	// EventGroupAccessChanged is a tenant-wide signal that WHICH educational
	// groups a staff member may open in "Meine Gruppe" has changed.
	//
	// That set is the union of exactly two tables (usercontext.GetMyGroups):
	// education.group_teacher — the group's leaders, edited in the group admin
	// UI — and education.group_substitution, written both by the admin
	// Vertretungsplan and by the self-service Gruppenübergabe. The event is
	// named for the invariant rather than for either table, so a new writer of
	// either one has an obvious signal to reuse instead of inventing a second.
	//
	// It exists BECAUSE the affected staff member's own client never made the
	// write and no other event covers it: a handover or a leader reassignment
	// changes no attendance, no activity session and no timetable block, so
	// none of the check-in, activity or instance events fire. Until #2083 the
	// visibility rode along by accident — the OGS page's BFF was refetched on
	// every check-in anywhere in the school — and after that refetch herd was
	// removed, a group handed to someone stayed invisible in their open tab
	// until the five-minute safety poll or a manual reload (#2084).
	//
	// Carries no staff identifier: the event reaches every staff client of the
	// tenant, and naming the person would tell colleagues outside the affected
	// group who is standing in for whom. Clients refetch their own
	// access-filtered views (Source names the emitting flow for log review).
	EventGroupAccessChanged EventType = "group_access_changed"

	// EventStaffTimeTrackingChanged is a tenant-wide invalidation trigger for
	// writes that change staff work sessions, absences, balances, contractual
	// schedules, or planned shifts. It carries no staff identifier: authorized
	// clients re-fetch their permission-scoped time-tracking views.
	EventStaffTimeTrackingChanged EventType = "staff_time_tracking_changed"

	// Active supervision refresh event — a tenant-wide signal emitted by
	// timetable operations, schedule instance lifecycle changes, and the overdue
	// scheduler. Attendance hot paths fold the same invalidation into
	// dashboard_counts_changed instead (#2115).
	//
	// Carries NO child identity (#2085), for the same reason
	// arrival_schedule_changed and pickup_schedule_changed are id-less: it
	// reaches every staff client of the school, so a student id would let a
	// client without student read access (guest/guardian, #2329) read
	// off the raw SSE stream which child had just moved — the very thing the
	// API responses redact for that person. It may carry the active_group_id,
	// instance_id and a reason: room/session/instance scope, never who.
	//
	// Attendance invalidates per-child detail caches through the separate
	// group-scoped student_checkin / student_checkout events, which reach exactly
	// the supervisors entitled to that child's data.
	EventActiveSupervisionChanged EventType = "active_supervision_changed"

	// Arrival schedule events affect derived "not arriving today" badges and
	// bulk arrival-time lookups across student list/detail pages.
	EventArrivalScheduleChanged EventType = "arrival_schedule_changed"

	// EventPickupScheduleChanged is the pickup-side counterpart of
	// EventArrivalScheduleChanged: a staff write changed a child's weekly
	// pickup plan, a date-specific pickup exception, or a day note (Gehzeit —
	// all three travel in the same pickup payload). It is its own
	// event rather than a reuse of the arrival one so clients can invalidate
	// only the pickup-derived caches, and so the arrival name keeps meaning
	// what it says.
	//
	// It exists BECAUSE the staff pickup handlers previously woke only the
	// child's guardians (#1725): the parents app updated live while every OTHER
	// staff tab — the per-child Betreuungsplan, the student list's Gehzeit
	// column — kept the old time indefinitely (those views disable focus
	// revalidation, so nothing else would refetch). Broadcast to ALL like its
	// arrival sibling: pickup times are read across tenants' dashboards and
	// list pages, and the payload carries no student data.
	EventPickupScheduleChanged EventType = "pickup_schedule_changed"

	// EventChangeRequestsChanged is a tenant-wide staff signal that a parent
	// change-request queue changed — a request was created, decided, or withdrawn
	// (excused absence today; care-schedule / master-data can adopt it too). Staff
	// review lists and the "Änderungsanfragen" pending-count badge refetch on it,
	// so an already-open review page updates in real time WITHOUT depending on the
	// parent-messaging pill (which the school may have disabled). Unlike
	// student_updated it fires ONLY on a request transition — never on the
	// high-frequency check-in/out traffic — so listeners can refetch unconditionally.
	EventChangeRequestsChanged EventType = "change_requests_changed"

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

	// EventParentMessageRead signals that the COUNTERPART in a parent-OGS thread
	// has read the conversation, so the other side's open chat refreshes its
	// "Gelesen" receipts live. Unlike EventParentMessage it adds no message: the
	// client re-decorates read receipts only and must NOT bump any unread badge (a
	// counterpart's read never changes the reader's own unread set). It is emitted
	// ONLY when a read cursor actually advances (parentmessaging.MarkReadToNewest),
	// so it cannot ping-pong with the receipt refetch it triggers on the far side.
	EventParentMessageRead EventType = "parent_message_read"

	// EventStaffMessage signals a new message in an OGS-INTERNAL colleague
	// conversation (#2598), so the participants' inbox, open chat and unread
	// badge refetch. Trigger only — clients refetch via the API.
	//
	// Unlike EventParentMessage this one KEEPS its thread_id on the wire. That
	// event fans out to every staff client in the tenant, so it is stripped by
	// staffSafeParentMessage to stop an unauthorized staffer correlating events
	// to a conversation. This one is delivered with BroadcastToStaffAccounts to
	// the thread's participants ONLY, so the id reaches nobody who may not open
	// the thread anyway — and carrying it lets an open chat refetch just that
	// conversation instead of every mounted messaging surface.
	EventStaffMessage EventType = "staff_message"

	// EventParentChildUpdated is a message-INDEPENDENT invalidation delivered to
	// EVERY guardian of a child (not just the one who acted) so an already-open
	// parents-app tab refetches that child's care state in real time — after a
	// parent write or a staff decision — regardless of whether parent messaging is
	// enabled. Unlike EventParentMessage it never accompanies a thread/pill and
	// must NOT touch any unread badge or thread list; it carries only student_id so
	// the affected child's view refetches while others skip. Trigger only.
	EventParentChildUpdated EventType = "parent_child_updated"

	// EventNotification carries a user-facing notification from the
	// notification abstraction (services/notifications, #1624) over the SSE
	// channel. Unlike every other event type it is NOT a cache-invalidation
	// trigger: clients render Title/Body directly (toast/bell). The payload is
	// display-safe by contract — the notifications service only accepts
	// non-sensitive title/body text and an app-relative deep link; sensitive
	// details are loaded authenticated after following the link (GDPR).
	EventNotification EventType = "notification"
)

// Event represents a Server-Sent Event that will be broadcast to clients
type Event struct {
	Type          EventType `json:"type"`
	ActiveGroupID string    `json:"active_group_id"`
	Data          EventData `json:"data"`
	Timestamp     time.Time `json:"timestamp"`
}

// EventData contains the payload for an SSE event. Fields follow the
// event-specific GDPR contract and must not expose sensitive data.
type EventData struct {
	// Student-related fields (for check-in/check-out events)
	StudentID *string `json:"student_id,omitempty"`

	// StudentIDs carries the affected students on a bulk_student_checkout or
	// bulk_student_checkin event. The client adds each to its per-student
	// detail-cache invalidation set; the refetch itself is topic-driven.
	StudentIDs *[]string `json:"student_ids,omitempty"`

	// GroupIDs carries the affected educational (OGS) group ids on
	// active_supervision_changed / dashboard_counts_changed / student_checkin / student_checkout /
	// bulk_student_checkout / bulk_student_checkin (#2057). Group ids only — NEVER student identity —
	// so the tenant-wide dashboard_counts_changed stays GDPR-safe under
	// student read access: it reveals "counts in group X changed", nothing
	// about who. Clients scope their ogs-students-{gid} revalidation to these
	// ids. Contract: absence of the field means "scope unknown → refresh
	// broadly"; emitters MUST omit the field entirely (nil) instead of sending
	// an empty array, which clients would read as "scope to nothing" and
	// silently drop the invalidation.
	GroupIDs *[]string `json:"group_ids,omitempty"`

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
	// NOTE: ThreadID, StudentID, and Source (guardian account id) are all cleared
	// for the staff fan-out by staffSafeParentMessage so an unauthorized staffer
	// cannot observe which child/guardian — or which specific conversation — a
	// thread concerns from raw SSE traffic (student read access,
	// group_supervisors_only). ThreadID is opaque, but a staffer who once opened a
	// thread (or otherwise knows its URL) and later loses access to that child
	// could still correlate future parent_message events to that conversation, so
	// it is stripped too. The addressed guardian's own clients still receive the
	// full event.
	ThreadID *string `json:"thread_id,omitempty"`

	// Source tracking
	Source *string `json:"source,omitempty"` // "rfid", "manual", "automated"

	// Reason tracking for generic refresh events.
	Reason *string `json:"reason,omitempty"`

	// Notification fields (notification events only, #1624). Display-safe by
	// contract: the notifications service validates that no sensitive student
	// data travels in the visible fields — clients render these directly instead
	// of refetching. NotificationData contains only opaque routing IDs.
	Title            *string           `json:"title,omitempty"`
	Body             *string           `json:"body,omitempty"`
	DeepLink         *string           `json:"deep_link,omitempty"`        // app-relative path, e.g. "/reminders"
	SchoolDeepLink   *string           `json:"school_deep_link,omitempty"` // the same destination on the school portal (#2208)
	Priority         *string           `json:"priority,omitempty"`         // "low" | "normal" | "high"
	NotificationType *string           `json:"notification_type,omitempty"`
	NotificationData map[string]string `json:"notification_data,omitempty"` // opaque IDs for client-side routing
}

// staffSafeParentMessage returns a copy of a parent-message event with every
// conversation-identifying field cleared, for fan-out to staff who may not be
// authorized to read the thread. The guardian's OWN clients receive the full
// event (their own data); staff only need a refresh trigger for their
// access-filtered inbox. ThreadID, StudentID, and Source (the guardian account
// id) are all removed so an unauthorized staffer cannot observe which
// child/guardian — or which specific conversation — a thread concerns from raw
// SSE traffic. ThreadID is opaque, but a staffer who once opened a thread (or
// otherwise knows its URL) and later loses access to that child could still
// correlate future parent_message events to that conversation through the
// per-thread useMessagesActivity filter, even though GET
// /api/messages/threads/{id} would now be forbidden — so it is stripped too.
//
// The receiver is a value copy: reassigning the copy's pointer fields to nil
// leaves the original event (delivered to the guardian) untouched.
//
// Accepted consequence: with ThreadID and StudentID both cleared, neither the
// per-thread filter on the staff thread page nor the per-student filter on the
// staff ParentMessagesCard can match, so every mounted staff messaging surface
// refetches its projection on any tenant-wide parent message (the threadId and
// studentId props are intentionally INERT for staff — they only narrow the
// guardian's own clients). This is a deliberate privacy-over-efficiency trade:
// leaking which child or conversation a thread concerns to an unauthorized
// staffer is worse than an extra refetch. The 500 ms debounce on the staff side
// collapses a sick-note rush into far fewer fetches.
func staffSafeParentMessage(event Event) Event {
	safe := event
	safe.Data.ThreadID = nil
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
// portals' messaging services (staff in Communication and parents in
// services/parent). The guardian account is stamped as Source so the guardian's
// own tabs can skip a redundant refetch; threadID/studentID (when positive) let
// an open chat refetch only the affected thread. Centralizing the payload shape
// here means adding a field can never drift between the two emitters.
func NewParentMessageEvent(guardianAccountID, threadID, studentID int64) Event {
	return NewEvent(EventParentMessage, "", parentMessageData(guardianAccountID, threadID, studentID))
}

// NewParentMessageReadEvent builds the EventParentMessageRead SSE event. It
// carries the SAME envelope as NewParentMessageEvent (guardian Source + the
// thread/student filter for an open chat), differing only in Type so the client
// routes it to a receipt-only refresh instead of a new-message refresh + badge
// bump. Same staffSafeParentMessage sanitization applies to the staff fan-out.
func NewParentMessageReadEvent(guardianAccountID, threadID, studentID int64) Event {
	return NewEvent(EventParentMessageRead, "", parentMessageData(guardianAccountID, threadID, studentID))
}

// NewParentChildUpdatedEvent builds the EventParentChildUpdated invalidation for
// one guardian: the guardian account as Source (so their own tabs can dedupe) and
// the affected student_id (so only that child's view refetches). No thread id — it
// accompanies no message. The caller fans it out once per guardian of the child.
func NewParentChildUpdatedEvent(guardianAccountID, studentID int64) Event {
	return NewEvent(EventParentChildUpdated, "", parentMessageData(guardianAccountID, 0, studentID))
}

// NewStaffMessageEvent builds the EventStaffMessage SSE event for the
// OGS-internal chat. It carries the thread id (see EventStaffMessage on why
// that is safe here) and the sender account as Source, so the sender's own tabs
// can skip the refetch their own send already triggered.
func NewStaffMessageEvent(senderAccountID, threadID int64) Event {
	sid := strconv.FormatInt(senderAccountID, 10)
	data := EventData{Source: &sid}
	if threadID > 0 {
		tid := strconv.FormatInt(threadID, 10)
		data.ThreadID = &tid
	}
	return NewEvent(EventStaffMessage, "", data)
}

// parentMessageData builds the shared payload for the parent-messaging events:
// the guardian account as Source plus the (optional) thread/student filter. One
// home so the new-message and read events can never drift in shape.
func parentMessageData(guardianAccountID, threadID, studentID int64) EventData {
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
	return data
}
