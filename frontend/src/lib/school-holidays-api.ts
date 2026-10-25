// Gesetzliche Feiertage der Schule (#4775). Das Backend berechnet sie aus
// dem Bundesland der Schule (operations.federal_state); die Liste ist nicht
// bearbeitbar. Proxy: /api/timetable/holidays.

import { apiErrorFromBody, transportFetch } from "~/lib/api-error";
import type { CalendarEvent } from "~/lib/personal-calendar-api";

export interface SchoolHoliday {
  readonly date: string; // YYYY-MM-DD
  readonly name: string;
}

export interface SchoolHolidays {
  /** ISO-Code wie „DE-NW“. */
  readonly federalState: string;
  /** Name des Bundeslands, z. B. „Nordrhein-Westfalen“. */
  readonly federalStateName: string;
  readonly holidays: readonly SchoolHoliday[];
}

interface BackendSchoolHolidays {
  readonly federal_state: string;
  readonly federal_state_name: string;
  readonly holidays: SchoolHoliday[] | null;
}

/** Feiertage im Fenster [from, to], beide Tage eingeschlossen. */
export async function getSchoolHolidays(
  fromISO: string,
  toISO: string,
): Promise<SchoolHolidays> {
  const params = new URLSearchParams({ from: fromISO, to: toISO });
  const response = await transportFetch(`/api/timetable/holidays?${params}`, {
    method: "GET",
    headers: { Accept: "application/json" },
    credentials: "include",
  });
  if (!response.ok) {
    throw apiErrorFromBody(
      `Anfrage fehlgeschlagen (HTTP ${response.status})`,
      response.status,
      await response.json().catch(() => undefined),
    );
  }
  const envelope = (await response.json()) as { data: BackendSchoolHolidays };
  return {
    federalState: envelope.data.federal_state,
    federalStateName: envelope.data.federal_state_name,
    holidays: envelope.data.holidays ?? [],
  };
}

/**
 * Die Feiertage als ganztägige Einträge im Kalenderraster. Sie lassen sich
 * weder beantworten noch ändern.
 */
export function holidaysAsCalendarEvents(
  holidays: readonly SchoolHoliday[],
  federalStateName: string,
): CalendarEvent[] {
  return holidays.map((holiday) => ({
    id: `public_holiday:${holiday.date}`,
    source: "public_holiday",
    title: holiday.name,
    description: `Gesetzlicher Feiertag in ${federalStateName}.`,
    start_date: holiday.date,
    end_date: holiday.date,
    start_time: "00:00",
    end_time: "23:59",
    all_day: true,
    can_respond: false,
    can_edit: false,
  }));
}
