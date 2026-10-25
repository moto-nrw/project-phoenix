// The periods of the caterer overview (#3873): tomorrow, this week and next
// week, Monday to Friday.

import { parseISODate, toISODate } from "./date-helpers";

export type Period = "tomorrow" | "this_week" | "next_week";

export const PERIODS: { value: Period; label: string }[] = [
  { value: "tomorrow", label: "Morgen" },
  { value: "this_week", label: "Diese Woche" },
  { value: "next_week", label: "Nächste Woche" },
];

function addDays(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

function weekday(iso: string): number {
  return parseISODate(iso).getDay();
}

function mondayOfWeek(iso: string): string {
  return addDays(iso, -((weekday(iso) + 6) % 7));
}

/** The days a period covers, Monday to Friday only. */
export function periodDays(period: Period, today: string): string[] {
  if (period === "tomorrow") {
    let day = addDays(today, 1);
    while (weekday(day) === 0 || weekday(day) === 6) day = addDays(day, 1);
    return [day];
  }
  const monday = addDays(mondayOfWeek(today), period === "next_week" ? 7 : 0);
  return [0, 1, 2, 3, 4].map((offset) => addDays(monday, offset));
}

/** From Friday on, the next order a kitchen plans for lies next week. */
export function defaultPeriod(today: string): Period {
  const day = weekday(today);
  return day === 5 || day === 6 || day === 0 ? "next_week" : "this_week";
}
