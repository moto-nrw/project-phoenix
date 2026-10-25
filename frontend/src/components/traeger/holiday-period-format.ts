/** Display helpers of the holiday periods (#4659). */

import { formatDate } from "~/lib/date-helpers";
import type { HolidayPeriod } from "~/lib/traeger-api";

/** "02.11.2026 bis 04.12.2026". */
export function formatDateRange(from: string, to: string): string {
  return `${formatDate(from)} bis ${formatDate(to)}`;
}

/** "2 Wochen: 04.01.2027 bis 15.01.2027", or "Noch keine Woche". */
export function formatWeeks(period: HolidayPeriod): string {
  const weeks = period.weeks;
  const first = weeks[0];
  const last = weeks.at(-1);
  if (!first || !last) return "Noch keine Woche";
  const count = weeks.length === 1 ? "1 Woche" : `${weeks.length} Wochen`;
  return `${count}: ${formatDateRange(first.startsOn, last.endsOn)}`;
}

export function holidayStatusLabel(period: HolidayPeriod): string {
  return period.status === "published" ? "Veröffentlicht" : "Entwurf";
}
