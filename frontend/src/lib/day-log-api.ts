// Tagesauswertung (#1456): client for GET /api/students/day-log.
// The proxy route forwards to the Go backend, which enforces the
// gdpr.attendance_log_enabled gate and group scope.

import { LOCATION_COLORS } from "~/lib/location-helper";

export type DayLogStatus =
  "present" | "sick" | "class_trip" | "excused" | "absent" | "not_scheduled";

interface DayLogCounters {
  present: number;
  sick: number;
  class_trip: number;
  excused: number;
  absent: number;
  not_scheduled: number;
  total: number;
}

export interface DayLogStudent {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  status: DayLogStatus;
  label: string;
  check_in_time?: string;
  check_out_time?: string;
  reported_at?: string;
  source?: string;
  hint?: string;
}

export interface DayLogGroup {
  group_id: string;
  name: string;
  students: DayLogStudent[];
  counters: DayLogCounters;
}

export interface DayLogResponse {
  date: string;
  groups: DayLogGroup[];
  counters: DayLogCounters;
}

export type DayLogErrorCode =
  | "feature_disabled"
  | "not_group_supervisor"
  | "no_permitted_groups"
  | "invalid_request"
  | "unknown";

export class DayLogError extends Error {
  readonly code: DayLogErrorCode;

  constructor(code: DayLogErrorCode, message?: string) {
    super(message ?? code);
    this.name = "DayLogError";
    this.code = code;
  }
}

const KNOWN_CODES: readonly DayLogErrorCode[] = [
  "feature_disabled",
  "not_group_supervisor",
  "no_permitted_groups",
  "invalid_request",
];

export async function fetchDayLog(
  dateISO: string,
  groupId?: string,
): Promise<DayLogResponse> {
  const params = new URLSearchParams({ date: dateISO });
  if (groupId) params.set("group_id", groupId);

  const response = await fetch(`/api/students/day-log?${params.toString()}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    let code: DayLogErrorCode = "unknown";
    try {
      const body = (await response.json()) as { error?: string };
      const match = KNOWN_CODES.find((known) => body.error === known);
      if (match) code = match;
    } catch {
      // non-JSON error body — keep "unknown"
    }
    throw new DayLogError(code, `day log request failed (${response.status})`);
  }

  const body = (await response.json()) as { data: DayLogResponse };
  return body.data;
}

export function dayLogExportUrl(
  dateISO: string,
  format: "pdf" | "xlsx",
  groupId?: string,
): string {
  const params = new URLSearchParams({ date: dateISO, format });
  if (groupId) params.set("group_id", groupId);
  return `/api/students/day-log/export?${params.toString()}`;
}

/** Display order of the status sections: explained absences in the middle,
 * unexplained "Abwesend" deliberately last (that's the follow-up list). */
export const DAY_LOG_STATUS_ORDER: readonly DayLogStatus[] = [
  "present",
  "sick",
  "class_trip",
  "excused",
  "not_scheduled",
  "absent",
];

export const DAY_LOG_STATUS_COLORS: Record<DayLogStatus, string> = {
  present: LOCATION_COLORS.GROUP_ROOM,
  sick: LOCATION_COLORS.SICK,
  class_trip: LOCATION_COLORS.CLASS_TRIP,
  excused: LOCATION_COLORS.EXCUSED,
  not_scheduled: LOCATION_COLORS.UNKNOWN,
  // DANGER, not HOME: HOME is grey now and not_scheduled above is already
  // grey, so an unexplained absence would lose its flag.
  absent: LOCATION_COLORS.DANGER,
};

/** Sources that came from the parents portal get a short origin hint. */
export function dayLogSourceLabel(source?: string): string {
  switch (source) {
    case "parent":
      return "Eltern-App";
    case "care_plan_cancelled":
      return "Abmeldung im Betreuungsplan";
    default:
      return "";
  }
}
