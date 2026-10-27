// Client-side API für die Klassenansicht im Schul-Portal (#2207). Gleiche
// Report-Typen wie das Tenant-Portal (lib/class-day-api), aber die Next.js
// Routen unter /api/school/* laufen über die school-Session und rufen das
// Backend mit dem school-Token unter /school/class-day auf.

import { ApiError } from "./api-error";
import { authFetch } from "./api-helpers";
import type { StudentStatusKind } from "./student-status-days-api";
import type { ClassDayReport, MorningAttendanceStatus } from "./class-day-api";
import type {
  ClassArrivalException,
  ClassArrivalExceptionInput,
  ClassArrivalExceptionList,
} from "./student-arrival-api";

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

/** Antwort von GET /school/class-day/classes. */
export interface ClassDayClasses {
  classes: string[];
  /**
   * Ob die Lehrkraft eine andere Ankunftszeit für eine Klasse eintragen darf
   * (#2970): Berechtigung UND Freigabe der OGS in den Einstellungen.
   */
  can_write_arrival_exception: boolean;
  /**
   * Ob die Lehrkraft einen Ausflug oder eine Klassenfahrt melden darf
   * (#4799): Berechtigung UND Freigabe der OGS in den Einstellungen.
   */
  can_report_class_trip?: boolean;
  /**
   * Die Schule betreut Sa/So nach dem Freitagsplan
   * (operations.weekend_follows_friday, #3921): dann ist das Wochenende ein
   * Tag wie jeder andere und wird geladen.
   */
  weekend_follows_friday?: boolean;
  /**
   * Die Schulverwaltung (class_day:all_classes, #4802) lädt die Startseite
   * über die Tagesübersicht der ganzen Schule: ein Abruf statt einer
   * Klassenansicht je Klasse.
   */
  can_view_school_day?: boolean;
}

export async function fetchClassDayClassesSchool(): Promise<ClassDayClasses> {
  const response = await authFetch<ProxyResponse<Partial<ClassDayClasses>>>(
    "/api/school/class-day/classes",
  );
  return {
    classes: response.data.classes ?? [],
    can_write_arrival_exception:
      response.data.can_write_arrival_exception === true,
    can_report_class_trip: response.data.can_report_class_trip === true,
    weekend_follows_friday: response.data.weekend_follows_friday === true,
    can_view_school_day: response.data.can_view_school_day === true,
  };
}

/** Zahlen der Tagesübersicht, für die ganze Schule oder eine Klasse. */
export interface SchoolDayTotals {
  students: number;
  /** Am Vormittag als fehlend markiert oder krank, entschuldigt, Klassenfahrt. */
  missing: number;
  /**
   * Von der Lehrkraft als fehlend markiert, ohne Krankmeldung, Entschuldigung
   * oder Klassenfahrt (#4791). Fehlt bei älteren Antworten.
   */
  unreported?: number;
  /** Kinder mit Tagesstatus, „Heute abgemeldet“ eingeschlossen. */
  absent: number;
  sick: number;
  excused: number;
  class_trip: number;
  staying: number;
  leaving: number;
  list_entries: number;
  /** Kinder, deren Tag anders ist als sonst (#2294). */
  changes: number;
}

/** Ein Kind der Tagesübersicht, nur mit dem, was die Zahlen zählen. */
export interface SchoolDayChild {
  student_id?: string;
  list_entry?: boolean;
  list_entry_id?: string;
  first_name: string;
  last_name: string;
  status?: string;
  missing?: boolean;
  /** Fehlt ohne Meldung (#4791). */
  unreported?: boolean;
  /** Den Status hat die Schulverwaltung nachgetragen (#4791). */
  status_by_school?: boolean;
  after_school?: "stays" | "goes_home";
  changed?: boolean;
}

interface SchoolDayClass {
  school_class: string;
  totals: SchoolDayTotals;
  children: SchoolDayChild[];
}

/** Antwort von GET /school/class-day/school-day (#4802). */
export interface SchoolDayReport {
  date: string;
  weekday: string;
  school_day: boolean;
  enrollment_known: boolean;
  totals: SchoolDayTotals;
  classes: SchoolDayClass[];
  /**
   * Die Person darf für ein Kind, das ohne Meldung fehlt, eine Meldung
   * nachtragen oder sie zurücknehmen (#4791): der Tag ist heute und das Recht
   * liegt vor.
   */
  can_report_absences?: boolean;
}

export async function fetchSchoolDaySchool(
  date: string,
): Promise<SchoolDayReport> {
  const params = new URLSearchParams({ date });
  const response = await authFetch<ProxyResponse<SchoolDayReport>>(
    `/api/school/class-day/school-day?${params.toString()}`,
  );
  return response.data;
}

export async function fetchClassDaySchool(
  schoolClass: string,
  date: string,
): Promise<ClassDayReport> {
  const params = new URLSearchParams({ class: schoolClass, date });
  const response = await authFetch<ProxyResponse<ClassDayReport>>(
    `/api/school/class-day?${params.toString()}`,
  );
  return response.data;
}

// Tagesausnahme einer Klasse aus moto schule (#2970). Der Klassenname reist
// in der Adresse als Segment; Next dekodiert Route-Handler-Parameter, das
// Backend prüft die Zuweisung.

function arrivalExceptionPath(schoolClass: string, date: string): string {
  return `/api/school/class-day/arrival-exceptions/${encodeURIComponent(schoolClass)}/${encodeURIComponent(date)}`;
}

export async function fetchClassArrivalExceptionsSchool(
  schoolClass: string,
): Promise<ClassArrivalExceptionList> {
  const params = new URLSearchParams({ class: schoolClass });
  const response = await authFetch<ProxyResponse<ClassArrivalExceptionList>>(
    `/api/school/class-day/arrival-exceptions?${params.toString()}`,
  );
  return response.data;
}

export async function upsertClassArrivalExceptionSchool(
  schoolClass: string,
  date: string,
  input: ClassArrivalExceptionInput,
): Promise<ClassArrivalException> {
  const response = await authFetch<ProxyResponse<ClassArrivalException>>(
    arrivalExceptionPath(schoolClass, date),
    { method: "PUT", body: input },
  );
  return response.data;
}

export async function deleteClassArrivalExceptionSchool(
  schoolClass: string,
  date: string,
): Promise<void> {
  await authFetch<unknown>(arrivalExceptionPath(schoolClass, date), {
    method: "DELETE",
  });
}

/**
 * Beginn des ersten Betreuungsblocks der Klasse an dem Tag ("HH:MM"), null
 * wenn keiner geplant ist. Vorbelegung für „Unterricht fällt aus“.
 */
export async function fetchClassBlockStartSchool(
  schoolClass: string,
  isoDate: string,
): Promise<string | null> {
  const params = new URLSearchParams({ class: schoolClass, date: isoDate });
  const response = await authFetch<ProxyResponse<{ start?: string }>>(
    `/api/school/class-day/arrival-exceptions/block-start?${params.toString()}`,
  );
  const start = response.data.start ?? "";
  return start === "" ? null : start;
}

/** Was die Lehrkraft in moto schule meldet (#4799). */
export interface ClassTripReportInput {
  studentIds: string[];
  from: string;
  to: string;
  reason?: string;
  /** "HH:MM", nur bei einem einzelnen Tag. */
  returnTime?: string;
}

/** Ein Tag, an dem ein gewähltes Kind schon einen Status hat (#4799). */
export interface ClassTripConflictDay {
  student_id: string;
  date: string;
  status: StudentStatusKind;
}

/**
 * Liest die blockierenden Tage aus der Ablehnung einer Meldung, wie die OGS
 * sie bei ihrer Klassenfahrt zeigt. Ohne genannte Tage `null`.
 */
export function classTripConflictsFromError(
  err: unknown,
): { conflicts: ClassTripConflictDay[]; total: number } | null {
  if (
    !(err instanceof ApiError) ||
    err.code !== "classday.class_trip_status_conflict"
  ) {
    return null;
  }
  const raw = err.details?.conflicts;
  if (!Array.isArray(raw) || raw.length === 0) {
    return null;
  }
  const conflicts = raw.flatMap((entry: unknown): ClassTripConflictDay[] => {
    if (!entry || typeof entry !== "object") return [];
    const { student_id, date, status } = entry as Record<string, unknown>;
    if (
      typeof student_id !== "number" ||
      typeof date !== "string" ||
      (status !== "sick" && status !== "excused" && status !== "class_trip")
    ) {
      return [];
    }
    return [{ student_id: student_id.toString(), date, status }];
  });
  const count = err.details?.conflict_count;
  const total =
    typeof count === "number" && Number.isFinite(count)
      ? count
      : conflicts.length;
  return conflicts.length > 0 ? { conflicts, total } : null;
}

/** Was eine Meldung geschrieben hat (#4799). */
export interface ClassTripReportResult {
  /** "class_trip": Status Klassenfahrt; "return_time": Ankunft in der OGS. */
  kind: "class_trip" | "return_time";
  student_count: number;
  date_count: number;
}

/**
 * Ausflug oder Klassenfahrt einer zugewiesenen Klasse melden (#4799). Ohne
 * Rückkehrzeit gilt jeder Tag als Klassenfahrt; mit Rückkehrzeit (nur ein
 * Tag) kommen die Kinder um diese Uhrzeit in die OGS.
 */
export async function reportClassTripSchool(
  schoolClass: string,
  input: ClassTripReportInput,
): Promise<ClassTripReportResult> {
  const response = await authFetch<ProxyResponse<ClassTripReportResult>>(
    `/api/school/class-day/class-trips/${encodeURIComponent(schoolClass)}`,
    {
      method: "POST",
      body: {
        from: input.from,
        to: input.to,
        student_ids: input.studentIds.map(Number),
        ...(input.reason ? { reason: input.reason } : {}),
        ...(input.returnTime ? { return_time: input.returnTime } : {}),
      },
    },
  );
  return response.data;
}

/**
 * Ziel eines Hinweises an die OGS (#4795): die ganze Klasse ohne studentId,
 * sonst ein Kind der Klasse. Ein Hinweis gilt nur für heute.
 */
function dayHintPath(
  schoolClass: string,
  date: string,
  studentId?: number,
): string {
  const base = `/api/school/class-day/day-hints/${encodeURIComponent(schoolClass)}/${encodeURIComponent(date)}`;
  return studentId
    ? `${base}/students/${encodeURIComponent(String(studentId))}`
    : base;
}

export async function saveDayHintSchool(
  schoolClass: string,
  date: string,
  body: string,
  studentId?: number,
): Promise<void> {
  await authFetch<unknown>(dayHintPath(schoolClass, date, studentId), {
    method: "PUT",
    body: { body },
  });
}

export async function deleteDayHintSchool(
  schoolClass: string,
  date: string,
  studentId?: number,
): Promise<void> {
  await authFetch<unknown>(dayHintPath(schoolClass, date, studentId), {
    method: "DELETE",
  });
}

/**
 * Eine Markierung der Anwesenheit am Vormittag (#4790): ein Kind mit
 * OGS-Datensatz über student_id, ein Klassenlisteneintrag über list_entry_id
 * (String, wie in der Tageszeile).
 */
export type MorningAttendanceMark =
  | { student_id: number; status: MorningAttendanceStatus }
  | { list_entry_id: string; status: MorningAttendanceStatus };

/** Speichert die Markierungen einer zugewiesenen Klasse für heute. */
export async function recordMorningAttendanceSchool(
  schoolClass: string,
  date: string,
  marks: readonly MorningAttendanceMark[],
): Promise<void> {
  await authFetch<unknown>(
    `/api/school/class-day/morning-attendance/${encodeURIComponent(schoolClass)}/${encodeURIComponent(date)}`,
    { method: "PUT", body: { marks } },
  );
}

/** Was die Lehrkraft für einen Hausaufgaben-Eintrag schickt (#4796). */
export interface HomeworkInput {
  subject: string | null;
  description: string;
}

/** Trägt Hausaufgaben für eine zugewiesene Klasse und einen Tag ein. */
export async function addHomeworkSchool(
  schoolClass: string,
  date: string,
  input: HomeworkInput,
): Promise<void> {
  await authFetch<unknown>(
    `/api/school/class-day/homework/${encodeURIComponent(schoolClass)}`,
    { method: "POST", body: { date, ...input } },
  );
}

/** Ändert einen eigenen Hausaufgaben-Eintrag. */
export async function changeHomeworkSchool(
  schoolClass: string,
  homeworkId: string,
  input: HomeworkInput,
): Promise<void> {
  await authFetch<unknown>(
    `/api/school/class-day/homework/${encodeURIComponent(schoolClass)}/${encodeURIComponent(homeworkId)}`,
    { method: "PUT", body: input },
  );
}

/** Löscht einen eigenen Hausaufgaben-Eintrag. */
export async function deleteHomeworkSchool(
  schoolClass: string,
  homeworkId: string,
): Promise<void> {
  await authFetch<unknown>(
    `/api/school/class-day/homework/${encodeURIComponent(schoolClass)}/${encodeURIComponent(homeworkId)}`,
    { method: "DELETE" },
  );
}

function classHandoverPath(schoolClass: string, date: string): string {
  return `/api/school/class-day/handover/${encodeURIComponent(schoolClass)}/${encodeURIComponent(date)}`;
}

/** Die übergebenen Kinder, die in die OGS gehen: „da“ und „nicht da“. */
export interface ClassHandoverChildren {
  readonly present: readonly number[];
  readonly absent: readonly number[];
}

/** Übergibt eine zugewiesene Klasse heute an die OGS (#4794). */
export async function handOverClassSchool(
  schoolClass: string,
  date: string,
  children: ClassHandoverChildren,
): Promise<void> {
  await authFetch<unknown>(classHandoverPath(schoolClass, date), {
    method: "PUT",
    body: {
      present_student_ids: children.present,
      absent_student_ids: children.absent,
    },
  });
}

/** Nimmt die heutige Übergabe einer zugewiesenen Klasse zurück (#4794). */
export async function withdrawClassHandoverSchool(
  schoolClass: string,
  date: string,
): Promise<void> {
  await authFetch<unknown>(classHandoverPath(schoolClass, date), {
    method: "DELETE",
  });
}

/** Was die Schulverwaltung nachträgt (#4791). */
export type AbsenceReportStatus = "sick" | "excused";

function absenceReportPath(
  schoolClass: string,
  date: string,
  studentId: string,
): string {
  return `/api/school/class-day/absence-reports/${encodeURIComponent(schoolClass)}/${encodeURIComponent(date)}/students/${encodeURIComponent(studentId)}`;
}

/**
 * Trägt für ein Kind, das ohne Meldung fehlt, „krank“ oder „entschuldigt“
 * für heute nach (#4791). Die OGS sieht den Status wie jeden anderen.
 */
export async function reportAbsenceSchool(
  schoolClass: string,
  date: string,
  studentId: string,
  status: AbsenceReportStatus,
): Promise<void> {
  await authFetch<unknown>(absenceReportPath(schoolClass, date, studentId), {
    method: "PUT",
    body: { status },
  });
}

/** Nimmt die Meldung zurück, die die Schulverwaltung heute nachgetragen hat. */
export async function withdrawAbsenceReportSchool(
  schoolClass: string,
  date: string,
  studentId: string,
): Promise<void> {
  await authFetch<unknown>(absenceReportPath(schoolClass, date, studentId), {
    method: "DELETE",
  });
}
