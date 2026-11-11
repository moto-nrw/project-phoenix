/**
 * Booking of single days in the holidays (#4701): the care days of a week,
 * German day labels and the placement rule the backend applies, so a screen
 * can say beforehand what a change does. Dates are "YYYY-MM-DD" strings.
 */

import { parseISODate, toISODate } from "~/lib/date-helpers";

const WEEKDAY_SHORT = ["So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"] as const;
const WEEKDAY_LONG = [
  "Sonntag",
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
] as const;

/** "Mo" for a Monday; another locale names it in its own language. */
export function holidayWeekdayShort(iso: string, locale = "de"): string {
  if (locale.startsWith("de")) {
    return WEEKDAY_SHORT[parseISODate(iso).getDay()] ?? "";
  }
  return new Intl.DateTimeFormat(locale, { weekday: "short" }).format(
    parseISODate(iso),
  );
}

/** "Montag" for a Monday. */
export function holidayWeekdayLong(iso: string): string {
  return WEEKDAY_LONG[parseISODate(iso).getDay()] ?? "";
}

/** "04.01." for 4 January. */
export function holidayDayDate(iso: string): string {
  const [, month = "", day = ""] = iso.split("-");
  return `${day}.${month}.`;
}

/** "Mo 04.01." for Monday 4 January. */
export function holidayDayLabel(iso: string, locale = "de"): string {
  return `${holidayWeekdayShort(iso, locale)} ${holidayDayDate(iso)}`;
}

/** "Mo 04.01., Mi 06.01." for the booked days. */
export function holidayDayList(days: readonly string[], locale = "de"): string {
  return [...days]
    .sort()
    .map((day) => holidayDayLabel(day, locale))
    .join(", ");
}

/** "1 Tag", "3 Tage". */
export function holidayDayCount(count: number): string {
  return count === 1 ? "1 Tag" : `${count} Tage`;
}

/** The care days of a week: its days from Monday to Friday. */
export function holidayCareDays(startsOn: string, endsOn: string): string[] {
  const days: string[] = [];
  if (!startsOn || !endsOn || endsOn < startsOn) return days;
  const day = parseISODate(startsOn);
  for (let i = 0; i < 7; i += 1) {
    const iso = toISODate(day);
    if (iso > endsOn) break;
    const weekday = day.getDay();
    if (weekday !== 0 && weekday !== 6) days.push(iso);
    day.setDate(day.getDate() + 1);
  }
  return days;
}

/** The weekdays parents can book, excluding holidays and closing days. */
export function bookableHolidayDays(
  startsOn: string,
  endsOn: string,
  noCareDays: readonly { readonly date: string }[],
): string[] {
  return holidayCareDays(startsOn, endsOn).filter(
    (day) => !noCareDays.some((none) => none.date === day),
  );
}

/**
 * How many waiting bookings get a place at a capacity, by the backend's rule
 * (holidayFirstFit): in the order of the waiting list a booking gets its
 * place when every one of its days has a free place, otherwise it keeps
 * waiting and a later booking of other days may pass it.
 */
export function holidayDayPromotions(
  capacity: number | null,
  placedOn: ReadonlyMap<string, number>,
  waiting: readonly (readonly string[])[],
): number {
  if (capacity === null) return waiting.length;
  const taken = new Map(placedOn);
  let moved = 0;
  for (const days of waiting) {
    if (days.length === 0) continue;
    if (days.some((day) => (taken.get(day) ?? 0) >= capacity)) continue;
    for (const day of days) taken.set(day, (taken.get(day) ?? 0) + 1);
    moved += 1;
  }
  return moved;
}
