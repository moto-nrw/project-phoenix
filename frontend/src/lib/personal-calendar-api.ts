import {
  type ApiError,
  apiErrorFromText,
  unavailableApiError,
} from "~/lib/api-error";
import { toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "personal-calendar-api" });

interface ApiEnvelope<T> {
  readonly status?: string;
  readonly data?: T;
}

// "closing_day" und "holiday" liefert das Backend nicht als Termin: moto
// schule baut sie aus closing_days/holidays, damit Schließtage und Ferien im
// selben Raster stehen (#4798). "public_holiday" sind die gesetzlichen
// Feiertage aus /api/timetable/holidays in „Mein Kalender“ (#4775).
type CalendarSource =
  | "appointment"
  | "timetable"
  | "shift"
  | "closing_day"
  | "holiday"
  | "public_holiday";
export type CalendarDeliveryMode = "rsvp_required" | "informational";
export type CalendarOverviewVisibility = "organizer" | "staff" | "all";
export type CalendarResponseStatus =
  "pending" | "accepted" | "declined" | "info";

/** Art eines Schultermins, den eine Lehrkraft in moto schule anlegt (#4798). */
export type SchoolEventType =
  "ausflug" | "paedagogischer_tag" | "unterrichtsende" | "sonstiges";

export const SCHOOL_EVENT_TYPE_LABELS: Record<SchoolEventType, string> = {
  ausflug: "Ausflug",
  paedagogischer_tag: "Pädagogischer Tag",
  unterrichtsende: "Unterrichtsende",
  sonstiges: "Sonstiges",
};

/**
 * Art eines Schultermins im Kalender. „elternsprechtag“ trägt ein gebuchtes
 * Zeitfenster am Elternsprechtag (#4801); es entsteht nur durch die Buchung,
 * nie im Formular eines Schultermins.
 */
export type CalendarSchoolEventType = SchoolEventType | "elternsprechtag";

export const CALENDAR_SCHOOL_EVENT_TYPE_LABELS: Record<
  CalendarSchoolEventType,
  string
> = {
  ...SCHOOL_EVENT_TYPE_LABELS,
  elternsprechtag: "Elternsprechtag",
};

export interface CalendarEvent {
  readonly id: string;
  readonly source: CalendarSource;
  readonly appointment_id?: string;
  readonly occurrence_date?: string;
  readonly timetable_id?: string;
  /** "duty" für einen Dienst ohne Kinder aus dem Betreuungsplan (#3822). */
  readonly activity_type?: string;
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
  /** Gesetzt, wenn eine Lehrkraft den Termin als Schultermin angelegt hat. */
  readonly school_event_type?: CalendarSchoolEventType;
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
  | "all_teachers"
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

/**
 * A request that never got a readable answer: the connection broke, or a 2xx
 * body came back cut off or empty. The person has nothing to correct, only to
 * retry, so it becomes `general.unavailable` and the shared error path shows
 * the catalog text (#2517). Native browser texts (Safari's "The string did
 * not match the expected pattern.", "Load failed") go to the log instead —
 * client logs ship to /api/logs, where a stale or truncated response is
 * diagnosable.
 */
function requestFailed(path: string, stage: string, cause: unknown): ApiError {
  logger.error("calendar_request_failed", {
    path,
    stage,
    error: cause instanceof Error ? cause.message : String(cause),
  });
  return unavailableApiError(cause);
}

function isAbortError(error: unknown): boolean {
  return (
    error !== null &&
    typeof error === "object" &&
    "name" in error &&
    error.name === "AbortError"
  );
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
    // A superseded request keeps its AbortError semantics.
    if (isAbortError(cause)) throw cause;
    throw requestFailed(path, "network", cause);
  }
  if (!response.ok) {
    // The backend text stays a diagnostic. Code, field errors and request ID
    // travel on the ApiError to the shared display path.
    let text = "";
    try {
      text = await response.text();
    } catch {
      // Stream broke: the status still classifies the error.
    }
    throw apiErrorFromText(
      `Anfrage fehlgeschlagen (HTTP ${response.status})`,
      response.status,
      text,
    );
  }
  if (response.status === 204) return undefined as T;

  // Read the body as text and parse it here rather than calling
  // response.json(): a browser's parse failure carries an English native
  // message that must not reach the calendar.
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
  readonly caldav?: {
    readonly server_url: string;
    readonly username: string;
    readonly app_password: string;
  } | null;
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

// Kalender in moto schule (#4798): dieselben Termine über die school-Session
// und die Proxy-Routen unter /api/school/calendar, dazu Schließtage, Ferien
// und die Klassen der Lehrkraft.

export interface SchoolCalendarDays {
  readonly id: string;
  readonly name: string;
  readonly start_date: string;
  readonly end_date: string;
}

export interface SchoolCalendarResponse extends CalendarResponse {
  readonly closing_days: SchoolCalendarDays[];
  readonly holidays: SchoolCalendarDays[];
  readonly classes: string[];
}

/** Welche Eltern einen Schultermin sehen. OGS und Lehrkräfte sehen ihn immer. */
export type SchoolAppointmentParents = "none" | "classes" | "school";

export interface SchoolAppointmentRequest {
  readonly title: string;
  readonly description?: string;
  readonly location?: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly all_day: boolean;
  readonly school_event_type: SchoolEventType;
  /** Nur beim Anlegen gelesen; der Empfängerkreis bleibt danach gleich. */
  readonly parents: SchoolAppointmentParents;
  readonly classes?: string[];
}

export function getSchoolCalendar(from: Date, to: Date) {
  return fetchJSON<SchoolCalendarResponse>(
    `/api/school/calendar/my?${buildCalendarQuery(from, to)}`,
  );
}

export function createSchoolAppointment(body: SchoolAppointmentRequest) {
  return fetchJSON("/api/school/calendar/appointments", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function updateSchoolAppointment(
  appointmentId: string,
  body: SchoolAppointmentRequest,
) {
  return fetchJSON(
    `/api/school/calendar/appointments/${encodeURIComponent(appointmentId)}`,
    { method: "PUT", body: JSON.stringify(body) },
  );
}

export function deleteSchoolAppointment(appointmentId: string) {
  return fetchJSON(
    `/api/school/calendar/appointments/${encodeURIComponent(appointmentId)}`,
    { method: "DELETE" },
  );
}

export function respondSchoolCalendar(
  recipientId: string,
  status: "accepted" | "declined",
) {
  return fetchJSON(
    `/api/school/calendar/recipients/${encodeURIComponent(recipientId)}/response`,
    { method: "POST", body: JSON.stringify({ status }) },
  );
}

export function getSchoolCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/school/calendar/feed");
}

export function rotateSchoolCalendarFeed() {
  return fetchJSON<CalendarFeedInfo>("/api/school/calendar/feed/rotate", {
    method: "POST",
  });
}
