"use client";

// Wie die Verträge jeder Schule entstehen (#4680): Gilt der Vertrag bei der
// Annahme der Anmeldung, oder verschickt das Träger-Büro ihn? Die Karte zeigt
// den Weg je Schule nur an; geändert wird er in den Einstellungen.

import { useSession } from "next-auth/react";
import {
  CONCLUSION_MODE_LABELS,
  CONCLUSION_MODE_TONES,
} from "~/components/traeger/care-contract-format";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { hasPermission } from "~/lib/auth-utils";
import type { SchoolConclusionMode } from "~/lib/traeger-care-contracts-api";
import { traegerPath } from "~/lib/traeger-url";

const SETTINGS_RIGHT = "traeger_overview:read";

function description(modes: readonly SchoolConclusionMode[]): string {
  const onAcceptance = modes.filter((school) => school.mode === "bei_annahme");
  if (onAcceptance.length === 0) {
    return "Sie erstellen jeden Vertrag hier und schicken ihn den Eltern.";
  }
  if (onAcceptance.length === modes.length) {
    return "Nimmt die Schule eine Anmeldung an, gilt der Vertrag. Die Eltern bekommen ihn per E-Mail und in der Eltern-App, meist nach wenigen Minuten. Sie müssen nichts tun.";
  }
  return "Bei „Gilt bei der Annahme“ entsteht der Vertrag von selbst, sobald die Schule die Anmeldung annimmt. Die anderen Verträge erstellen und verschicken Sie hier.";
}

export function CareContractConclusionCard({
  modes,
}: Readonly<{ modes: readonly SchoolConclusionMode[] }>) {
  const { data: session } = useSession();
  if (modes.length === 0) return null;
  return (
    <SectionCard
      title="Wann gilt der Vertrag?"
      description={description(modes)}
      action={
        hasPermission(session, SETTINGS_RIGHT) ? (
          <ButtonLink
            href={traegerPath("/traeger/einstellungen")}
            variant="surface"
            size="compact"
          >
            In den Einstellungen ändern
          </ButtonLink>
        ) : null
      }
    >
      <ul aria-label="Abschluss je Schule" className="divide-y divide-gray-100">
        {modes.map((school) => (
          <li
            key={school.schoolId}
            className="flex items-center justify-between gap-3 py-2.5 first:pt-0 last:pb-0"
          >
            <span className="min-w-0 text-sm font-medium wrap-anywhere text-gray-900">
              {school.schoolName}
            </span>
            <StatusBadge
              label={CONCLUSION_MODE_LABELS[school.mode]}
              tone={CONCLUSION_MODE_TONES[school.mode]}
            />
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}
