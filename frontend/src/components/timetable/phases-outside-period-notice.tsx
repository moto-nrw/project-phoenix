"use client";

import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { formatDate } from "~/lib/date-helpers";
import type { OfferingPhaseOutsidePeriod } from "~/lib/offering-gap";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** Where the left-out offerings are missing: the checks or the choice. */
export type PhasesOutsidePeriodContext = "checks" | "choice";

function range(start: string, end: string): string {
  return `${formatDate(start)} bis ${formatDate(end)}`;
}

function offerings(count: number): string {
  return count === 1 ? "ihr Angebot" : `ihre ${count} Angebote`;
}

/**
 * Names the enrollment phases a planning period leaves out because they reach
 * beyond it (#5344). Their offerings are neither checked against the
 * Regeltermine nor offered as a source, so without this notice an empty list
 * would read as „alles in Ordnung“. The notice names the phase, both date
 * ranges and the way out; only who may manage Anmeldephasen gets the link.
 */
export function PhasesOutsidePeriodNotice({
  phases,
  context,
  canManagePhases,
}: Readonly<{
  phases: readonly OfferingPhaseOutsidePeriod[];
  context: PhasesOutsidePeriodContext;
  canManagePhases: boolean;
}>) {
  const tenantPath = useTenantAwarePath();
  const [first] = phases;
  if (!first) return null;

  const single = phases.length === 1;
  const total = phases.reduce((sum, phase) => sum + phase.offeringCount, 0);
  const period = range(first.periodStart, first.periodEnd);
  const theirOfferings = offerings(total);
  const missing =
    context === "checks"
      ? `Für ${theirOfferings} zeigt moto hier deshalb keine Hinweise.`
      : `${theirOfferings.charAt(0).toUpperCase()}${theirOfferings.slice(1)} ${total === 1 ? "steht" : "stehen"} hier deshalb nicht zur Auswahl.`;
  const question = single
    ? "Gehört die Phase zu diesem Zeitraum?"
    : "Gehört eine Phase zu diesem Zeitraum?";
  const fix = canManagePhases
    ? `${question} Dann gleichen Sie die Daten an.`
    : `${question} Dann muss jemand mit Zugriff auf die Anmeldephasen die Daten angleichen.`;
  const message = single
    ? `Die Anmeldephase läuft vom ${range(first.serviceStart, first.serviceEnd)}, der Planungszeitraum vom ${period}. ${missing} ${fix}`
    : `Sie laufen länger als dieser Planungszeitraum (${period}). ${missing} ${fix}`;

  return (
    <Alert
      type="warning"
      announce="off"
      data-testid="phases-outside-period-notice"
      title={
        single
          ? `„${first.phaseName}“ wird hier nicht geprüft`
          : `${phases.length} Anmeldephasen werden hier nicht geprüft`
      }
      message={message}
      details={
        single ? undefined : (
          <ul className="mt-1 list-disc pl-5">
            {phases.map((phase) => (
              <li key={phase.phaseId}>
                „{phase.phaseName}“:{" "}
                {range(phase.serviceStart, phase.serviceEnd)}
              </li>
            ))}
          </ul>
        )
      }
      action={
        canManagePhases ? (
          <ButtonLink
            href={tenantPath("/enrollment-phases")}
            variant="surface"
            size="compact"
          >
            Anmeldephasen öffnen
          </ButtonLink>
        ) : undefined
      }
    />
  );
}
