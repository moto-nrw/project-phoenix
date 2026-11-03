"use client";

// Fester Baustein „Anschrift“ des Anmeldeformulars eines Ferienzeitraums
// (#5625): Straße, PLZ und Ort der Person, die anmeldet. Er ist keine Frage
// des Baukastens, weil Rechnung und Bestätigungsschreiben die Anschrift als
// feste Angabe übernehmen.

import { SectionCard } from "~/components/ui/section-card";
import {
  SegmentedControl,
  type SegmentedControlItem,
} from "~/components/ui/segmented-control";
import type { HolidayAddressRequirement } from "~/lib/traeger-holiday-form-api";

const ADDRESS_CHOICES: readonly SegmentedControlItem<HolidayAddressRequirement>[] =
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

export function HolidayFormAddress({
  value,
  onChange,
  disabled,
}: Readonly<{
  value: HolidayAddressRequirement;
  onChange: (value: HolidayAddressRequirement) => void;
  disabled: boolean;
}>) {
  return (
    <SectionCard
      title="Anschrift"
      headingLevel={3}
      namedRegion
      description="Eltern geben Straße, PLZ und Ort an. Rechnung und Bestätigungsschreiben übernehmen die Anschrift."
    >
      {/* Unter sm untereinander: drei Segmente nebeneinander sind breiter
          als ein 320-px-Telefon abzüglich der beiden Kartenränder. */}
      <SegmentedControl
        className="max-sm:grid max-sm:w-full max-sm:grid-flow-row"
        ariaLabel="Anschrift abfragen"
        items={ADDRESS_CHOICES.map((item) => ({ ...item, disabled }))}
        value={value}
        onChange={onChange}
      />
    </SectionCard>
  );
}
