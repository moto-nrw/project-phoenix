"use client";

// Ferienbeitrag einer Anmeldung (#4664): je Kind die Positionen der
// Regelkette, vor der Bestätigung als Vorschau, danach festgehalten. Die
// Karte zeigt nur; bestätigt wird in der Karte „Status“. Hat eine schon
// ausgestellte Rechnung einen anderen Betrag als der Beitrag, etwa nach dem
// Storno eines Geschwisters, sagt sie, wie das Büro das korrigiert (#5662).

import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { HolidayContribution } from "~/lib/traeger-contributions-api";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import { contributionLineLabel, formatEuro } from "./contribution-format";
import { ContributionRulesLink } from "./contribution-rules-link";

function description(
  contribution: HolidayContribution,
  registration: HolidayRegistrationDetail,
): string {
  if (contribution.frozen && contribution.confirmedAt) {
    return `Festgehalten am ${formatBerlinDate(contribution.confirmedAt)}. Spätere Preise und Regeln ändern ihn nicht.`;
  }
  if (registration.status === "received") {
    return "Vorschau mit den Preisen und Regeln von heute. „Bestätigen“ hält den Beitrag fest.";
  }
  return "Für diese Anmeldung wurde kein Beitrag festgehalten. Das ist eine Vorschau mit den Preisen und Regeln von heute.";
}

export function HolidayContributionCard({
  contribution,
  registration,
  canManageBillingSettings = false,
}: Readonly<{
  contribution: HolidayContribution;
  registration: HolidayRegistrationDetail;
  /** Das Recht „Abrechnung“ öffnet die Beitragsregeln. */
  canManageBillingSettings?: boolean;
}>) {
  const childName = (childId: string, position: number) => {
    const child =
      registration.children.find((candidate) => candidate.id === childId) ??
      registration.children[position - 1];
    return child ? `${child.firstName} ${child.lastName}` : `Kind ${position}`;
  };
  return (
    <SectionCard
      title="Ferienbeitrag"
      description={description(contribution, registration)}
    >
      {contribution.correctionNeeded ? (
        <Alert
          type="warning"
          className="mb-4"
          title="Rechnung prüfen"
          message="Der Beitrag hat sich geändert, zum Beispiel durch ein Storno bei den Geschwistern. Die Rechnung zeigt noch den alten Betrag. Stornieren Sie die Rechnung. Der nächste Abrechnungslauf stellt dann den neuen Betrag aus."
        />
      ) : null}
      <ContributionBreakdown
        items={contribution.children.map((child) => ({
          key: child.childId || String(child.position),
          name: childName(child.childId, child.position),
          lines: child.lines.map((line) => ({
            label: contributionLineLabel(line),
            amount: formatEuro(line.amountCents),
          })),
          total: formatEuro(child.totalCents),
        }))}
        childTotalLabel="Summe"
        totalLabel="Gesamt"
        total={formatEuro(contribution.totalCents)}
      />
      <ContributionRulesLink
        canManageBillingSettings={canManageBillingSettings}
        kinds={contribution.children.flatMap((child) =>
          child.lines.map((line) => line.kind),
        )}
      />
    </SectionCard>
  );
}
