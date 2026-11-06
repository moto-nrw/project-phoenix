// Texte zu einer offenen Abmeldung. Bewusst getrennt von care-exit-api.ts:
// reine Datumslogik, die Tests nicht mitmocken müssen.

import type { CareWithdrawalCompletion } from "./care-exit-api";
import { formatDate, parseISODate, toISODate } from "./date-helpers";

/** Der letzte Betreuungstag vor dem ersten Tag ohne Buchung. */
export function lastCareDayBefore(firstBookinglessDay: string): string {
  const day = parseISODate(firstBookinglessDay);
  day.setDate(day.getDate() - 1);
  return toISODate(day);
}

/**
 * Was eine offene Abmeldung für die Betreuung heißt. Geplant nennt sie den
 * letzten Betreuungstag, damit „Betreuung beenden“ nicht wie ein sofortiges
 * Ende klingt; überfällig nennt sie den Tag, seit dem nichts mehr gebucht ist.
 */
export function careWithdrawalNotice(
  row: Pick<CareWithdrawalCompletion, "firstBookinglessDay" | "urgency">,
): string {
  if (row.urgency === "planned") {
    return `Letzter Betreuungstag ist der ${formatDate(lastCareDayBefore(row.firstBookinglessDay))}. Bis dahin bleibt alles wie gebucht.`;
  }
  return `Seit dem ${formatDate(row.firstBookinglessDay)} ist kein Betreuungstag mehr gebucht.`;
}
