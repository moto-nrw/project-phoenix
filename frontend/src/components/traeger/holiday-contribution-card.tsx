"use client";

// Ferienbeitrag einer Anmeldung (#4664): je Kind die Positionen der
// Regelkette, vor der Bestätigung als Vorschau, danach festgehalten. Die
// Karte zeigt nur; bestätigt wird in der Karte „Status“.

import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
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
      <div className="space-y-6">
        {contribution.children.map((child) => (
          <div key={child.childId || child.position} className="space-y-2">
            <p className="text-sm font-semibold text-gray-900">
              {childName(child.childId, child.position)}
            </p>
            <DataGrid>
              {child.lines.map((line, index) => (
                <DataField
                  key={`${line.kind}-${index}`}
                  label={contributionLineLabel(line)}
                >
                  <span className="tabular-nums">
                    {formatEuro(line.amountCents)}
                  </span>
                </DataField>
              ))}
              <DataField label="Summe">
                <span className="font-semibold tabular-nums">
                  {formatEuro(child.totalCents)}
                </span>
              </DataField>
            </DataGrid>
          </div>
        ))}
        {contribution.children.length > 1 ? (
          <p className="text-sm font-semibold text-gray-900">
            Gesamt: {formatEuro(contribution.totalCents)}
          </p>
        ) : null}
      </div>
    </SectionCard>
  );
}
