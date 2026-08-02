// SSE Event Types for Real-Time Updates

type SSEEventType =
  | "student_checkin"
  | "student_checkout"
  // Batched checkout emitted when a whole session ends (manual end or
  // scheduler timeout). One event per topic carries all affected student IDs
  // instead of one student_checkout per student — see backend issue #848.
  | "bulk_student_checkout"
  | "student_updated"
  // Tenant-wide signal that a child's Laufgemeinschaft ("läuft mit",
  // users.student_companions) may have changed: a submitted companion list, or
  // a departure-plan write, which trims the links on a weekday the new plan no
  // longer allows. Separate from student_updated because the links are
  // symmetric and the editing forms hold a draft — reacting to every student
  // write would discard or block an edit for a rename or a photo upload. See
  // backend/realtime/events.go EventStudentCompanionsChanged.
  | "student_companions_changed"
  | "activity_start"
  | "activity_end"
  | "activity_update"
  | "active_supervision_changed"
  | "dashboard_counts_changed"
  // Tenant-wide, identifier-free trigger emitted after staff work-session
  // writes commit. Time-account and school-overview caches refetch on it.
  | "staff_time_tracking_changed"
  | "arrival_schedule_changed"
  // Pickup-side counterpart of arrival_schedule_changed: a staff write changed
  // a child's weekly Gehzeit plan or a date-specific pickup exception. Its own
  // event so only the pickup-derived caches refetch (per-child Betreuungsplan,
  // student lists, student detail) — those disable focus revalidation, so
  // before it existed a staff pickup write stayed invisible in every other open
  // tab. See backend/realtime/events.go EventPickupScheduleChanged.
  | "pickup_schedule_changed"
  | "tenant_settings_changed"
  // Tenant-wide staff signal that a parent change-request queue changed (a
  // request was created/decided/withdrawn). Staff review lists + the pending-count
  // badge refetch on it, so an open review page updates live without depending on
  // the parent-messaging pill. Fires only on request transitions. See
  // backend/realtime/events.go EventChangeRequestsChanged.
  | "change_requests_changed"
  // Timetable instance lifecycle events emitted by the backend (WP-B9).
  // The weekly planner subscribes to these so admin and office staff see
  // live status changes (start/complete/cancel) without manual refresh.
  | "instance_started"
  | "instance_completed"
  | "instance_cancelled"
  | "instance_overdue"
  // Tenant-wide signal that Vertretungsplan staffing state changed on an
  // instance — absence set/cleared, substitute assigned, understaffed
  // acknowledgement toggled. These writes emit no instance_* lifecycle event
  // (planned blocks change no status; active blocks only get a group-scoped
  // activity_update), so timetable + own-assignment caches refetch on this
  // instead. See backend/realtime/events.go EventStaffingDeviationChanged.
  | "staffing_deviation_changed"
  // Tenant-wide signal that WHICH groups a staff member may open in "Meine
  // Gruppe" changed: a Vertretung created/edited/deleted, a Gruppenübergabe
  // handed over or taken back, a group's leaders reassigned, or a group
  // deleted. That set is the union of education.group_teacher and
  // education.group_substitution, so the event is named for the invariant
  // rather than for either table. The affected colleague's client makes no
  // request of its own and no other event covers this. Carries no staff
  // identity; clients refetch their own access-filtered views. See
  // backend/realtime/events.go EventGroupAccessChanged.
  | "group_access_changed"
  // Parent-OGS messaging: a parent sent a message or staff replied. A trigger
  // for the staff inbox / child thread / parent thread to refetch and update
  // the unread badge. See backend/realtime/events.go EventParentMessage.
  | "parent_message"
  // Parent-OGS messaging: the COUNTERPART read the conversation. A trigger for an
  // open chat to refresh its "Gelesen" receipts only — no new message, no unread
  // badge change. See backend/realtime/events.go EventParentMessageRead.
  | "parent_message_read"
  // Message-INDEPENDENT invalidation delivered to every guardian of a child so an
  // open parents-app tab refetches that child's care state (e.g. after an excused
  // request is created/decided/withdrawn), regardless of parent messaging being on.
  // Carries only student_id; must NOT touch any unread badge or thread list. See
  // backend/realtime/events.go EventParentChildUpdated.
  | "parent_child_updated"
  // A user-facing notification from the notification abstraction (#1624).
  // Unlike every other event this is NOT a cache-invalidation trigger: the
  // payload carries display-safe title/body/deep_link that clients render
  // directly (toast). See backend/realtime/events.go EventNotification.
  | "notification";

// SSE Connection Status
export type ConnectionStatus = "connected" | "reconnecting" | "failed" | "idle";

interface SSEEventData {
  // Student-related fields (for check-in/check-out events)
  student_id?: string;
  student_name?: string;
  school_class?: string;
  group_name?: string; // Student's OGS group

  // Affected students on a bulk_student_checkout event (whole-session end).
  student_ids?: string[];

  // Affected educational (OGS) group ids on dashboard_counts_changed /
  // student_checkin / student_checkout / bulk_student_checkout (#2057).
  // Group ids only — never student identity — so the tenant-wide
  // dashboard_counts_changed stays GDPR-safe. Clients scope their
  // ogs-students-{gid} revalidation to these ids; an ABSENT field means
  // "scope unknown → refresh broadly" (old backends, activity events,
  // students without an OGS group). The backend never sends an empty array.
  group_ids?: string[];

  // Activity session fields (for activity_start/end/update events)
  activity_name?: string;
  room_id?: string;
  room_name?: string;
  supervisor_ids?: string[];

  // Source tracking. Student/activity events typically use "rfid",
  // "manual", or "automated"; tenant-wide invalidation events also reuse
  // this field for setting keys / feature labels.
  source?: string;

  // Timetable instance fields (for instance_* events). instance_id is the
  // schedule.activity_instances row id; date and start_time identify the
  // affected slot in the weekly planner.
  instance_id?: string;
  instance_date?: string; // YYYY-MM-DD
  instance_start_time?: string; // HH:MM:SS

  // Reason tracking for generic refresh events.
  reason?: string;

  // Notification fields (notification events only, #1624). Display-safe by
  // backend contract — rendered directly, no refetch.
  title?: string;
  body?: string;
  deep_link?: string; // app-relative path, e.g. "/reminders"
  priority?: string; // "low" | "normal" | "high"
  notification_type?: string;
  notification_data?: Record<string, string>; // opaque IDs for client-side routing

  // Parent-messaging field (parent_message events). With student_id, lets an
  // open chat/conversation skip refetching when the event is about a different
  // thread/child.
  thread_id?: string;
}

export interface SSEEvent {
  type: SSEEventType;
  active_group_id: string;
  data: SSEEventData;
  timestamp: string; // ISO 8601 string
}

export interface SSEHookOptions {
  onMessage?: (event: SSEEvent) => void;
  onError?: (error: Event) => void;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  enabled?: boolean; // when false, do not establish EventSource
  reconnectKey?: string | number; // when this changes, force teardown+reconnect (e.g. tenantId)
}

export interface SSEHookState {
  isConnected: boolean;
  error: string | null;
  reconnectAttempts: number;
  status: ConnectionStatus;
}
