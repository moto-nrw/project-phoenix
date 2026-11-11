/**
 * Hinweis an einer Gruppe, die als Feriengruppe einer Woche gewählt ist, die
 * noch nicht begonnen hat (#6680). Feriengäste stehen erst ab dem ersten Tag
 * ihrer Woche in der Gruppe; bis dahin wirkt die Gruppe leer. Der Hinweis
 * sagt, wie viele Kinder dazukommen und ab wann.
 */

import {
  fetchHolidayGuestGroupHints,
  type HolidayGuestGroupHint,
} from "~/lib/holiday-host-api";
import { useSWRAuth } from "~/lib/swr";
import { holidayKeys } from "~/lib/swr/portal-keys";

// Die Wahl der Feriengruppe und neue Bestätigungen kommen höchstens mit dem
// stündlichen Abgleich; die Viertelstunde bringt sie auf eine offene Seite.
const HOLIDAY_GUEST_GROUP_HINT_OPTIONS = {
  revalidateOnFocus: false,
  dedupingInterval: 60_000,
  refreshInterval: 15 * 60_000,
  keepPreviousData: true,
} as const;

/**
 * Gibt den Hinweis der Gruppe zurück, bei mehreren kommenden Wochen den der
 * nächsten; `undefined` für eine gewöhnliche Gruppe. Scheitert das Laden
 * (etwa ohne das Recht der Kinderliste), bleibt die Seite wie ohne Hinweis:
 * die Gruppe selbst ist geladen, der Hinweis ergänzt sie nur.
 */
export function useHolidayGuestGroupHint(
  groupId: string | null | undefined,
): HolidayGuestGroupHint | undefined {
  const { data } = useSWRAuth(
    groupId ? holidayKeys.guestGroupHints() : null,
    fetchHolidayGuestGroupHints,
    HOLIDAY_GUEST_GROUP_HINT_OPTIONS,
  );
  if (!groupId || !data) return undefined;
  // Das Backend sortiert nach dem ersten Tag der Woche.
  return data.find((hint) => hint.groupId === groupId);
}
