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

interface ApiResponse<T> {
  status: string;
  data?: T;
  conflicts?: T;
  /** Full conflict count when `conflicts` is a capped sample. */
  conflict_count?: number;
  message?: string;
  error?: string;
  /** Stable backend conflict code (e.g. partial_absence_conflict). */
  code?: string;
}

/** Matches backend `partial_absence_conflict` on status-day writes. */
const PARTIAL_ABSENCE_CONFLICT_CODE = "partial_absence_conflict";

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

export class StudentStatusDayConflictError extends Error {
  readonly conflicts: StudentStatusDay[];
  /** Full conflict count; may exceed `conflicts.length` when the API capped samples. */
  readonly totalCount: number;

  constructor(conflicts: StudentStatusDay[], totalCount?: number) {
    super("Vorhandene Status-Tage wurden nicht überschrieben");
    this.name = "StudentStatusDayConflictError";
    this.conflicts = conflicts;
    this.totalCount = totalCount ?? conflicts.length;
  }
}

/** Full-day status write refused because a partial-day excusal already exists. */
export class StudentStatusDayPartialAbsenceConflictError extends Error {
  constructor() {
    super(
      "Für diesen Tag liegt bereits eine Abmeldung ab einer Uhrzeit vor. Bitte zuerst die Teilabwesenheit entfernen.",
    );
    this.name = "StudentStatusDayPartialAbsenceConflictError";
  }
}

function parseConflictError(
  result: ApiResponse<BackendStudentStatusDay[]>,
): Error {
  if (result.code === PARTIAL_ABSENCE_CONFLICT_CODE) {
    return new StudentStatusDayPartialAbsenceConflictError();
  }
  const conflicts = (result.conflicts ?? result.data ?? []).map(mapStatusDay);
  const totalCount =
    typeof result.conflict_count === "number" &&
    Number.isFinite(result.conflict_count)
      ? result.conflict_count
      : conflicts.length;
  return new StudentStatusDayConflictError(conflicts, totalCount);
}

async function parseApiResult<T>(
  response: Response,
  fallback: string,
): Promise<T> {
  const result = (await response.json()) as ApiResponse<T>;
  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? fallback);
  }
  return result.data;
}

export async function fetchStudentStatusDays(
  studentId: string,
  from: string,
  to: string,
): Promise<StudentStatusDay[]> {
  const response = await fetch(
    `/api/students/${studentId}/status-days?from=${from}&to=${to}`,
  );
  if (!response.ok) {
    throw new Error("Geplante Einträge konnten nicht geladen werden");
  }
  const data = await parseApiResult<BackendStudentStatusDay[]>(
    response,
    "Geplante Einträge konnten nicht geladen werden",
  );
  return data.map(mapStatusDay);
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
}

/** Thrown when the account has no staff link (backend 403). */
export class StatusDayOverviewForbiddenError extends Error {
  constructor() {
    super(
      "Ihr Konto ist keinem Personaleintrag zugeordnet. Bitte wenden Sie sich an Ihre Administration.",
    );
    this.name = "StatusDayOverviewForbiddenError";
  }
}

export async function fetchStatusDayOverview(
  from: string,
  to: string,
): Promise<StatusDayOverview> {
  const response = await fetch(
    `/api/students/status-days?from=${from}&to=${to}`,
  );
  if (response.status === 403) {
    throw new StatusDayOverviewForbiddenError();
  }
  if (!response.ok) {
    throw new Error("Abwesenheiten konnten nicht geladen werden");
  }
  return parseApiResult<StatusDayOverview>(
    response,
    "Abwesenheiten konnten nicht geladen werden",
  );
}

export async function createStudentStatusDays(
  studentId: string,
  status: StudentStatusKind,
  dates: string[],
  reason?: string,
): Promise<StudentStatusDay[]> {
  const response = await fetch(`/api/students/${studentId}/status-days`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    // Only include the reason when one is supplied so the default request
    // shape is unchanged (the backend treats an absent reason as no note).
    body: JSON.stringify(
      reason ? { status, dates, reason } : { status, dates },
    ),
  });
  if (response.status === 409) {
    const result = (await response.json()) as ApiResponse<
      BackendStudentStatusDay[]
    >;
    throw parseConflictError(result);
  }
  if (!response.ok) {
    throw new Error("Geplante Einträge konnten nicht gespeichert werden");
  }
  const data = await parseApiResult<BackendStudentStatusDay[]>(
    response,
    "Geplante Einträge konnten nicht gespeichert werden",
  );
  return data.map(mapStatusDay);
}

export async function bulkCreateStudentStatusDays(
  studentIds: string[],
  status: StudentStatusKind,
  from: string,
  to: string,
  reason?: string,
): Promise<{ student_count: number; date_count: number }> {
  const response = await fetch("/api/students/status-days/bulk", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(
      reason
        ? { student_ids: studentIds.map(Number), status, from, to, reason }
        : { student_ids: studentIds.map(Number), status, from, to },
    ),
  });
  if (response.status === 409) {
    const result = (await response.json()) as ApiResponse<
      BackendStudentStatusDay[]
    >;
    throw parseConflictError(result);
  }
  if (!response.ok) {
    throw new Error("Klassenfahrt konnte nicht gespeichert werden");
  }
  return parseApiResult<{ student_count: number; date_count: number }>(
    response,
    "Klassenfahrt konnte nicht gespeichert werden",
  );
}

export async function deleteStudentStatusDay(
  studentId: string,
  statusDayId: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/status-days/${statusDayId}`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    throw new Error("Geplanter Eintrag konnte nicht gelöscht werden");
  }
}
