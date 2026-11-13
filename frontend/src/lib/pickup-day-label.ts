import { formatStatusDate, parseISODate, toISODate } from "~/lib/date-helpers";

/** „Heute“, „Morgen“ or the weekday and date of a calendar day (#6855). */
export function pickupDayLabel(date: string, today: string): string {
  if (date === today) return "Heute";
  const tomorrow = parseISODate(today);
  tomorrow.setDate(tomorrow.getDate() + 1);
  if (date === toISODate(tomorrow)) return "Morgen";
  return formatStatusDate(date);
}
