"use client";

// Familien, deren Kinderzahl die Abrechnung prüfen muss (#5918): neue oder
// geänderte Angaben der Eltern und Bestätigungen, die ablaufen. Eine
// neutrale Karte, eine Zeile je Familie, eine Aktion in einer Spalte.

import { ChevronDown, ChevronUp, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import type { FamilyChildren } from "~/lib/traeger-family-children-api";
import {
  checkReason,
  familyName,
  STATUS_LABELS,
  STATUS_TONES,
} from "./family-children-format";

/** Rows shown before „N weitere anzeigen“. */
const INITIAL_ROWS = 5;

export function FamilyChildrenChecks({
  families,
  onCheck,
}: Readonly<{
  families: readonly FamilyChildren[];
  onCheck: (family: FamilyChildren) => void;
}>) {
  const [expanded, setExpanded] = useState(false);
  if (families.length === 0) return null;
  const shown = expanded ? families : families.slice(0, INITIAL_ROWS);
  const hidden = families.length - INITIAL_ROWS;
  return (
    <SectionCard
      title="Zu prüfen"
      titleBadge={
        <StatusBadge tone="orange" label={`${families.length} offen`} />
      }
      description="Bis Sie prüfen, zählt der Monatslauf die bisher bestätigte Zahl. Ohne sie zählen nur die Kinder mit Vertrag."
      leading={
        <TriangleAlert
          className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
          aria-hidden="true"
        />
      }
    >
      <ResponsiveActionList>
        {shown.map((family) => (
          <li
            key={family.recipientEmail}
            className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
          >
            <div className="min-w-0">
              <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-gray-900">
                {familyName(family)}
                <StatusBadge
                  tone={STATUS_TONES[family.status]}
                  label={STATUS_LABELS[family.status]}
                />
              </p>
              <p className="mt-0.5 text-sm text-gray-600">
                {checkReason(family)}
              </p>
            </div>
            <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
              <Button
                type="button"
                variant="surface"
                size="compact"
                aria-label={`Kinderzahl von ${familyName(family)} prüfen`}
                onClick={() => onCheck(family)}
              >
                Prüfen
              </Button>
            </div>
          </li>
        ))}
      </ResponsiveActionList>
      {hidden > 0 ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          className="mt-3"
          aria-expanded={expanded}
          onClick={() => setExpanded((open) => !open)}
        >
          {expanded ? (
            <>
              <ChevronUp className="size-4" aria-hidden="true" />
              Weniger anzeigen
            </>
          ) : (
            <>
              <ChevronDown className="size-4" aria-hidden="true" />
              {hidden} weitere anzeigen
            </>
          )}
        </Button>
      ) : null}
    </SectionCard>
  );
}
