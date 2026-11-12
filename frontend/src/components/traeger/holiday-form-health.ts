// Auswahl und Vorschau des festen Bausteins „Gesundheit“ des Anmeldeformulars
// eines Ferienzeitraums (#6686): Allergien und Medikamente jedes Kindes sind
// freiwillig oder Pflicht. Bei Pflicht antworten Eltern mit „Nein, keine“
// oder schreiben auf, was ihr Kind hat. Weitere Hinweise bleiben freiwillig.

import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import type {
  HolidayFormField,
  HolidayHealthRequirement,
} from "~/lib/traeger-holiday-form-api";

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

/** Words of a question that asks what the fixed health fields ask. */
const REPEATS_HEALTH_FIELD =
  /allergi|unverträglich|intoleranz|medikament|arznei/i;

/**
 * Own questions that repeat the fixed fields allergies and medication
 * (#6686). Before the building block a Träger made them required with an own
 * question; with „Pflicht“ parents would answer twice. Information blocks ask
 * nothing and stay out.
 */
export function questionsRepeatingHealthFields(
  fields: readonly HolidayFormField[],
): HolidayFormField[] {
  return fields.filter(
    (field) =>
      field.type !== "information" && REPEATS_HEALTH_FIELD.test(field.label),
  );
}
