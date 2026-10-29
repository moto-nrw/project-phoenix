import { apiErrorFromResponse, transportFetch } from "./api-error";
// Statistik (#2606): client for GET /api/statistics/report and the export
// URL. The proxy routes forward to the Go backend, which enforces
// config:read + users:read, validates the window and writes the audit row.

export interface StatisticsStudentRow {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  group_id?: string;
  group_name: string;
  present_days: number;
  sick_days: number;
  excused_days: number;
  unexplained_days: number;
  attendance_rate: number | null;
}

export interface StatisticsGroupRow {
  group_id: string;
  name: string;
  student_count: number;
  present_days: number;
  sick_days: number;
  excused_days: number;
  unexplained_days: number;
  attendance_rate: number | null;
}

export interface StatisticsRoomRow {
  room_id: string;
  name: string;
  capacity?: number;
  days_used: number;
  distinct_students: number;
  student_minutes: number;
  peak_occupancy: number;
  peak_utilization_percent: number | null;
}

/** Kursteilnahme je Kurs (#2891). */
export interface StatisticsCourseRow {
  course_id: string;
  name: string;
  category_name: string;
  /** 0 = keine Teilnehmergrenze. */
  max_participants: number;
  held_instances: number;
  cancelled_instances: number;
  student_count: number;
  present_days: number;
  absent_days: number;
  open_days: number;
  participation_rate: number | null;
  occupancy_percent: number | null;
}

/** Eine Zeile je Kind und Kurs. */
export interface StatisticsCourseStudentRow {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  group_name: string;
  course_id: string;
  course_name: string;
  present_days: number;
  absent_days: number;
  open_days: number;
  participation_rate: number | null;
}

/** Eine halbe Stunde der Belegung nach Uhrzeit (#4764). */
export interface StatisticsOccupancySlot {
  /** Beginn als "HH:MM" (Berliner Zeit). */
  start: string;
  planned_average: number;
  planned_max: number;
  /** null, wenn für diesen Wochentag keine Anwesenheiten mehr gespeichert sind. */
  actual_average: number | null;
  actual_max: number | null;
}

export interface StatisticsOccupancyWeekday {
  /** ISO: 1 = Montag … 7 = Sonntag. */
  weekday: number;
  plan_days: number;
  actual_days: number;
  slots: StatisticsOccupancySlot[];
}

export interface StatisticsOccupancy {
  slot_minutes: number;
  weekdays: StatisticsOccupancyWeekday[];
  untimed_plan_days: number;
  actual_data_days: number;
  actual_data_from: string;
}

export interface StatisticsReport {
  from: string;
  to: string;
  care_days: number;
  excluded_days: {
    total: number;
    public_holidays: number;
    closing_days: number;
    holiday_periods: number;
  };
  totals: StatisticsGroupRow;
  students: StatisticsStudentRow[];
  groups: StatisticsGroupRow[];
  rooms: StatisticsRoomRow[];
  room_history_disabled: boolean;
  room_data_days: number;
  room_data_from: string;
  courses: StatisticsCourseRow[];
  course_students: StatisticsCourseStudentRow[];
  course_totals: StatisticsCourseRow;
  course_data_days: number;
  course_data_from: string;
  /** Nur vorhanden, wenn der Abschnitt "occupancy" angefragt wurde. */
  occupancy?: StatisticsOccupancy;
}

/** Abschnitte, die der Bericht berechnen soll; leer = alle. */
export type StatisticsReportSection =
  "attendance" | "rooms" | "courses" | "occupancy";

export interface StatisticsFilter {
  groupIds?: readonly string[];
  schoolClasses?: readonly string[];
}

export type StatisticsExportFormat = "pdf" | "xlsx" | "docx";
/**
 * attendance = children and groups (default), rooms = room utilization,
 * courses / course-students = the two views of the course participation
 * section. Each is its own document with its own columns.
 */
export type StatisticsExportSection =
  "attendance" | "rooms" | "courses" | "course-students" | "occupancy";

/** Dateiname-Stamm je Bereich, falls der Server keinen Namen mitschickt. */
export const EXPORT_FILENAME_STEM: Record<StatisticsExportSection, string> = {
  attendance: "statistik",
  rooms: "raumauslastung",
  courses: "kurse",
  "course-students": "kurse-je-kind",
  occupancy: "belegung-nach-uhrzeit",
};

function buildParams(
  fromISO: string,
  toISO: string,
  filter: StatisticsFilter,
): URLSearchParams {
  const params = new URLSearchParams({ from: fromISO, to: toISO });
  for (const id of filter.groupIds ?? []) params.append("group_id", id);
  for (const name of filter.schoolClasses ?? [])
    params.append("school_class", name);
  return params;
}

export async function fetchStatisticsReport(
  fromISO: string,
  toISO: string,
  filter: StatisticsFilter = {},
  sections: readonly StatisticsReportSection[] = [],
): Promise<StatisticsReport> {
  const params = buildParams(fromISO, toISO, filter);
  for (const section of sections) params.append("section", section);
  const response = await transportFetch(
    `/api/statistics/report?${params.toString()}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      `statistics request failed (${response.status})`,
    );
  }
  const body = (await response.json()) as { data: StatisticsReport };
  return body.data;
}

export function statisticsExportUrl(
  fromISO: string,
  toISO: string,
  format: StatisticsExportFormat,
  filter: StatisticsFilter = {},
  section: StatisticsExportSection = "attendance",
): string {
  const params = buildParams(fromISO, toISO, filter);
  params.set("format", format);
  params.set("section", section);
  return `/api/statistics/export?${params.toString()}`;
}

/** Renders a percent value the way the tables show it ("87,5 %"). */
export function formatRate(value: number | null | undefined): string {
  if (value === null || value === undefined) return "";
  return `${value.toLocaleString("de-DE", { minimumFractionDigits: 1, maximumFractionDigits: 1 })} %`;
}

/** Minutes → hours with one decimal ("12,5"). */
export function formatHours(minutes: number): string {
  return (minutes / 60).toLocaleString("de-DE", {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  });
}
