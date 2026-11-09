import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromResponse,
} from "~/lib/api-error";
import { bffJson } from "~/lib/bff-fetch";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { sessionFetch } from "~/lib/session-cache";

export type StudentStatusKind = "sick" | "excused" | "class_trip";

export interface StudentStatusDay {
  id: string;
  student_id: string;
  date: string;
  status: StudentStatusKind;
  label: string;
  reported_at: string;
  cleared_at?: string | null;
  source: string;
  /** Optional free-text reason, e.g. a parent-supplied sick note. */
  note?: string | null;
  created_at: string;
  updated_at: string;
}

/** The 409 body of a status-day write that would overwrite existing days. */
interface ConflictBody {
  conflicts?: BackendStudentStatusDay[];
  /** Full conflict count when `conflicts` is a capped sample. */
  conflict_count?: number;
  code?: string;
  instance?: string;
}

/** Matches backend `students.partial_absence_conflict` on status-day writes. */
const PARTIAL_ABSENCE_CONFLICT_CODE: ErrorCode =
  "students.partial_absence_conflict";

interface BackendStudentStatusDay {
  id: number;
  student_id: number;
  date: string;
  status: StudentStatusKind;
  label: string;
  reported_at: string;
  cleared_at?: string | null;
  source: string;
  note?: string | null;
  created_at: string;
  updated_at: string;
}

function mapStatusDay(row: BackendStudentStatusDay): StudentStatusDay {
  return {
    ...row,
    id: row.id.toString(),
    student_id: row.student_id.toString(),
  };
}

export class StudentStatusDayConflictError extends ApiError {
  readonly conflicts: StudentStatusDay[];
  /** Full conflict count; may exceed `conflicts.length` when the API capped samples. */
  readonly totalCount: number;

  constructor(
    conflicts: StudentStatusDay[],
    totalCount?: number,
    payload?: { code?: string; instance?: string },
  ) {
    super("Vorhandene Status-Tage wurden nicht überschrieben", 409, payload);
    this.name = "StudentStatusDayConflictError";
    this.conflicts = conflicts;
    this.totalCount = totalCount ?? conflicts.length;
  }
}

function parseConflictError(result: ConflictBody): Error {
  if (result.code === PARTIAL_ABSENCE_CONFLICT_CODE) {
    return apiErrorFromBody("partial absence conflict", 409, result);
  }
  const conflicts = (result.conflicts ?? []).map(mapStatusDay);
  const totalCount =
    typeof result.conflict_count === "number" &&
    Number.isFinite(result.conflict_count)
      ? result.conflict_count
      : conflicts.length;
  return new StudentStatusDayConflictError(conflicts, totalCount, {
    code: result.code,
    instance: result.instance,
  });
}

/**
 * A status-day write. Its 409 is a question with the conflicting days in the
 * body (or the partial-absence refusal), not a plain failure.
 */
async function writeStatusDays<T>(
  url: string,
  body: unknown,
  fallback: string,
): Promise<T> {
  const response = await sessionFetch(url, {
    method: "POST",
    body: JSON.stringify(body),
  });
  if (response.status === 409) {
    throw parseConflictError((await response.json()) as ConflictBody);
  }
  if (!response.ok) throw await apiErrorFromResponse(response, fallback);
  return ((await response.json()) as { data: T }).data;
}

export async function fetchStudentStatusDays(
  studentId: string,
  from: string,
  to: string,
): Promise<StudentStatusDay[]> {
  const rows = await bffJson<BackendStudentStatusDay[]>(
    `/api/students/${encodeURIComponent(studentId)}/status-days?from=${from}&to=${to}`,
  );
  return rows.map(mapStatusDay);
}

/**
 * One row of the tenant-wide absence overview (#2288). IDs arrive as strings
 * from the backend already; no mapping needed.
 */
export interface StatusDayOverviewEntry {
  id: string;
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  group_id: string;
  group_name: string;
  date: string;
  status: StudentStatusKind;
  label: string;
  reported_at: string;
  source: string;
}

export interface StatusDayOverview {
  from: string;
  to: string;
  groups: Array<{ id: string; name: string }>;
  entries: StatusDayOverviewEntry[];
  page: number;
  page_size: number;
  has_more: boolean;
}

export async function fetchStatusDayOverview(
  from: string,
  to: string,
  filters: { page: number; query: string; status: string; groupId: string },
): Promise<StatusDayOverview> {
  const params = new URLSearchParams({
    from,
    to,
    page: filters.page.toString(),
    page_size: "50",
  });
  if (filters.query.trim()) params.set("q", filters.query.trim());
  if (filters.status !== "all") params.set("status", filters.status);
  if (filters.groupId !== "all") params.set("group_id", filters.groupId);
  return bffJson<StatusDayOverview>(`/api/students/status-days?${params}`);
}

export async function createStudentStatusDays(
  studentId: string,
  status: StudentStatusKind,
  dates: string[],
  reason?: string,
  /** Days whose partial excusal the person chose to replace (#5698). */
  replacePartialAbsenceDates?: readonly string[],
): Promise<StudentStatusDay[]> {
  const rows = await writeStatusDays<BackendStudentStatusDay[]>(
    `/api/students/${encodeURIComponent(studentId)}/status-days`,
    {
      status,
      dates,
      ...(reason ? { reason } : {}),
      ...(replacePartialAbsenceDates?.length
        ? { replace_partial_absence_dates: replacePartialAbsenceDates }
        : {}),
    },
    "Geplante Einträge konnten nicht gespeichert werden",
  );
  return rows.map(mapStatusDay);
}

export async function bulkCreateStudentStatusDays(
  studentIds: string[],
  status: StudentStatusKind,
  from: string,
  to: string,
  reason?: string,
): Promise<{ student_count: number; date_count: number }> {
  const student_ids = studentIds.map(Number);
  return writeStatusDays<{ student_count: number; date_count: number }>(
    "/api/students/status-days/bulk",
    reason
      ? { student_ids, status, from, to, reason }
      : { student_ids, status, from, to },
    "Klassenfahrt konnte nicht gespeichert werden",
  );
}

export async function deleteStudentStatusDay(
  studentId: string,
  statusDayId: string,
): Promise<void> {
  await bffJson<unknown>(
    `/api/students/${encodeURIComponent(studentId)}/status-days/${encodeURIComponent(statusDayId)}`,
    { method: "DELETE" },
  );
}
