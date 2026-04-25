// SSE Event Types for Real-Time Updates

export type SSEEventType =
  | "student_checkin"
  | "student_checkout"
  | "student_updated"
  | "activity_start"
  | "activity_end"
  | "activity_update"
  | "dashboard_counts_changed"
  | "arrival_schedule_changed";

// SSE Connection Status
export type ConnectionStatus = "connected" | "reconnecting" | "failed" | "idle";

export interface SSEEventData {
  // Student-related fields (for check-in/check-out events)
  student_id?: string;
  student_name?: string;
  school_class?: string;
  group_name?: string; // Student's OGS group

  // Activity session fields (for activity_start/end/update events)
  activity_name?: string;
  room_id?: string;
  room_name?: string;
  supervisor_ids?: string[];

  // Source tracking
  source?: "rfid" | "manual" | "automated";
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
