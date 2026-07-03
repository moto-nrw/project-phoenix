// Client-side API for the staff "Erinnerungen" page (issue #1457).
// Visual-only reminders — no sound. The backend computes the list from
// schedules already in the system; the frontend renders and badge-counts it.

import { authFetch } from "./api-helpers";

export type ReminderType =
  | "pickup_upcoming"
  | "pickup_overdue"
  | "activity_start"
  | "activity_overdue";

export interface Reminder {
  type: ReminderType;
  student_id?: string;
  title: string;
  subtitle?: string;
  due_time: string; // "HH:MM"
  minutes_away: number; // negative when overdue
}

export interface RemindersResult {
  reminders: Reminder[];
  count: number;
  // True when the tenant has enabled at least one reminder type. Drives
  // whether the header bell (the only entry point to /reminders) renders at all.
  enabled: boolean;
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
