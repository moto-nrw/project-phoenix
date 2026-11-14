"use client";

// Wie ein Ferienzeitraum seine Plätze vergibt (#4700): nach Eingang wie
// bisher (#4699) oder nach Kriterien. Nach Kriterien bekommt beim Absenden
// niemand automatisch einen Platz; moto sortiert vor, das Büro entscheidet.
// Sobald es eine Anmeldung gibt, bleibt das Verfahren so.

import { ListChecks } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Radio } from "~/components/ui/radio";
import { SectionLabel } from "~/components/ui/section-label";
import type { HolidayCriterion } from "~/lib/traeger-holiday-allocation-api";
import type { HolidayAllocationMode } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";

const MODES: readonly {
  readonly value: HolidayAllocationMode;
  readonly label: string;
  readonly text: string;
}[] = [
  {
    value: "arrival",
    label: "Nach Eingang",
    text: "Wer sich zuerst anmeldet, bekommt zuerst einen Platz. Ist eine Woche voll, kommt das Kind auf die Warteliste.",
  },
  {
    value: "criteria",
    label: "Nach Kriterien",
    text: "Eltern beantworten Ihre Fragen. moto sortiert die Anmeldungen vor. Das Büro entscheidet, wer einen Platz bekommt.",
  },
];

/** The active criteria as the line under the choice names them. */
function activeCriteriaText(criteria: readonly HolidayCriterion[]): string {
  const active = criteria.filter((criterion) => criterion.active);
  const names = active
    .map((criterion, index) => `${index + 1}. ${criterion.name}`)
    .join(", ");
  return active.length === 1
    ? `1 Kriterium ist eingeschaltet: ${names}.`
    : `${active.length} Kriterien sind eingeschaltet: ${names}.`;
}

export function HolidayAllocationModeField({
  value,
  locked,
  criteria,
  onChange,
}: Readonly<{
  value: HolidayAllocationMode;
  /** The period has a registration; the choice stays as it is. */
  locked: boolean;
  /** The Träger's criteria; undefined while they load or cannot be read. */
  criteria?: readonly HolidayCriterion[];
  onChange: (mode: HolidayAllocationMode) => void;
}>) {
  const activeCount =
    criteria?.filter((criterion) => criterion.active).length ?? 0;
  return (
    <div className="space-y-4">
      <fieldset
        className="min-w-0 space-y-3"
        disabled={locked}
        aria-describedby={locked ? "allocation-mode-locked" : undefined}
      >
        <SectionLabel as="legend" className="mb-1">
          Wie werden die Plätze vergeben?
        </SectionLabel>
        {locked ? (
          <p id="allocation-mode-locked" className="text-sm text-gray-600">
            Es gibt schon Anmeldungen. Darum bleibt die Platzvergabe so.
          </p>
        ) : null}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {MODES.map((mode) => (
            <ChoiceTile
              key={mode.value}
              selected={value === mode.value}
              disabled={locked && value !== mode.value}
              tone="green"
              className="items-start p-4"
            >
              <Radio
                name="allocation_mode"
                value={mode.value}
                checked={value === mode.value}
                onChange={() => onChange(mode.value)}
                className="mt-0.5"
              />
              <span className="flex min-w-0 flex-col gap-1">
                <span className="text-sm font-semibold text-gray-900">
                  {mode.label}
                </span>
                <span className="text-sm font-normal text-gray-600">
                  {mode.text}
                </span>
              </span>
            </ChoiceTile>
          ))}
        </div>
      </fieldset>
      {value === "criteria" ? (
        <div className="space-y-3">
          {criteria && activeCount === 0 ? (
            <Alert
              type="warning"
              message="Es ist noch kein Kriterium eingeschaltet. Dann sortiert moto nur nach Eingang. Bitte legen Sie zuerst die Kriterien fest."
            />
          ) : null}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-gray-700">
              {criteria && activeCount > 0
                ? activeCriteriaText(criteria)
                : "Die Kriterien gelten für alle Ferien Ihres Trägers."}{" "}
              moto schlägt nur vor. Jede Entscheidung trifft Ihr Büro.
            </p>
            <ButtonLink
              href={traegerPath("/traeger/ferien/kriterien")}
              variant="outline"
              size="md"
              className="shrink-0 gap-2"
              tooltip="Öffnet die Kriterien. Sie gelten für alle Ferien Ihres Trägers."
            >
              <ListChecks className="h-4 w-4" aria-hidden="true" />
              Kriterien festlegen
            </ButtonLink>
          </div>
        </div>
      ) : null}
    </div>
  );
}
