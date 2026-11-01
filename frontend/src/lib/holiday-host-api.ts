/**
 * Client der Teilnehmerliste für die ausrichtende OGS (#4665). Spricht mit
 * den Next.js-Routen unter /api/holiday-weeks, die mit dem Mitarbeiter-JWT an
 * das Backend weiterreichen. Die Schule sieht nur die bestätigten Kinder der
 * Wochen, die sie ausrichtet, und nur bis zum letzten Tag der Woche.
 * IDs kommen als Strings heraus.
 */

import { apiErrorFromText, transportFetch } from "~/lib/api-error";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";

export interface HostedHolidayWeek {
  readonly id: string;
  readonly periodName: string;
  /** Kalendertage als „YYYY-MM-DD“. */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly participants: number;
  /** Gruppe der Feriengäste dieser Woche (#4666), `null` vor der Wahl. */
  readonly guestGroupId: string | null;
  /** Feriengäste in dieser Gruppe, deren Zugriff nicht vorzeitig endete. */
  readonly guests: number;
}

export interface HolidayTimeModelCount {
  readonly id: string;
  readonly name: string;
  readonly startsAt: string;
  readonly endsAt: string;
  readonly count: number;
}

export interface HolidayHomeSchoolCount {
  /** `null` zählt die Feriengäste ohne Schule des Trägers. */
  readonly schoolId: string | null;
  readonly name: string;
  readonly count: number;
}

/**
 * Antwort auf eine Frage, die der Träger für die OGS freigibt (#5202). In
 * `answers` nur Fragen ohne Gesundheitsangabe; Antworten auf Gesundheitsfragen
 * stehen in `health` (#4722). `value` ist `true`/`false` bei Ja/Nein, die
 * gewählte Antwort bei einer Auswahl, ein Kalendertag „YYYY-MM-DD“ bei einem
 * Datum, sonst der Text; `null` ohne Antwort.
 */
export interface HolidayHostAnswer {
  readonly key: string;
  readonly label: string;
  readonly type: string;
  readonly value: boolean | string | null;
}

/**
 * Die Gesundheitsangaben eines Kindes aus der Ferienanmeldung (#4722): nur,
 * wenn die Eltern der Weitergabe an die OGS zugestimmt haben.
 */
export interface HolidayHostHealth {
  readonly allergies: string;
  readonly medication: string;
  readonly healthNotes: string;
  readonly answers: HolidayHostAnswer[];
}

interface BackendHostHealth {
  allergies: string;
  medication: string;
  health_notes: string;
  answers?: HolidayHostAnswer[] | null;
}

function mapHealth(
  health: BackendHostHealth | null | undefined,
): HolidayHostHealth | null {
  if (!health) return null;
  return {
    allergies: health.allergies,
    medication: health.medication,
    healthNotes: health.health_notes,
    answers: health.answers ?? [],
  };
}

/** Eine freigegebene Frage der Liste: eine Spalte. */
interface HolidayHostQuestion {
  readonly key: string;
  readonly label: string;
  readonly type: string;
}

/** Die zweite sorgeberechtigte Person: Name und Telefon, keine E-Mail. */
interface HolidayHostGuardian {
  readonly name: string;
  readonly phone: string;
}

export interface HolidayParticipant {
  readonly childId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthDate: string;
  /** `null` bei einem Feriengast. */
  readonly homeSchoolId: string | null;
  readonly homeSchoolName: string;
  readonly timeModelId: string;
  /** `null` ohne zweite sorgeberechtigte Person. */
  readonly secondGuardian: HolidayHostGuardian | null;
  readonly answers: HolidayHostAnswer[];
  /** `null` ohne Gesundheitsangaben oder ohne Zustimmung der Eltern. */
  readonly health: HolidayHostHealth | null;
}

export interface HolidayParticipantList {
  readonly week: HostedHolidayWeek;
  readonly timeModels: HolidayTimeModelCount[];
  readonly homeSchools: HolidayHomeSchoolCount[];
  /** Die freigegebenen Fragen; leer, wenn der Träger keine freigibt. */
  readonly questions: HolidayHostQuestion[];
  readonly participants: HolidayParticipant[];
}

/** Eine Antwort zum Lesen: „Ja“, „Nein“, ein deutsches Datum oder der Text. */
export function formatHolidayHostAnswer(
  answer: Pick<HolidayHostAnswer, "type" | "value"> | undefined,
): string {
  if (answer === undefined || answer.value === null) return "–";
  if (typeof answer.value === "boolean") return answer.value ? "Ja" : "Nein";
  if (answer.type === "date") return formatDate(answer.value);
  return answer.value;
}

interface BackendWeek {
  id: number | string;
  period_name: string;
  starts_on: string;
  ends_on: string;
  participants: number;
  guest_group_id?: number | string | null;
  guests?: number;
}

interface BackendParticipantList {
  week: BackendWeek;
  time_models: {
    id: number | string;
    name: string;
    starts_at: string;
    ends_at: string;
    count: number;
  }[];
  home_schools: {
    school_id: number | string | null;
    name: string;
    count: number;
  }[];
  questions?: HolidayHostQuestion[] | null;
  participants: {
    child_id: number | string;
    first_name: string;
    last_name: string;
    birth_date: string;
    home_school_id: number | string | null;
    home_school_name: string;
    time_model_id: number | string;
    second_guardian?: HolidayHostGuardian | null;
    answers?: HolidayHostAnswer[] | null;
    health?: BackendHostHealth | null;
  }[];
}

const BASE = "/api/holiday-weeks";

function optionalId(id: number | string | null): string | null {
  return id === null ? null : String(id);
}

function mapWeek(week: BackendWeek): HostedHolidayWeek {
  return {
    id: String(week.id),
    periodName: week.period_name,
    startsOn: week.starts_on,
    endsOn: week.ends_on,
    participants: week.participants,
    guestGroupId: optionalId(week.guest_group_id ?? null),
    guests: week.guests ?? 0,
  };
}

/** Die Antwort des Backends in der Form der Oberfläche. */
function mapHolidayParticipantList(
  list: BackendParticipantList,
): HolidayParticipantList {
  return {
    week: mapWeek(list.week),
    timeModels: list.time_models.map((model) => ({
      id: String(model.id),
      name: model.name,
      startsAt: model.starts_at,
      endsAt: model.ends_at,
      count: model.count,
    })),
    homeSchools: list.home_schools.map((school) => ({
      schoolId: optionalId(school.school_id),
      name: school.name,
      count: school.count,
    })),
    questions: list.questions ?? [],
    participants: list.participants.map((participant) => ({
      childId: String(participant.child_id),
      firstName: participant.first_name,
      lastName: participant.last_name,
      birthDate: participant.birth_date,
      homeSchoolId: optionalId(participant.home_school_id),
      homeSchoolName: participant.home_school_name,
      timeModelId: String(participant.time_model_id),
      secondGuardian: participant.second_guardian ?? null,
      answers: participant.answers ?? [],
      health: mapHealth(participant.health),
    })),
  };
}

async function readData<T>(url: string, fallback: string): Promise<T> {
  const response = await transportFetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw apiErrorFromText(fallback, response.status, await response.text());
  }
  const body = (await response.json()) as { data?: T };
  if (body.data === undefined) throw apiErrorFromText(fallback, 500, "");
  return body.data;
}

/** Die Wochen, die die Schule ausrichtet und die noch nicht vorbei sind. */
export async function fetchHostedHolidayWeeks(): Promise<HostedHolidayWeek[]> {
  const weeks = await readData<BackendWeek[] | null>(
    BASE,
    "Ferienwochen konnten nicht geladen werden",
  );
  return (weeks ?? []).map(mapWeek);
}

/** Die Teilnehmerliste einer Woche. Jeder Abruf steht im Zugriffsprotokoll. */
export async function fetchHolidayParticipants(
  weekId: string,
): Promise<HolidayParticipantList> {
  return mapHolidayParticipantList(
    await readData<BackendParticipantList>(
      `${BASE}/${encodeURIComponent(weekId)}/participants`,
      "Teilnehmerliste konnte nicht geladen werden",
    ),
  );
}

/**
 * Lädt die Teilnehmerliste als CSV oder Excel unter dem Dateinamen des
 * Backends. Erst holen, dann speichern: ein Fehler bleibt als Meldung auf der
 * Seite.
 */
export async function downloadHolidayParticipants(
  weekId: string,
  format: "csv" | "xlsx",
): Promise<void> {
  const response = await transportFetch(
    `${BASE}/${encodeURIComponent(weekId)}/participants/export/${format}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw apiErrorFromText(
      "Die Datei konnte nicht erstellt werden",
      response.status,
      await response.text(),
    );
  }
  const blob = await response.blob();
  downloadBlob(
    blob,
    filenameFromDisposition(response) ?? `teilnehmerliste.${format}`,
  );
}

// ─── Feriengäste (#4666) ────────────────────────────────────────────────────

export interface HolidayGuestContact {
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
}

/**
 * Was die ausrichtende OGS von einem Feriengast über den Namen hinaus sieht:
 * Betreuung, Notfall und Abholung, dazu die Antworten, die der Träger
 * freigibt (#5202), und die Gesundheitsangaben (#4722).
 */
export interface HolidayGuestSheet {
  readonly studentId: string;
  /** Kalendertage der Ferienwoche als „YYYY-MM-DD“. */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly timeModelName: string;
  readonly timeModelStartsAt: string;
  readonly timeModelEndsAt: string;
  /** `null` bei einem Kind ohne Schule des Trägers. */
  readonly homeSchoolId: string | null;
  readonly guestSchoolName: string;
  readonly emergencyContacts: HolidayGuestContact[];
  readonly pickupContacts: HolidayGuestContact[];
  readonly answers: HolidayHostAnswer[];
  /** `null` ohne Gesundheitsangaben oder ohne Zustimmung der Eltern. */
  readonly health: HolidayHostHealth | null;
  /** Nach dem letzten Tag der Woche: nur noch sieben Tage zum Nachlesen. */
  readonly readOnly: boolean;
}

interface BackendGuestSheet {
  student_id: number | string;
  starts_on: string;
  ends_on: string;
  time_model_name: string;
  time_model_starts_at: string;
  time_model_ends_at: string;
  home_school_id: number | string | null;
  guest_school_name: string;
  emergency_contacts: HolidayGuestContact[] | null;
  pickup_contacts: HolidayGuestContact[] | null;
  answers?: HolidayHostAnswer[] | null;
  health?: BackendHostHealth | null;
  read_only: boolean;
}

/** Die Klasse, unter der ein Feriengast in der ausrichtenden OGS steht. */
export const HOLIDAY_GUEST_SCHOOL_CLASS = "Feriengast";

/**
 * Wählt die Gruppe für die Feriengäste einer Woche. Das Backend übernimmt
 * die bestätigten Gäste sofort und gibt ihre Zahl zurück.
 */
export async function assignHolidayGuestGroup(
  weekId: string,
  groupId: string,
): Promise<number> {
  const response = await transportFetch(
    `${BASE}/${encodeURIComponent(weekId)}/guest-group`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group_id: groupId }),
    },
  );
  if (!response.ok) {
    throw apiErrorFromText(
      "Die Feriengäste konnten nicht übernommen werden",
      response.status,
      await response.text(),
    );
  }
  const body = (await response.json()) as { data?: { guests?: number } };
  return body.data?.guests ?? 0;
}

/** Betreuung, Notfall und Abholung eines Feriengasts. Jeder Abruf steht im Zugriffsprotokoll. */
export async function fetchHolidayGuestSheet(
  studentId: string,
): Promise<HolidayGuestSheet> {
  const sheet = await readData<BackendGuestSheet>(
    `/api/holiday-guests/${encodeURIComponent(studentId)}`,
    "Die Angaben zum Feriengast konnten nicht geladen werden",
  );
  return {
    studentId: String(sheet.student_id),
    startsOn: sheet.starts_on,
    endsOn: sheet.ends_on,
    timeModelName: sheet.time_model_name,
    timeModelStartsAt: sheet.time_model_starts_at,
    timeModelEndsAt: sheet.time_model_ends_at,
    homeSchoolId: optionalId(sheet.home_school_id),
    guestSchoolName: sheet.guest_school_name,
    emergencyContacts: sheet.emergency_contacts ?? [],
    pickupContacts: sheet.pickup_contacts ?? [],
    answers: sheet.answers ?? [],
    health: mapHealth(sheet.health),
    readOnly: sheet.read_only,
  };
}

// ─── Eigene Kinder der ausrichtenden OGS (#4722) ────────────────────────────

/**
 * Was eine bestätigte Ferienanmeldung zur Gesundheit eines eigenen Kindes
 * angibt, für eine Woche, die die Schule ausrichtet. Es ändert das
 * Gesundheitsfeld der Schule nicht.
 */
export interface HolidayStudentHealth {
  readonly weekId: string;
  readonly periodName: string;
  /** Kalendertage der Ferienwoche als „YYYY-MM-DD“. */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly health: HolidayHostHealth;
}

interface BackendStudentHealth {
  week_id: number | string;
  period_name: string;
  starts_on: string;
  ends_on: string;
  health: BackendHostHealth;
}

/**
 * Die Gesundheitsangaben der Ferienanmeldungen eines eigenen Kindes bis zum
 * letzten Tag der Woche. Jeder Abruf mit Angaben steht im Zugriffsprotokoll.
 */
export async function fetchHolidayStudentHealth(
  studentId: string,
): Promise<HolidayStudentHealth[]> {
  const entries = await readData<BackendStudentHealth[] | null>(
    `${BASE}/students/${encodeURIComponent(studentId)}/health`,
    "Die Angaben aus der Ferienanmeldung konnten nicht geladen werden",
  );
  return (entries ?? []).map((entry) => ({
    weekId: String(entry.week_id),
    periodName: entry.period_name,
    startsOn: entry.starts_on,
    endsOn: entry.ends_on,
    health: mapHealth(entry.health) ?? {
      allergies: "",
      medication: "",
      healthNotes: "",
      answers: [],
    },
  }));
}
