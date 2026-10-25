/**
 * Client der Teilnehmerliste für die ausrichtende OGS (#4665). Spricht mit
 * den Next.js-Routen unter /api/holiday-weeks, die mit dem Mitarbeiter-JWT an
 * das Backend weiterreichen. Die Schule sieht nur die bestätigten Kinder der
 * Wochen, die sie ausrichtet, und nur bis zum letzten Tag der Woche.
 * IDs kommen als Strings heraus.
 */

import { apiErrorFromText, transportFetch } from "~/lib/api-error";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";

export interface HostedHolidayWeek {
  readonly id: string;
  readonly periodName: string;
  /** Kalendertage als „YYYY-MM-DD“. */
  readonly startsOn: string;
  readonly endsOn: string;
  readonly participants: number;
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

export interface HolidayParticipant {
  readonly childId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthDate: string;
  /** `null` bei einem Feriengast. */
  readonly homeSchoolId: string | null;
  readonly homeSchoolName: string;
  readonly timeModelId: string;
}

export interface HolidayParticipantList {
  readonly week: HostedHolidayWeek;
  readonly timeModels: HolidayTimeModelCount[];
  readonly homeSchools: HolidayHomeSchoolCount[];
  readonly participants: HolidayParticipant[];
}

interface BackendWeek {
  id: number | string;
  period_name: string;
  starts_on: string;
  ends_on: string;
  participants: number;
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
  participants: {
    child_id: number | string;
    first_name: string;
    last_name: string;
    birth_date: string;
    home_school_id: number | string | null;
    home_school_name: string;
    time_model_id: number | string;
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
  };
}

/** Die Antwort des Backends in der Form der Oberfläche. */
export function mapHolidayParticipantList(
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
    participants: list.participants.map((participant) => ({
      childId: String(participant.child_id),
      firstName: participant.first_name,
      lastName: participant.last_name,
      birthDate: participant.birth_date,
      homeSchoolId: optionalId(participant.home_school_id),
      homeSchoolName: participant.home_school_name,
      timeModelId: String(participant.time_model_id),
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
