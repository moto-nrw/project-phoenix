import { getCachedSession } from "./session-cache";

/**
 * One care day of a child. The row says the child is in care that weekday;
 * the time on it is optional and comes from the class timetable unless the
 * child deviates (#2414).
 */
export interface ArrivalSchedule {
  id: number;
  student_id: number;
  weekday: number;
  weekday_name: string;
  /** HH:MM, empty when neither the child nor its class carries a time. */
  expected_arrival: string;
  /** "class_schedule" = taken from the class, "staff" = per-child deviation. */
  source?: "class_schedule" | "staff";
  /** The class the time came from, set when source is "class_schedule". */
  source_class?: string;
  notes?: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalException {
  id: number;
  student_id: number;
  exception_date: string;
  expected_arrival?: string | null;
  reason?: string | null;
  // "staff" or "guardian" — a guardian-sourced row was set by a parent in the
  // parents portal.
  source?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalNote {
  id: number;
  student_id: number;
  note_date: string;
  content: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface ArrivalData {
  schedules: ArrivalSchedule[];
  exceptions: ArrivalException[];
  notes: ArrivalNote[];
}

export type CareDaysSource = "weekly_plan" | "bookings";

export interface ArrivalSettings {
  care_days_source: CareDaysSource;
}

export interface ArrivalScheduleInput {
  weekday: number;
  /** Empty string = care day whose time comes from the class timetable. */
  expected_arrival: string;
  notes?: string | null;
}

interface UpdateArrivalSchedulesBody {
  schedules: ArrivalScheduleInput[];
}

interface BulkArrivalRequestBody {
  school_class?: string;
  group_id?: number;
  student_ids?: number[];
  schedules: ArrivalScheduleInput[];
}

export type BulkArrivalFilter =
  | { type: "school_class"; schoolClass: string }
  | { type: "group"; groupId: string }
  | { type: "students"; studentIds: string[] };

interface ApiWrapper<T> {
  data: T;
}

export const WEEKDAYS: { value: number; label: string; short: string }[] = [
  { value: 1, label: "Montag", short: "Mo" },
  { value: 2, label: "Dienstag", short: "Di" },
  { value: 3, label: "Mittwoch", short: "Mi" },
  { value: 4, label: "Donnerstag", short: "Do" },
  { value: 5, label: "Freitag", short: "Fr" },
];

function unwrap<T>(value: ApiWrapper<T> | T): T {
  if (
    value &&
    typeof value === "object" &&
    "data" in (value as ApiWrapper<T>)
  ) {
    return (value as ApiWrapper<T>).data;
  }
  return value as T;
}

async function authHeaders(): Promise<HeadersInit> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const session = await getCachedSession();
  if (session?.user?.token) {
    headers.Authorization = `Bearer ${session.user.token}`;
  }
  return headers;
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const text = await response.text().catch(() => "");
    throw new Error(
      text
        ? `Request failed (${response.status}): ${text}`
        : `Request failed (${response.status})`,
    );
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const payload = (await response.json()) as ApiWrapper<T> | T;
  return unwrap<T>(payload);
}

export async function fetchArrivalData(
  studentId: string,
): Promise<ArrivalData> {
  const response = await fetch(`/api/students/${studentId}/arrival-schedules`, {
    method: "GET",
    headers: await authHeaders(),
    credentials: "include",
  });
  return parseResponse<ArrivalData>(response);
}

export async function fetchArrivalSettings(): Promise<ArrivalSettings> {
  const response = await fetch("/api/students/arrival-settings", {
    method: "GET",
    headers: await authHeaders(),
    credentials: "include",
  });
  return parseResponse<ArrivalSettings>(response);
}

export async function updateArrivalSchedules(
  studentId: string,
  schedules: ArrivalScheduleInput[],
): Promise<ArrivalSchedule[]> {
  const body: UpdateArrivalSchedulesBody = { schedules };
  const response = await fetch(`/api/students/${studentId}/arrival-schedules`, {
    method: "PUT",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify(body),
  });
  return parseResponse<ArrivalSchedule[]>(response);
}

export async function bulkUpsertArrivalSchedules(
  filter: BulkArrivalFilter,
  schedules: ArrivalScheduleInput[],
): Promise<unknown> {
  const body: BulkArrivalRequestBody = { schedules };
  if (filter.type === "school_class") {
    body.school_class = filter.schoolClass;
  } else if (filter.type === "group") {
    const groupId = Number.parseInt(filter.groupId, 10);
    if (!Number.isSafeInteger(groupId) || groupId <= 0) {
      throw new Error("Ungültige Gruppen-ID");
    }
    body.group_id = groupId;
  } else {
    const studentIds = [...new Set(filter.studentIds)].map((id) =>
      Number.parseInt(id, 10),
    );
    if (
      studentIds.length === 0 ||
      studentIds.length > 500 ||
      studentIds.some((id) => !Number.isSafeInteger(id) || id <= 0)
    ) {
      throw new Error("Ungültige Kinderauswahl");
    }
    body.student_ids = studentIds;
  }
  const response = await fetch("/api/students/arrival-schedules/bulk", {
    method: "POST",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify(body),
  });
  return parseResponse<unknown>(response);
}

interface ArrivalExceptionInput {
  exception_date: string;
  expected_arrival: string | null;
  clear_expected_arrival?: boolean;
  reason?: string | null;
}

export async function createArrivalException(
  studentId: string,
  input: ArrivalExceptionInput,
): Promise<ArrivalException> {
  const response = await fetch(
    `/api/students/${studentId}/arrival-exceptions`,
    {
      method: "POST",
      headers: await authHeaders(),
      credentials: "include",
      body: JSON.stringify(input),
    },
  );
  return parseResponse<ArrivalException>(response);
}

export async function updateArrivalException(
  studentId: string,
  exceptionId: number,
  input: ArrivalExceptionInput,
): Promise<ArrivalException> {
  const response = await fetch(
    `/api/students/${studentId}/arrival-exceptions/${exceptionId}`,
    {
      method: "PUT",
      headers: await authHeaders(),
      credentials: "include",
      body: JSON.stringify(input),
    },
  );
  return parseResponse<ArrivalException>(response);
}

export async function deleteArrivalException(
  studentId: string,
  exceptionId: number,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/arrival-exceptions/${exceptionId}`,
    {
      method: "DELETE",
      headers: await authHeaders(),
      credentials: "include",
    },
  );
  if (!response.ok && response.status !== 204) {
    const text = await response.text().catch(() => "");
    throw new Error(
      text
        ? `Request failed (${response.status}): ${text}`
        : `Request failed (${response.status})`,
    );
  }
}

interface ArrivalNoteInput {
  note_date: string;
  content: string;
}

export async function createArrivalNote(
  studentId: string,
  input: ArrivalNoteInput,
): Promise<ArrivalNote> {
  const response = await fetch(`/api/students/${studentId}/arrival-notes`, {
    method: "POST",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify(input),
  });
  return parseResponse<ArrivalNote>(response);
}

export async function updateArrivalNote(
  studentId: string,
  noteId: number,
  input: ArrivalNoteInput,
): Promise<ArrivalNote> {
  const response = await fetch(
    `/api/students/${studentId}/arrival-notes/${noteId}`,
    {
      method: "PUT",
      headers: await authHeaders(),
      credentials: "include",
      body: JSON.stringify(input),
    },
  );
  return parseResponse<ArrivalNote>(response);
}

export async function deleteArrivalNote(
  studentId: string,
  noteId: number,
): Promise<void> {
  const response = await fetch(
    `/api/students/${studentId}/arrival-notes/${noteId}`,
    {
      method: "DELETE",
      headers: await authHeaders(),
      credentials: "include",
    },
  );
  if (!response.ok && response.status !== 204) {
    const text = await response.text().catch(() => "");
    throw new Error(
      text
        ? `Request failed (${response.status}): ${text}`
        : `Request failed (${response.status})`,
    );
  }
}

interface BulkArrivalDayNoteResponse {
  id: number;
  content: string;
}

interface BulkArrivalDayNote {
  id: string;
  content: string;
}

interface BulkArrivalTimeResponse {
  student_id: number;
  date: string;
  weekday_name: string;
  expected_arrival?: string;
  is_exception: boolean;
  day_notes?: BulkArrivalDayNoteResponse[];
  notes?: string;
}

export interface BulkArrivalTime {
  studentId: string;
  date: string;
  weekdayName: string;
  expectedArrival?: string;
  isException: boolean;
  dayNotes: BulkArrivalDayNote[];
  notes?: string;
}

function mapBulkArrivalTimeResponse(
  data: BulkArrivalTimeResponse,
): BulkArrivalTime {
  return {
    studentId: data.student_id.toString(),
    date: data.date,
    weekdayName: data.weekday_name,
    expectedArrival: data.expected_arrival,
    isException: data.is_exception,
    dayNotes: (data.day_notes ?? []).map((n) => ({
      id: n.id.toString(),
      content: n.content,
    })),
    notes: data.notes,
  };
}

/**
 * Fetch effective arrival times for multiple students on a given date.
 * Uses bulk backend endpoint (O(3) queries instead of O(N)).
 */
export async function fetchBulkArrivalTimes(
  studentIds: string[],
  date?: string,
): Promise<Map<string, BulkArrivalTime>> {
  if (studentIds.length === 0) {
    return new Map();
  }

  const response = await fetch("/api/students/arrival-times/bulk", {
    method: "POST",
    headers: await authHeaders(),
    credentials: "include",
    body: JSON.stringify({
      student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
      date,
    }),
  });

  const data = await parseResponse<BulkArrivalTimeResponse[]>(response);

  const arrivalTimesMap = new Map<string, BulkArrivalTime>();
  for (const item of data) {
    const mapped = mapBulkArrivalTimeResponse(item);
    arrivalTimesMap.set(mapped.studentId, mapped);
  }

  return arrivalTimesMap;
}

/** The Unterrichtsschluss a school class carries (#2414). */
export interface ClassArrivalTimes {
  school_class: string;
  /** Day code ("mon" … "fri") to HH:MM. Absent days have no time. */
  times: Record<string, string>;
  updated_at?: string;
}

export async function fetchClassArrivalTimes(
  schoolClass: string,
): Promise<ClassArrivalTimes> {
  const response = await fetch(
    `/api/students/class-arrival-times/${encodeURIComponent(schoolClass)}`,
    { headers: await authHeaders() },
  );
  return parseResponse<ClassArrivalTimes>(response);
}
