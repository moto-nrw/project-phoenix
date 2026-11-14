"use client";

// „Zur Abrechnung“ im Kopf eines Ferienzeitraums (#5688). Gibt es für den
// Zeitraum genau einen Abrechnungslauf, öffnet die Aktion ihn; sonst die
// Abrechnung, gefiltert auf diesen Zeitraum. Nur mit dem Recht
// „Abrechnung“ und nur für einen veröffentlichten Zeitraum: Ein Entwurf hat
// keine Läufe und lässt sich nicht abrechnen.

import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import type { HolidayPeriod } from "~/lib/traeger-api";
import { type BillingRun, fetchBillingRuns } from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

/** Das Recht, das die Abrechnungsläufe öffnet (#4668). */
const TRAEGER_BILLING_RIGHT = "traeger_billing:manage";

/** Was ein Klick auf „Zur Abrechnung“ tut (#6747). */
export const HOLIDAY_BILLING_HINT =
  "Öffnet die Abrechnung dieses Ferienzeitraums.";

/** URL-Parameter der Abrechnung, der die Liste auf einen Zeitraum filtert. */
export const BILLING_PERIOD_PARAM = "ferienzeitraum";

/** Die Abrechnung, gefiltert auf einen Ferienzeitraum (Pfad mit /traeger). */
function billingListPathForPeriod(periodId: string): string {
  return `/traeger/abrechnung?${BILLING_PERIOD_PARAM}=${encodeURIComponent(periodId)}`;
}

/**
 * Wohin „Zur Abrechnung“ führt: genau ein Lauf des Zeitraums öffnet seine
 * Seite, sonst (kein Lauf, mehrere, Läufe nicht lesbar) die gefilterte Liste.
 */
export function billingPathForPeriod(
  periodId: string,
  runs: readonly BillingRun[] | undefined,
): string {
  const own = runs?.filter((run) => run.holidayPeriodId === periodId) ?? [];
  const only = own.length === 1 ? own[0] : undefined;
  return only
    ? `/traeger/abrechnung/${encodeURIComponent(only.id)}`
    : billingListPathForPeriod(periodId);
}

export function useHolidayPeriodBillingHref(
  period: HolidayPeriod | undefined,
): string | null {
  const { data: session } = useSession();
  const allowed =
    hasPermission(session ?? null, TRAEGER_BILLING_RIGHT) &&
    period?.status === "published";
  // Ein Fehler beim Laden der Läufe zeigt nichts an: Die Aktion führt dann
  // zur gefilterten Liste, und die meldet ihren eigenen Ladefehler.
  const { data: runs, error } = useSWRAuth(
    allowed ? traegerKeys.billingRuns() : null,
    fetchBillingRuns,
  );
  // Erst mit den Läufen steht das Ziel fest; ein Klick davor führte an einem
  // einzigen Lauf vorbei zur Liste.
  if (!allowed || !period || (runs === undefined && !error)) return null;
  return traegerPath(billingPathForPeriod(period.id, runs));
}
