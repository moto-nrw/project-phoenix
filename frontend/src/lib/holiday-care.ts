/**
 * Ferienbetreuung in den Planungsrastern: an welchem Tag die Schule eine
 * Ferienwoche für ihren Träger ausrichtet, wie viele Kinder kommen und in
 * welcher Zeit betreut wird (ADR 0116, 0120).
 *
 * Betreut wird von Montag bis Freitag der Woche. Ein Schließtag der Schule
 * ist ein Tag ohne Ferienbetreuung (#4702): an ihm zeigt das Raster nur den
 * Schließtag. In einer tageweise gebuchten Woche zählt jeder Tag seine
 * eigenen Kinder.
 */

import type { HostedHolidayWeek } from "~/lib/holiday-host-api";
import { parseISODate, toISODate } from "~/lib/date-helpers";

/** Ein Tag mit Ferienbetreuung. */
export interface HolidayCareDay {
  readonly weekId: string;
  /** Bestätigte Kinder an diesem Tag. */
  readonly children: number;
  /** Betreuungszeit als „HH:MM“; leer, wenn die Woche keine nennt. */
  readonly startsAt: string;
  readonly endsAt: string;
}

const EMPTY: ReadonlyMap<string, HolidayCareDay> = new Map();

/**
 * Die Tage mit Ferienbetreuung im Fenster [fromISO, toISO], keyed
 * „YYYY-MM-DD“. Wochenenden und Schließtage fallen heraus.
 */
export function holidayCareDays(
  weeks: readonly HostedHolidayWeek[],
  fromISO: string,
  toISO: string,
  closingDays?: ReadonlyMap<string, string>,
): ReadonlyMap<string, HolidayCareDay> {
  if (weeks.length === 0 || fromISO === "" || toISO === "") return EMPTY;
  const days = new Map<string, HolidayCareDay>();
  for (const week of weeks) {
    const first = week.startsOn > fromISO ? week.startsOn : fromISO;
    const last = week.endsOn < toISO ? week.endsOn : toISO;
    if (first > last) continue;
    // Eine Woche hat höchstens sieben Tage; die Grenze schützt vor einem
    // fehlerhaften Zeitraum.
    const cursor = parseISODate(first);
    for (let step = 0; step < 7; step++) {
      const iso = toISODate(cursor);
      if (iso > last) break;
      const weekday = cursor.getDay();
      if (weekday !== 0 && weekday !== 6 && !closingDays?.has(iso)) {
        days.set(iso, {
          weekId: week.id,
          children:
            week.bookingUnit === "day"
              ? (week.dayParticipants.get(iso) ?? 0)
              : week.participants,
          startsAt: week.careStartsAt,
          endsAt: week.careEndsAt,
        });
      }
      cursor.setDate(cursor.getDate() + 1);
    }
  }
  return days;
}

/** „1 Kind“, „3 Kinder“, „0 Kinder“. */
export function holidayCareChildren(children: number): string {
  return children === 1 ? "1 Kind" : `${children} Kinder`;
}

/** „08:00–16:00“, leer ohne Zeit. */
export function holidayCareWindow(day: HolidayCareDay): string {
  return day.startsAt && day.endsAt ? `${day.startsAt}–${day.endsAt}` : "";
}

/** Sichtbare Beschriftung: „Ferienbetreuung · 3 Kinder · 08:00–16:00“. */
export function holidayCareLabel(
  day: HolidayCareDay,
  { withWindow = true }: { readonly withWindow?: boolean } = {},
): string {
  const window = withWindow ? holidayCareWindow(day) : "";
  return ["Ferienbetreuung", holidayCareChildren(day.children), window]
    .filter(Boolean)
    .join(" · ");
}

/**
 * Der ganze Satz für Tooltip und Vorlesen: „Ferienbetreuung für den Träger:
 * 3 Kinder, 08:00 bis 16:00 Uhr.“
 */
export function holidayCareSentence(day: HolidayCareDay): string {
  const children =
    day.children === 0
      ? "noch kein Kind bestätigt"
      : holidayCareChildren(day.children);
  const time =
    day.startsAt && day.endsAt ? `, ${day.startsAt} bis ${day.endsAt} Uhr` : "";
  return `Ferienbetreuung für den Träger: ${children}${time}.`;
}
