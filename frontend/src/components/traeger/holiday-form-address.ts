// Auswahl und Vorschau des festen Bausteins „Anschrift“ des Anmeldeformulars eines Ferienzeitraums
// (#5625): Straße, PLZ und Ort der Person, die anmeldet. Er ist keine Frage
// des Baukastens, weil Rechnung und Bestätigungsschreiben die Anschrift als
// feste Angabe übernehmen.

import type { SegmentedControlItem } from "~/components/ui/segmented-control";
import type { HolidayAddressRequirement } from "~/lib/traeger-holiday-form-api";

export const HOLIDAY_ADDRESS_CHOICES: readonly SegmentedControlItem<HolidayAddressRequirement>[] =
  [
    { value: "off", label: "Nicht abfragen" },
    { value: "optional", label: "Freiwillig" },
    { value: "required", label: "Pflicht" },
  ];

/** The fields the preview for parents lists under „Anschrift“. */
export function holidayAddressPreviewFields(
  requirement: HolidayAddressRequirement,
): string[] {
  const mark = requirement === "required" ? " *" : "";
  return [`Straße und Hausnummer${mark}`, `PLZ${mark}`, `Ort${mark}`];
}
