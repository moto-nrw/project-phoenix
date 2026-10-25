"use client";

// Ferienbeitrag einer Anmeldung (#4664): je Kind die Positionen der
// Regelkette, vor der Bestätigung als Vorschau, danach festgehalten. Die
// Karte zeigt nur; bestätigt wird in der Karte „Status“.

import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { HolidayContribution } from "~/lib/traeger-contributions-api";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";
import { contributionLineLabel, formatEuro } from "./contribution-format";

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
}: Readonly<{
  contribution: HolidayContribution;
  registration: HolidayRegistrationDetail;
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
    </SectionCard>
  );
}
