// Schulferien des Bundeslands der Schule (#5345). moto liest sie täglich aus
// der OpenHolidays API; die Seiten lesen nur den gespeicherten Stand. Der
// Hinweis zählt geplante Regeltermine in den Ferien der nächsten acht Wochen.
// Proxy: /api/timetable/school-vacations und /school-vacation-hints.

import { apiErrorFromBody, transportFetch } from "~/lib/api-error";

import { timetableKeys } from "~/lib/swr/portal-keys";

/** SWR-Schlüssel des Ferienhinweises; Planflächen laden ihn nach Änderungen neu. */
export const SCHOOL_VACATION_HINTS_KEY = timetableKeys.schoolVacationHints();

export interface SchoolVacation {
  readonly name: string;
  readonly startDate: string; // YYYY-MM-DD
  readonly endDate: string; // YYYY-MM-DD
  /** Die Schule hat bestätigt, dass sie in diesen Ferien Betreuung plant. */
  readonly acknowledged: boolean;
}

export interface SchoolVacations {
  readonly federalState: string;
  readonly federalStateName: string;
  /** Letzter Abgleich; null, solange moto die Ferien noch nie gelesen hat. */
  readonly syncedAt: string | null;
  readonly vacations: readonly SchoolVacation[];
}

export interface SchoolVacationHint {
  readonly name: string;
  readonly startDate: string;
  readonly endDate: string;
  /** Erster Tag, ab dem eine Absage greift: Ferienbeginn oder heute. */
  readonly cancelFrom: string;
  /** Geplante Regeltermine in den Ferien ab cancelFrom. */
  readonly count: number;
}

interface BackendSchoolVacation {
  readonly name: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly acknowledged: boolean;
}

interface BackendSchoolVacations {
  readonly federal_state: string;
  readonly federal_state_name: string;
  readonly synced_at: string | null;
  readonly vacations: BackendSchoolVacation[] | null;
}

interface BackendSchoolVacationHint {
  readonly name: string;
  readonly start_date: string;
  readonly end_date: string;
  readonly cancel_from: string;
  readonly count: number;
}

async function readData<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw apiErrorFromBody(
      `Anfrage fehlgeschlagen (HTTP ${response.status})`,
      response.status,
      await response.json().catch(() => undefined),
    );
  }
  const envelope = (await response.json()) as { data: T };
  return envelope.data;
}

/** Ferien im Fenster [from, to], beide Tage eingeschlossen (höchstens 400 Tage). */
export async function getSchoolVacations(
  fromISO: string,
  toISO: string,
): Promise<SchoolVacations> {
  const params = new URLSearchParams({ from: fromISO, to: toISO });
  const data = await readData<BackendSchoolVacations>(
    await transportFetch(`/api/timetable/school-vacations?${params}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    }),
  );
  return {
    federalState: data.federal_state,
    federalStateName: data.federal_state_name,
    syncedAt: data.synced_at,
    vacations: (data.vacations ?? []).map((vacation) => ({
      name: vacation.name,
      startDate: vacation.start_date,
      endDate: vacation.end_date,
      acknowledged: vacation.acknowledged,
    })),
  };
}

export async function getSchoolVacationHints(): Promise<SchoolVacationHint[]> {
  const data = await readData<{ hints: BackendSchoolVacationHint[] | null }>(
    await transportFetch("/api/timetable/school-vacation-hints", {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    }),
  );
  return (data.hints ?? []).map((hint) => ({
    name: hint.name,
    startDate: hint.start_date,
    endDate: hint.end_date,
    cancelFrom: hint.cancel_from,
    count: hint.count,
  }));
}

/** Bestätigt für die ganze Schule: In diesen Ferien ist Betreuung geplant. */
export async function acknowledgeSchoolVacation(
  startDate: string,
  endDate: string,
): Promise<void> {
  await readData<unknown>(
    await transportFetch(
      "/api/timetable/school-vacation-hints/acknowledgements",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ start_date: startDate, end_date: endDate }),
      },
    ),
  );
}
