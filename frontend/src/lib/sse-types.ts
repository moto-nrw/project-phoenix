// SSE Event Types for Real-Time Updates

type SSEEventType =
  | "student_checkin"
  | "student_checkout"
  // Batched checkout emitted when a whole session ends (manual end or
  // scheduler timeout). One event per topic carries all affected student IDs
  // instead of one student_checkout per student — see backend issue #848.
  | "bulk_student_checkout"
  | "student_updated"
  | "activity_start"
  | "activity_end"
  | "activity_update"
  | "active_supervision_changed"
  | "dashboard_counts_changed"
  | "arrival_schedule_changed"
  | "tenant_settings_changed"
  // Timetable instance lifecycle events emitted by the backend (WP-B9).
  // The weekly planner subscribes to these so admin and office staff see
  // live status changes (start/complete/cancel) without manual refresh.
  | "instance_started"
  | "instance_completed"
  | "instance_cancelled"
  | "instance_overdue"
  // Parent-OGS messaging: a parent sent a message or staff replied. A trigger
  // for the staff inbox / child thread / parent thread to refetch and update
  // the unread badge. See backend/realtime/events.go EventParentMessage.
  | "parent_message";

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
