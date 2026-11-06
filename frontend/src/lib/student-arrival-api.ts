import { ApiError } from "./api-error";
import { bffJson } from "./bff-fetch";
import { DEPARTURE_WEEKDAYS } from "./departure-plan";

/**
 * One care day of a child. The row says the child is in care that weekday;
 * the time on it is optional and comes from the class timetable unless the
 * child deviates (#2414).
 */
export interface ArrivalSchedule {
  id: string;
  studentId: string;
  weekday: number;
  weekdayName: string;
  /** HH:MM, empty when neither the child nor its class carries a time. */
  expectedArrival: string;
  /** "class_schedule" = taken from the class, "staff" = per-child deviation. */
  source?: "class_schedule" | "staff";
  /** The class the time came from, set when source is "class_schedule". */
  sourceClass?: string;
  notes?: string | null;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface ArrivalException {
  id: string;
  studentId: string;
  exceptionDate: string;
  expectedArrival?: string | null;
  reason?: string | null;
  // "staff" or "guardian" — a guardian-sourced row was set by a parent in the
  // parents portal.
  source?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface ArrivalNote {
  id: string;
  studentId: string;
  noteDate: string;
  content: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface ArrivalData {
  schedules: ArrivalSchedule[];
  exceptions: ArrivalException[];
  notes: ArrivalNote[];
}

interface BackendArrivalSchedule {
  id: number;
  student_id: number;
  weekday: number;
  weekday_name: string;
  expected_arrival: string;
  source?: "class_schedule" | "staff";
  source_class?: string;
  notes?: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
}

interface BackendArrivalException {
  id: number;
  student_id: number;
  exception_date: string;
  expected_arrival?: string | null;
  reason?: string | null;
  source?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

interface BackendArrivalNote {
  id: number;
  student_id: number;
  note_date: string;
  content: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

interface BackendArrivalData {
  schedules?: BackendArrivalSchedule[] | null;
  exceptions?: BackendArrivalException[] | null;
  notes?: BackendArrivalNote[] | null;
}

function mapArrivalSchedule(data: BackendArrivalSchedule): ArrivalSchedule {
  return {
    id: String(data.id),
    studentId: String(data.student_id),
    weekday: data.weekday,
    weekdayName: data.weekday_name,
    expectedArrival: data.expected_arrival,
    source: data.source,
    sourceClass: data.source_class,
    notes: data.notes,
    createdBy: String(data.created_by),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

function mapArrivalException(data: BackendArrivalException): ArrivalException {
  return {
    id: String(data.id),
    studentId: String(data.student_id),
    exceptionDate: data.exception_date,
    expectedArrival: data.expected_arrival,
    reason: data.reason,
    source: data.source,
    createdBy: String(data.created_by),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

function mapArrivalNote(data: BackendArrivalNote): ArrivalNote {
  return {
    id: String(data.id),
    studentId: String(data.student_id),
    noteDate: data.note_date,
    content: data.content,
    createdBy: String(data.created_by),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export type CareDaysSource = "weekly_plan" | "bookings";

/** A lesson the school maintains an end time for (#3372). */
export interface SchoolPeriod {
  /** 1-based number of the lesson. */
  period: number;
  /** "HH:MM" */
  endTime: string;
}

interface ArrivalSettings {
  careDaysSource: CareDaysSource;
  /** In lesson order; empty when the school maintains no lesson end times. */
  schoolPeriods: SchoolPeriod[];
  /** "HH:MM", or empty when the school maintains none (#3371). */
  defaultArrivalTime: string;
  /** "HH:MM", or empty when the school maintains none (#3371). */
  defaultPickupTime: string;
}

interface BackendArrivalSettings {
  care_days_source: CareDaysSource;
  school_periods?: Array<{ period: number; end_time: string }> | null;
  default_arrival_time: string;
  default_pickup_time: string;
}

/**
 * The usual clock times a school offers for one-click adoption in the weekly
 * plan (#3371). Empty strings offer nothing.
 */
export interface CareTimePresets {
  readonly arrival: string;
  readonly pickup: string;
}

export interface ArrivalScheduleInput {
  weekday: number;
  /** Empty string = care day whose time comes from the class timetable. */
  expected_arrival: string;
  notes?: string | null;
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

/** Mo–Fr with the backend's ISO weekday numbers (1 = Montag). */
export const WEEKDAYS: { value: number; label: string; short: string }[] =
  DEPARTURE_WEEKDAYS.map((day, index) => ({
    value: index + 1,
    label: day.label,
    short: day.label.slice(0, 2),
  }));

export async function fetchArrivalData(
  studentId: string,
  date?: string,
  toDate?: string,
): Promise<ArrivalData> {
  const query = new URLSearchParams();
  if (date) query.set("date", date);
  if (toDate) query.set("to", toDate);
  const suffix = query.size > 0 ? `?${query.toString()}` : "";
  const data = await bffJson<BackendArrivalData>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-schedules${suffix}`,
  );
  return {
    schedules: (data.schedules ?? []).map(mapArrivalSchedule),
    exceptions: (data.exceptions ?? []).map(mapArrivalException),
    notes: (data.notes ?? []).map(mapArrivalNote),
  };
}

export async function fetchBulkArrivalScheduleStatus(
  studentIds: string[],
): Promise<number> {
  const status = await bffJson<{ student_ids: number[] }>(
    "/api/students/arrival-schedules/status",
    {
      method: "POST",
      body: { student_ids: studentIds.map((id) => Number.parseInt(id, 10)) },
    },
  );
  return status.student_ids.length;
}

export async function fetchArrivalSettings(): Promise<ArrivalSettings> {
  const settings = await bffJson<BackendArrivalSettings>(
    "/api/students/arrival-settings",
  );
  return {
    careDaysSource: settings.care_days_source,
    schoolPeriods: (settings.school_periods ?? []).map((period) => ({
      period: period.period,
      endTime: period.end_time,
    })),
    defaultArrivalTime: settings.default_arrival_time,
    defaultPickupTime: settings.default_pickup_time,
  };
}

export async function updateArrivalSchedules(
  studentId: string,
  schedules: ArrivalScheduleInput[],
): Promise<ArrivalSchedule[]> {
  const data = await bffJson<BackendArrivalSchedule[]>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-schedules`,
    { method: "PUT", body: { schedules } },
  );
  return data.map(mapArrivalSchedule);
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
      throw new ApiError("Ungültige Gruppen-ID", 400, {
        code: "general.input",
      });
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
      throw new ApiError("Ungültige Kinderauswahl", 400, {
        code: "general.input",
      });
    }
    body.student_ids = studentIds;
  }
  return bffJson<unknown>("/api/students/arrival-schedules/bulk", {
    method: "POST",
    body,
  });
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
  const data = await bffJson<BackendArrivalException>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-exceptions`,
    { method: "POST", body: input },
  );
  return mapArrivalException(data);
}

export async function updateArrivalException(
  studentId: string,
  exceptionId: string,
  input: ArrivalExceptionInput,
): Promise<ArrivalException> {
  const data = await bffJson<BackendArrivalException>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-exceptions/${encodeURIComponent(exceptionId)}`,
    { method: "PUT", body: input },
  );
  return mapArrivalException(data);
}

export async function deleteArrivalException(
  studentId: string,
  exceptionId: string,
): Promise<void> {
  await bffJson<void>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-exceptions/${encodeURIComponent(exceptionId)}`,
    { method: "DELETE" },
  );
}

interface ArrivalNoteInput {
  note_date: string;
  content: string;
}

export async function createArrivalNote(
  studentId: string,
  input: ArrivalNoteInput,
): Promise<ArrivalNote> {
  const data = await bffJson<BackendArrivalNote>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-notes`,
    { method: "POST", body: input },
  );
  return mapArrivalNote(data);
}

export async function updateArrivalNote(
  studentId: string,
  noteId: string,
  input: ArrivalNoteInput,
): Promise<ArrivalNote> {
  const data = await bffJson<BackendArrivalNote>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-notes/${encodeURIComponent(noteId)}`,
    { method: "PUT", body: input },
  );
  return mapArrivalNote(data);
}

export async function deleteArrivalNote(
  studentId: string,
  noteId: string,
): Promise<void> {
  await bffJson<void>(
    `/api/students/${encodeURIComponent(studentId)}/arrival-notes/${encodeURIComponent(noteId)}`,
    {
      method: "DELETE",
    },
  );
}

/** The Unterrichtsschluss a school class carries (#2414). */
interface ClassArrivalTimes {
  school_class: string;
  /** Day code ("mon" … "fri") to HH:MM. Absent days have no time. */
  times: Record<string, string>;
  updated_at?: string;
}

export async function fetchClassArrivalTimes(
  schoolClass: string,
): Promise<ClassArrivalTimes> {
  return bffJson<ClassArrivalTimes>(
    `/api/students/class-arrival-times/${encodeURIComponent(schoolClass)}`,
  );
}

/**
 * One date on which a whole class arrives at a different time than its
 * Unterrichtsschluss says (#2962), for example "Unterricht fällt aus".
 */
export interface ClassArrivalException {
  school_class: string;
  /** YYYY-MM-DD */
  date: string;
  /** HH:MM */
  arrival_time: string;
  reason?: string | null;
  created_at: string;
  /** Wer die Ausnahme eingetragen hat: die OGS oder die Schule (#2970). */
  origin?: "ogs" | "school";
}

export interface ClassArrivalExceptionList {
  school_class: string;
  /** Whether the caller may set or remove exceptions (setting + permission). */
  can_edit: boolean;
  exceptions: ClassArrivalException[];
}

export interface ClassArrivalExceptionInput {
  /** HH:MM */
  arrival_time: string;
  reason?: string | null;
}

function classArrivalExceptionPath(schoolClass: string, date?: string): string {
  const base = `/api/students/class-arrival-exceptions/${encodeURIComponent(schoolClass)}`;
  return date ? `${base}/${encodeURIComponent(date)}` : base;
}

/** Lists the class-wide exceptions from today on (backend default: 60 days). */
export async function fetchClassArrivalExceptions(
  schoolClass: string,
): Promise<ClassArrivalExceptionList> {
  return bffJson<ClassArrivalExceptionList>(
    classArrivalExceptionPath(schoolClass),
  );
}

export async function upsertClassArrivalException(
  schoolClass: string,
  date: string,
  input: ClassArrivalExceptionInput,
): Promise<ClassArrivalException> {
  return bffJson<ClassArrivalException>(
    classArrivalExceptionPath(schoolClass, date),
    { method: "PUT", body: input },
  );
}

export async function deleteClassArrivalException(
  schoolClass: string,
  date: string,
): Promise<void> {
  await bffJson<void>(classArrivalExceptionPath(schoolClass, date), {
    method: "DELETE",
  });
}
