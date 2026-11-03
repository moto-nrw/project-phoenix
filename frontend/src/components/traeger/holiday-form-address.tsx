"use client";

// Fester Baustein „Anschrift“ des Anmeldeformulars eines Ferienzeitraums
// (#5625): Straße, PLZ und Ort der Person, die anmeldet. Er ist keine Frage
// des Baukastens, weil Rechnung und Bestätigungsschreiben die Anschrift als
// feste Angabe übernehmen.

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
    <section
      aria-labelledby="holiday-form-address-title"
      className="moto-content-surface rounded-2xl border p-5"
    >
      <h2
        id="holiday-form-address-title"
        className="text-base font-semibold text-gray-900"
      >
        Anschrift
      </h2>
      <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
        Eltern geben Straße, PLZ und Ort an. Rechnung und Bestätigungsschreiben
        übernehmen die Anschrift.
      </p>
      <SegmentedControl
        className="mt-4"
        ariaLabel="Anschrift abfragen"
        items={ADDRESS_CHOICES.map((item) => ({ ...item, disabled }))}
        value={value}
        onChange={onChange}
      />
    </section>
  );
}
