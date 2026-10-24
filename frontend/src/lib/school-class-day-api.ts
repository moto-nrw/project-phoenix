// Client-side API für die Klassenansicht im Schul-Portal (#2207). Gleiche
// Report-Typen wie das Tenant-Portal (lib/class-day-api), aber die Next.js
// Routen unter /api/school/* laufen über die school-Session und rufen das
// Backend mit dem school-Token unter /school/class-day auf.

import { authFetch } from "./api-helpers";
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
   * Die Schule betreut Sa/So nach dem Freitagsplan
   * (operations.weekend_follows_friday, #3921): dann ist das Wochenende ein
   * Tag wie jeder andere und wird geladen.
   */
  weekend_follows_friday?: boolean;
}

export async function fetchClassDayClassesSchool(): Promise<ClassDayClasses> {
  const response = await authFetch<ProxyResponse<Partial<ClassDayClasses>>>(
    "/api/school/class-day/classes",
  );
  return {
    classes: response.data.classes ?? [],
    can_write_arrival_exception:
      response.data.can_write_arrival_exception === true,
    weekend_follows_friday: response.data.weekend_follows_friday === true,
  };
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
