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
  room_data_days: number;
  room_data_from: string;
}

export type StatisticsErrorCode = "forbidden" | "invalid_request" | "unknown";

export class StatisticsError extends Error {
  readonly code: StatisticsErrorCode;

  constructor(code: StatisticsErrorCode, message?: string) {
    super(message ?? code);
    this.name = "StatisticsError";
    this.code = code;
  }
}

export type StatisticsExportFormat = "pdf" | "xlsx" | "docx";

function buildParams(
  fromISO: string,
  toISO: string,
  groupIds: readonly string[],
): URLSearchParams {
  const params = new URLSearchParams({ from: fromISO, to: toISO });
  for (const id of groupIds) params.append("group_id", id);
  return params;
}

export async function fetchStatisticsReport(
  fromISO: string,
  toISO: string,
  groupIds: readonly string[] = [],
): Promise<StatisticsReport> {
  const params = buildParams(fromISO, toISO, groupIds);
  const response = await fetch(`/api/statistics/report?${params.toString()}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    let code: StatisticsErrorCode = "unknown";
    if (response.status === 403) code = "forbidden";
    if (response.status === 400) code = "invalid_request";
    throw new StatisticsError(
      code,
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
  groupIds: readonly string[] = [],
): string {
  const params = buildParams(fromISO, toISO, groupIds);
  params.set("format", format);
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
