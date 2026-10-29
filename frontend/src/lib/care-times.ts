import { apiErrorFromResponse, transportFetch } from "./api-error";

// Soll und Ist je Kind (#4774): client for the per-child comparison and the
// overrun list of the school management. The definitions live in the
// backend (studentpresence.CareTimeToleranceMinutes) and are repeated here
// only as screen text.

export type CareTimeDayKind =
  | "within_plan"
  | "overrun"
  | "unbooked"
  | "absent"
  | "sick"
  | "excused"
  | "cancelled"
  | "closed"
  | "open"
  | "no_plan";

interface CareTimeSummary {
  overrun_count: number;
  overrun_days: number;
  unbooked_days: number;
  absent_days: number;
  extra_minutes: number;
  planned_minutes: number;
  actual_minutes: number;
  deviation_minutes: number;
}

export interface CareTimeDay {
  date: string;
  kind: CareTimeDayKind;
  overrun: boolean;
  planned_start: string | null;
  planned_end: string | null;
  actual_start: string | null;
  actual_end: string | null;
  planned_minutes: number | null;
  actual_minutes: number | null;
  deviation_minutes: number | null;
  early_minutes: number;
  late_minutes: number;
}

export interface StudentCareTimes {
  student_id: string;
  from: string;
  to: string;
  tolerance_minutes: number;
  summary: CareTimeSummary;
  days: CareTimeDay[];
}

export interface CareTimeOverrunRow {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  group_name: string;
  summary: CareTimeSummary;
}

export interface CareTimeOverruns {
  from: string;
  to: string;
  tolerance_minutes: number;
  students: CareTimeOverrunRow[];
}

export type CareTimeExportFormat = "pdf" | "xlsx";

/** Tone of the day's status pill; matches the kit's StatusBadge tones. */
export type CareTimeTone = "blue" | "green" | "orange" | "red" | "gray";

function windowParams(
  fromISO: string,
  toISO: string,
  groupIds: readonly string[] = [],
): URLSearchParams {
  const params = new URLSearchParams({ from: fromISO, to: toISO });
  for (const id of groupIds) params.append("group_id", id);
  return params;
}

async function readData<T>(url: string, failure: string): Promise<T> {
  const response = await transportFetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `${failure} (${response.status})`,
    );
  }
  const body = (await response.json()) as { data: T };
  return body.data;
}

export function fetchStudentCareTimes(
  studentId: string,
  fromISO: string,
  toISO: string,
): Promise<StudentCareTimes> {
  return readData<StudentCareTimes>(
    `/api/statistics/students/${encodeURIComponent(studentId)}/care-times?${windowParams(fromISO, toISO).toString()}`,
    "student care times request failed",
  );
}

export function fetchCareTimeOverruns(
  fromISO: string,
  toISO: string,
  groupIds: readonly string[] = [],
): Promise<CareTimeOverruns> {
  return readData<CareTimeOverruns>(
    `/api/statistics/care-time-overruns?${windowParams(fromISO, toISO, groupIds).toString()}`,
    "care time overruns request failed",
  );
}

export function careTimeOverrunsExportUrl(
  fromISO: string,
  toISO: string,
  format: CareTimeExportFormat,
  groupIds: readonly string[] = [],
): string {
  const params = windowParams(fromISO, toISO, groupIds);
  params.set("format", format);
  return `/api/statistics/care-time-overruns/export?${params.toString()}`;
}

/** Text und Farbe je Tag; die Markierung trägt immer den Text, nie nur Farbe. */
export const CARE_TIME_DAY_LABELS: Record<
  CareTimeDayKind,
  { label: string; tone: CareTimeTone }
> = {
  within_plan: { label: "Wie gebucht", tone: "green" },
  overrun: { label: "Länger als gebucht", tone: "orange" },
  unbooked: { label: "Da ohne Buchung", tone: "orange" },
  absent: { label: "Nicht da", tone: "gray" },
  sick: { label: "Krank", tone: "red" },
  excused: { label: "Entschuldigt", tone: "gray" },
  cancelled: { label: "Abgemeldet", tone: "gray" },
  closed: { label: "Geschlossen", tone: "gray" },
  open: { label: "Noch nicht abgemeldet", tone: "blue" },
  no_plan: { label: "Keine Zeiten hinterlegt", tone: "gray" },
};

/** „08:00–15:00“, „ab 08:00“ oder „–“, wenn beide Zeiten fehlen. */
export function formatClockSpan(
  start: string | null,
  end: string | null,
): string {
  if (start && end) return `${start}–${end}`;
  if (start) return `ab ${start}`;
  if (end) return `bis ${end}`;
  return "–";
}

/** Minuten mit Vorzeichen: „+20 Min.“, „-420 Min.“, „0 Min.“ */
export function formatDeviation(minutes: number | null): string {
  if (minutes === null) return "–";
  if (minutes > 0) return `+${minutes} Min.`;
  return `${minutes} Min.`;
}

/** Warum ein Tag als Überschreitung zählt, in Worten. */
export function overrunReason(day: CareTimeDay): string | null {
  if (day.kind === "unbooked")
    return "An diesem Tag ist keine Betreuung gebucht.";
  if (day.kind !== "overrun") return null;
  const parts: string[] = [];
  if (day.early_minutes > 0)
    parts.push(`${day.early_minutes} Min. früher gekommen`);
  if (day.late_minutes > 0)
    parts.push(`${day.late_minutes} Min. später gegangen`);
  return parts.join(", ");
}
