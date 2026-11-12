// Auswahl und Vorschau des festen Bausteins „Gesundheit“ des Anmeldeformulars
// eines Ferienzeitraums (#6686): Allergien und Medikamente jedes Kindes sind
// freiwillig oder Pflicht. Bei Pflicht antworten Eltern mit „Nein, keine“
// oder schreiben auf, was ihr Kind hat. Weitere Hinweise bleiben freiwillig.

import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import type { HolidayHealthRequirement } from "~/lib/traeger-holiday-form-api";

export const HOLIDAY_HEALTH_CHOICES: readonly SegmentedControlItem<HolidayHealthRequirement>[] =
  [
    { value: "optional", label: "Freiwillig" },
    { value: "required", label: "Pflicht" },
  ];

/** The fixed health fields the preview for parents lists under „Kind“. */
export function holidayHealthPreviewFields(
  requirement: HolidayHealthRequirement,
): string[] {
  return requirement === "required"
    ? [
        "Allergien: Nein, keine / Ja *",
        "Medikamente: Nein, keine / Ja *",
        "Weitere Hinweise zur Gesundheit",
      ]
    : ["Allergien", "Medikamente", "Weitere Hinweise zur Gesundheit"];
}
