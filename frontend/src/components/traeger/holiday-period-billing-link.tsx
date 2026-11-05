"use client";

// „Zur Abrechnung“ im Kopf eines Ferienzeitraums (#5688). Gibt es für den
// Zeitraum genau einen Abrechnungslauf, öffnet die Aktion ihn; sonst die
// Abrechnung, gefiltert auf diesen Zeitraum. Nur mit dem Recht
// „Abrechnung“ und nur für einen veröffentlichten Zeitraum: Ein Entwurf hat
// keine Läufe und lässt sich nicht abrechnen.

import { useSession } from "next-auth/react";
import { ButtonLink } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { hasPermission } from "~/lib/auth-utils";
import { billingConcept } from "~/lib/concepts/billing";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import type { HolidayPeriod } from "~/lib/traeger-api";
import { type BillingRun, fetchBillingRuns } from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

/** Das Recht, das die Abrechnungsläufe öffnet (#4668). */
export const TRAEGER_BILLING_RIGHT = "traeger_billing:manage";

/** URL-Parameter der Abrechnung, der die Liste auf einen Zeitraum filtert. */
export const BILLING_PERIOD_PARAM = "ferienzeitraum";

/** Die Abrechnung, gefiltert auf einen Ferienzeitraum (Pfad mit /traeger). */
export function billingListPathForPeriod(periodId: string): string {
  return `/traeger/abrechnung?${BILLING_PERIOD_PARAM}=${encodeURIComponent(periodId)}`;
}

/**
 * Wohin „Zur Abrechnung“ führt: genau ein Lauf des Zeitraums öffnet seine
 * Seite, sonst (kein Lauf, mehrere, noch nicht geladen) die gefilterte Liste.
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

export function HolidayPeriodBillingLink({
  period,
}: Readonly<{ period: HolidayPeriod | undefined }>) {
  const { data: session } = useSession();
  const allowed =
    hasPermission(session ?? null, TRAEGER_BILLING_RIGHT) &&
    period?.status === "published";
  // Ein Fehler beim Laden der Läufe zeigt nichts an: Die Aktion führt dann
  // zur gefilterten Liste, und die meldet ihren eigenen Ladefehler.
  const { data: runs } = useSWRAuth(
    allowed ? traegerKeys.billingRuns() : null,
    fetchBillingRuns,
  );
  if (!allowed || !period) return null;
  return (
    <ButtonLink
      href={traegerPath(billingPathForPeriod(period.id, runs))}
      variant="outline"
      size="md"
    >
      <MotoConceptIcon concept={billingConcept} size={18} className="mr-2" />
      Zur Abrechnung
    </ButtonLink>
  );
}
