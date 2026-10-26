// Client-side API for the staff "Erinnerungen" page (issue #1457).
// Visual-only reminders — no sound. The backend computes the list from
// schedules already in the system; the frontend renders and badge-counts it.

import { authFetch } from "./api-helpers";

export type ReminderType =
  | "pickup_upcoming"
  | "pickup_overdue"
  | "activity_start"
  | "activity_overdue"
  | "activity_running_over";

export interface Reminder {
  type: ReminderType;
  student_id?: string;
  activity_instance_id?: string;
  title: string;
  subtitle?: string;
  due_time: string; // "HH:MM"
  minutes_away: number; // negative when overdue
  // activity_running_over only: children still in the block's room whose
  // pickup time has been reached (#4765).
  pickup_due_count?: number;
}

export interface RemindersResult {
  reminders: Reminder[];
  count: number;
  // True when the tenant has enabled at least one reminder type. Drives
  // whether the header bell (the only entry point to /reminders) renders at all.
  enabled: boolean;
  // Wall-clock "HH:MM" of the soonest same-day future moment this list changes
  // purely by time passing. A next-day boundary is an RFC 3339 instant, so the
  // hook can schedule it without losing the calendar date. Absent when nothing
  // time-based is pending. Data-driven changes still arrive via SSE.
  next_change_at?: string;
}

// Proxy route wrapper envelope: { success: true, data: <payload> }.
interface ProxyResponse {
  success: boolean;
  data: RemindersResult;
}

// Shared SWR key so the page and the header bell badge dedupe to one request.
export const REMINDERS_SWR_KEY = "reminders";

export async function fetchReminders(): Promise<RemindersResult> {
  const response = await authFetch<ProxyResponse>("/api/reminders");
  return response.data;
}
