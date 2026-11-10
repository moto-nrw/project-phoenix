"use client";

/**
 * Tage ohne Betreuung je Ferienwoche im Editor (#4702). Feiertage und
 * Schließtage des Trägers kommen aus dem Kalender des Trägers, solange der
 * Editor die Tage ändert; die Schließtage, die eine Schule selbst einträgt,
 * nennt das Backend für die gespeicherte Woche. Gespeichert wird eine Woche
 * mit so einem Tag nicht; das Backend prüft es noch einmal.
 */

import { useSession } from "next-auth/react";

import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import type { HolidayNoCareDay, HolidayPeriod } from "~/lib/traeger-api";
import {
  fetchTraegerCalendar,
  type TraegerCalendar,
} from "~/lib/traeger-closing-days";
import { isoDaysBetween } from "./closing-day-format";
import type { WeekDraft } from "./holiday-period-model";

export interface WeekNoCareDays {
  readonly days: readonly HolidayNoCareDay[];
  /** The stored week as it is: its bookings stay, it takes no new one. */
  readonly keptAsStored: boolean;
}

/** The days without care of one week, from the calendar and the stored week. */
export function noCareDaysOfWeek(
  week: WeekDraft,
  calendar: TraegerCalendar | undefined,
  stored: HolidayPeriod | undefined,
): WeekNoCareDays {
  if (
    !week.startsOn ||
    !week.endsOn ||
    !week.hostSchoolId ||
    week.endsOn < week.startsOn
  ) {
    return { days: [], keptAsStored: false };
  }
  const inWeek = (date: string) => date >= week.startsOn && date <= week.endsOn;
  const byDate = new Map<string, HolidayNoCareDay>();
  const school = calendar?.schools.find((s) => s.id === week.hostSchoolId);
  for (const region of calendar?.regions ?? []) {
    if (region.code !== school?.federalStateCode) continue;
    for (const holiday of region.holidays) {
      if (inWeek(holiday.date)) {
        byDate.set(holiday.date, {
          date: holiday.date,
          kind: "holiday",
          name: holiday.name,
        });
      }
    }
  }
  for (const closing of calendar?.closingDays ?? []) {
    if (!closing.schoolIds.includes(week.hostSchoolId)) continue;
    for (const date of isoDaysBetween(closing.startDate, closing.endDate)) {
      if (inWeek(date) && !byDate.has(date)) {
        byDate.set(date, { date, kind: "closing_day", name: closing.reason });
      }
    }
  }
  const storedWeek = stored?.weeks.find(
    (w) => w.id !== undefined && w.id === week.id,
  );
  if (storedWeek?.hostSchoolId === week.hostSchoolId) {
    for (const day of storedWeek.noCareDays ?? []) {
      if (inWeek(day.date) && !byDate.has(day.date)) byDate.set(day.date, day);
    }
  }
  const keptAsStored =
    storedWeek !== undefined &&
    storedWeek.startsOn === week.startsOn &&
    storedWeek.endsOn === week.endsOn &&
    storedWeek.hostSchoolId === week.hostSchoolId;
  return {
    days: [...byDate.values()].sort((a, b) => a.date.localeCompare(b.date)),
    keptAsStored,
  };
}

/** The days without care of every week of the draft, keyed by week key. */
export function weekNoCareDays(
  weeks: readonly WeekDraft[],
  calendar: TraegerCalendar | undefined,
  stored: HolidayPeriod | undefined,
): ReadonlyMap<string, WeekNoCareDays> {
  return new Map(
    weeks.map((week) => [week.key, noCareDaysOfWeek(week, calendar, stored)]),
  );
}

/**
 * The Träger calendar from today on, for the hints of the holiday period
 * editor: a holiday season lies within the next 400 days.
 */
export function useHolidayEditorCalendar(): TraegerCalendar | undefined {
  const { data: session } = useSession();
  const today = useBerlinToday();
  const to = toISODate(addDays(parseISODate(today), 400));
  const { data } = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.closingDays(today, to) : null,
    () => fetchTraegerCalendar(today, to),
    { keepPreviousData: true },
  );
  return data;
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}
