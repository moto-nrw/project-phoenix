"use client";

import { ISODatePicker } from "~/components/ui/date-picker";

interface DateFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
  /** Label shown when the value is empty (e.g., "Kein Datum"). */
  readonly emptyLabel?: string;
}

/**
 * Kit date picker for FieldDate settings. It round-trips ISO YYYY-MM-DD
 * strings, which is the backend storage shape (the registry stores strings and
 * the schema validates the format on write).
 *
 * Unlike TimeField this needs no masking or blur handling: picking a day in the
 * calendar is a complete, deliberate value, so the settings row saves it
 * immediately instead of waiting out the 3s text-field debounce.
 */
export function DateField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  disabled = false,
  emptyLabel,
}: DateFieldProps) {
  return (
    <ISODatePicker
      ariaLabel={ariaLabel}
      value={value}
      onChange={onChange}
      disabled={disabled}
      placeholder={emptyLabel ?? "Datum auswählen"}
      calendarLayout="popover"
      className="w-44"
    />
  );
}
