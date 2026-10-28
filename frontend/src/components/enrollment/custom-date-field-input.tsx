"use client";

// Eine Datumsfrage des Formular-Baukastens (#5198). Eigene Datei, weil der
// Kalender mit seinen Sprachen groß ist: Die Ferienanmeldung lädt ihn erst,
// wenn das Formular eine Datumsfrage hat.

import { ISODatePicker } from "~/components/ui/date-picker";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import {
  CustomFieldLabel,
  type CustomFieldInputProps,
} from "./custom-field-input";

export function CustomDateFieldInput({
  field,
  value,
  onChange,
  error,
  controlId,
}: Pick<
  CustomFieldInputProps,
  "field" | "value" | "onChange" | "error" | "controlId"
>) {
  const datePicker = useLocalizedDatePicker();
  const dateId = controlId ?? `enrollment-${field.key}`;
  return (
    <div className="block">
      <label htmlFor={dateId}>
        <CustomFieldLabel field={field} />
      </label>
      <ISODatePicker
        {...datePicker}
        id={dateId}
        controlSize="md"
        value={typeof value === "string" ? value : ""}
        onChange={onChange}
        invalid={Boolean(error)}
        className="mt-1"
        calendarLayout="popover"
        // A form-defined date field can be anything from a birthday to a
        // future appointment, so the year dropdown stays available and no
        // bound is imposed.
        monthYearNavigation
      />
      {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
    </div>
  );
}
