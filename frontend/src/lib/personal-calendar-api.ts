import { toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "personal-calendar-api" });

interface ApiEnvelope<T> {
  readonly status?: string;
  readonly data?: T;
}

type CalendarSource = "appointment" | "timetable" | "shift";
export type CalendarDeliveryMode = "rsvp_required" | "informational";
export type CalendarOverviewVisibility = "organizer" | "staff" | "all";
export type CalendarResponseStatus =
  "pending" | "accepted" | "declined" | "info";

export interface CalendarEvent {
  readonly id: string;
  readonly source: CalendarSource;
  readonly appointment_id?: string;
  readonly occurrence_date?: string;
  readonly timetable_id?: string;
  readonly student_id?: string;
  readonly student_name?: string;
  readonly tenant_id?: string;
  readonly school_name?: string;
  readonly title: string;
  readonly description?: string;
  readonly location?: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly all_day: boolean;
  readonly cancelled?: boolean;
  readonly recurring?: boolean;
  readonly delivery_mode?: CalendarDeliveryMode;
  readonly response_status?: CalendarResponseStatus;
  readonly recipient_id?: string;
  readonly organizer_staff_id?: string;
  readonly can_respond: boolean;
  readonly can_edit: boolean;
  readonly can_view_overview?: boolean;
}

export interface CalendarResponse {
  readonly from: string;
  readonly to: string;
  readonly events: CalendarEvent[];
}

export type CalendarTargetType =
  | "staff"
  | "guardian_profile"
  | "all_staff"
  | "all_school_parents"
  | "parents_by_class"
  | "parents_by_group"
  | "parents_by_student";

export interface CalendarTarget {
  readonly type: CalendarTargetType;
  readonly id?: string;
  readonly value?: string;
}

interface CalendarRecurrenceRequest {
  readonly frequency: "daily" | "weekly" | "monthly" | "yearly";
  readonly interval_count: number;
  readonly weekdays?: string[];
  readonly month_days?: number[];
  readonly ends_on?: string;
  readonly occurrence_count?: number;
}

export interface CreateCalendarAppointmentRequest {
  readonly title: string;
  readonly description?: string;
  readonly location?: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly all_day: boolean;
  readonly delivery_mode: CalendarDeliveryMode;
  readonly overview_visibility?: CalendarOverviewVisibility;
  readonly recurrence?: CalendarRecurrenceRequest;
  readonly targets: CalendarTarget[];
  readonly send_email?: boolean;
}

// Editing an appointment cannot change its audience (targets) or delivery mode:
// re-resolving recipients would discard the RSVP responses already collected.
export interface UpdateCalendarAppointmentRequest {
  readonly title: string;
  readonly description?: string;
  readonly location?: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly all_day: boolean;
  readonly overview_visibility?: CalendarOverviewVisibility;
  readonly recurrence?: CalendarRecurrenceRequest;
  readonly send_email?: boolean;
}

interface CalendarAttendee {
  readonly recipient_id: string;
  readonly recipient_type: "staff" | "guardian_profile";
  readonly name: string;
  readonly status: CalendarResponseStatus;
  readonly responded_at?: string;
}

export interface CalendarAppointmentOverview {
  readonly appointment_id: string;
  readonly delivery_mode: CalendarDeliveryMode;
  readonly overview_visibility: CalendarOverviewVisibility;
  readonly attendees: CalendarAttendee[];
}

export interface CalendarRecipientOptions {
  readonly staff: Array<{ readonly id: string; readonly name: string }>;
  readonly parents: Array<{ readonly id: string; readonly name: string }>;
  readonly groups: Array<{ readonly id: string; readonly name: string }>;
  readonly classes: string[];
  readonly students: Array<{
    readonly id: string;
    readonly name: string;
    readonly school_class?: string;
    readonly group_id?: string;
  }>;
}

function unwrap<T>(json: ApiEnvelope<T>): T {
  if (json && typeof json === "object" && "data" in json) {
    return json.data as T;
  }
  return json as T;
}

// Shown when the request never arrived or the answer came back unreadable —
// cases where the school user has nothing to correct, only to retry.
const REQUEST_FAILED_MESSAGE =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

/**
 * One German sentence for a failure that carries no message from the backend.
 *
 * The calendar page renders `error.message` verbatim in its red box, so a
 * native browser error used to land in front of school staff: Safari reports a
 * truncated or non-JSON 2xx body as "The string did not match the expected
 * pattern." (its default SyntaxError text), and a cut connection as "Load
 * failed". Both are English, technical, and name nothing the reader can do.
 * The technical detail goes to the log instead — client logs ship to
 * /api/logs, which is where a stale or truncated response is diagnosable.
 */
function requestFailed(path: string, stage: string, cause: unknown): Error {
  logger.error("calendar_request_failed", {
    path,
    stage,
    error: cause instanceof Error ? cause.message : String(cause),
  });
  return new Error(REQUEST_FAILED_MESSAGE, { cause });
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
      credentials: "include",
    });
  } catch (cause) {
    throw requestFailed(path, "network", cause);
  }
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Keep generic message.
    }
    throw new Error(message);
  }
  if (response.status === 204) return undefined as T;

  // Read the body as text and parse it here rather than calling
  // response.json(): a browser's parse failure carries an English native
  // message, and this is the one place that can turn it into readable German
  // before it reaches the calendar's error box.
  let body: string;
  try {
    body = await response.text();
  } catch (cause) {
    throw requestFailed(path, "body", cause);
  }
  if (body.trim() === "") throw requestFailed(path, "empty", "empty body");
  try {
    return unwrap<T>(JSON.parse(body) as ApiEnvelope<T>);
  } catch (cause) {
    throw requestFailed(path, "parse", cause);
  }
}

function buildCalendarQuery(from: Date, to: Date): string {
  return new URLSearchParams({
    from: toISODate(from),
    to: toISODate(to),
  }).toString();
}

export function getStaffCalendar(from: Date, to: Date) {
  return fetchJSON<CalendarResponse>(
    `/api/calendar/my?${buildCalendarQuery(from, to)}`,
  );
}

export function getParentCalendar(from: Date, to: Date) {
  return fetchJSON<CalendarResponse>(
    `/api/parent/calendar?${buildCalendarQuery(from, to)}`,
  );
}

export function respondStaffCalendar(
  recipientId: string,
  status: "accepted" | "declined",
) {
  return fetchJSON(
    `/api/calendar/recipients/${encodeURIComponent(recipientId)}/response`,
    {
      method: "POST",
      body: JSON.stringify({ status }),
    },
  );
}

export function respondParentCalendar(
  recipientId: string,
  status: "accepted" | "declined",
) {
  return fetchJSON(
    `/api/parent/calendar/recipients/${encodeURIComponent(recipientId)}/response`,
    {
      method: "POST",
      body: JSON.stringify({ status }),
    },
  );
}

export function getStaffAppointmentOverview(appointmentId: string) {
  return fetchJSON<CalendarAppointmentOverview>(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`,
  );
}

export interface CalendarFeedInfo {
  readonly url: string;
  readonly webcal_url: string;
}

export function getParentCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/parent/calendar/feed");
}

export function rotateParentCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/parent/calendar/feed/rotate", {
    method: "POST",
  });
}

export function getStaffCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/calendar/feed");
}

export function rotateStaffCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/calendar/feed/rotate", {
    method: "POST",
  });
}

export function getParentAppointmentOverview(appointmentId: string) {
  return fetchJSON<CalendarAppointmentOverview>(
    `/api/parent/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`,
  );
}

export function createStaffAppointment(body: CreateCalendarAppointmentRequest) {
  return fetchJSON("/api/calendar/appointments", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

// Minimal shape of the staff appointment-detail response, used to prefill the
// edit modal with recurrence + visibility that the per-occurrence event omits.
export interface CalendarAppointmentDetail {
  readonly appointment: {
    readonly title: string;
    readonly description?: string;
    readonly location?: string;
    // Persisted SERIES base dates ("YYYY-MM-DD") — for a recurring appointment
    // these are the anchor, not the clicked occurrence, so an edit doesn't
    // re-anchor the series.
    readonly start_date: string;
    readonly end_date: string;
    readonly all_day: boolean;
    readonly overview_visibility: CalendarOverviewVisibility;
    readonly delivery_mode: CalendarDeliveryMode;
    readonly notify_guardians: boolean;
  };
  readonly recurrence?: {
    readonly frequency: "daily" | "weekly" | "monthly" | "yearly";
    readonly interval_count: number;
    readonly weekdays?: string[];
    readonly month_days?: number[];
    readonly ends_on?: string;
    readonly occurrence_count?: number;
  } | null;
}

export function getStaffAppointmentDetail(appointmentId: string) {
  return fetchJSON<CalendarAppointmentDetail>(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
  );
}

export function updateStaffAppointment(
  appointmentId: string,
  body: UpdateCalendarAppointmentRequest,
) {
  return fetchJSON(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
    {
      method: "PUT",
      body: JSON.stringify(body),
    },
  );
}

export function cancelStaffAppointment(appointmentId: string) {
  return fetchJSON(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/cancel`,
    { method: "POST" },
  );
}

export function deleteStaffAppointment(appointmentId: string) {
  return fetchJSON(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}`,
    { method: "DELETE" },
  );
}

export function cancelStaffAppointmentOccurrence(
  appointmentId: string,
  occurrenceDate: string,
) {
  return fetchJSON(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/occurrences/${encodeURIComponent(occurrenceDate)}/cancel`,
    { method: "POST" },
  );
}

export function getCalendarRecipientOptions(query: string) {
  const params = new URLSearchParams();
  if (query.trim()) params.set("q", query.trim());
  params.set("limit", "30");
  return fetchJSON<CalendarRecipientOptions>(
    `/api/calendar/recipient-options?${params}`,
  );
}
