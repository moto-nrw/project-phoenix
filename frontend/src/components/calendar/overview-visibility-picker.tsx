"use client";

import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Radio } from "~/components/ui/radio";
import { VisibilityBadge } from "~/components/ui/visibility-badge";
import type { CalendarOverviewVisibility } from "~/lib/personal-calendar-api";

const OPTIONS: ReadonlyArray<{
  value: CalendarOverviewVisibility;
  label: string;
  description: string;
}> = [
  {
    value: "organizer",
    label: "Nur ich",
    description: "Nur Sie sehen, wer zugesagt oder abgesagt hat.",
  },
  {
    value: "staff",
    label: "Eingeladene Mitarbeitende",
    description: "Eingeladene Mitarbeitende sehen alle Namen und Antworten.",
  },
  {
    value: "all",
    label: "Alle Eingeladenen",
    description:
      "Mitarbeitende sehen alle Namen. Eltern sehen nur, wie viele zugesagt und abgesagt haben.",
  },
];

/**
 * Wer die Teilnehmerübersicht eines Termins sieht. Eltern sehen nie Namen
 * oder Antworten anderer Eltern, auch nicht bei „Alle Eingeladenen“ (#6846);
 * jede Option sagt deshalb in einem Satz, was wer sieht.
 */
export function OverviewVisibilityPicker({
  value,
  onChange,
  disabled = false,
}: Readonly<{
  value: CalendarOverviewVisibility;
  onChange: (next: CalendarOverviewVisibility) => void;
  disabled?: boolean;
}>) {
  return (
    <FormFieldset legend="Wer sieht die Teilnehmerübersicht?">
      <div className="grid gap-2 lg:grid-cols-3">
        {OPTIONS.map((option) => {
          const id = `calendar-overview-visibility-${option.value}`;
          const selected = value === option.value;
          return (
            <ChoiceTile
              key={option.value}
              htmlFor={id}
              selected={selected}
              disabled={disabled}
              className="items-start py-3"
            >
              <Radio
                id={id}
                name="calendar-overview-visibility"
                value={option.value}
                checked={selected}
                disabled={disabled}
                onChange={() => onChange(option.value)}
              />
              <span className="min-w-0">
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="text-sm font-semibold">{option.label}</span>
                  {option.value === "all" ? (
                    <VisibilityBadge
                      audience="parents"
                      label="Eltern sehen Zahlen"
                      hint="Eltern sehen nur die Zahl der Zusagen und Absagen, keine Namen."
                    />
                  ) : null}
                </span>
                <span className="mt-1 block text-xs leading-5 font-normal text-gray-500">
                  {option.description}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </div>
    </FormFieldset>
  );
}
