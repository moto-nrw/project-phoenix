"use client";

import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import { findPeriodForDate } from "~/lib/calendar-period-helpers";
import { berlinTodayISO } from "~/lib/date-helpers";
import { useSWRAuth } from "~/lib/swr";
import { databaseKeys, timetableKeys } from "~/lib/swr/portal-keys";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { timetableService } from "~/lib/timetable-api";

/**
 * Aufgabe auf der Startseite (#5102): Kinder, die nach einer
 * Buchungsänderung noch von Hand in einem Regeltermin stehen. Der Link führt
 * in die Serienansicht, wo sie entfernt oder behalten werden. Ohne offene
 * Einträge ist sie nicht da. Kann der Zeitraum oder die Prüfung nicht
 * gelesen werden, fehlt nur dieser Hinweis; der Betreuungsplan zeigt ihn
 * weiter.
 */
export function BookingLeftoverTodos({
  enabled,
}: Readonly<{
  /** Recht am Betreuungsplan und Betreuungsplan eingeschaltet. */
  enabled: boolean;
}>) {
  const tenantPath = useTenantAwarePath();
  const { data: periods } = useSWRAuth(
    enabled ? databaseKeys.calendarPeriods() : null,
    () => calendarPeriodService.list(),
  );
  const period = periods ? findPeriodForDate(periods, berlinTodayISO()) : null;
  // Same cache entry as the Betreuungsplan, so both show the same count.
  const { data: leftovers } = useSWRAuth(
    enabled && period ? timetableKeys.bookingLeftovers(period.id) : null,
    () => timetableService.getBookingLeftovers(period?.id ?? ""),
  );

  const count = leftovers?.filter((leftover) => !leftover.kept).length ?? 0;
  if (!enabled || count === 0) return null;
  return (
    <Alert
      type="warning"
      announce="polite"
      title="Kinder ohne passende Buchung"
      message={
        count === 1
          ? "1 Kind steht noch in einem Regeltermin, obwohl seine Buchung weggefallen ist."
          : `${count} Kinder stehen noch in Regelterminen, obwohl ihre Buchung weggefallen ist.`
      }
      action={
        <ButtonLink
          href={tenantPath("/betreuungsplan?view=serien")}
          variant="surface"
          size="md"
        >
          Kinder prüfen
        </ButtonLink>
      }
    />
  );
}
