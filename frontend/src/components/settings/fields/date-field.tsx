"use client";

import { useEffect, useState } from "react";

interface DateFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /** Label shown when the value is empty (e.g., "Kein Datum"). */
  readonly emptyLabel?: string;
}

/**
 * Native date input. Browsers render a calendar picker for type="date" and
 * round-trip ISO YYYY-MM-DD strings, which matches the backend storage shape
 * for FieldDate settings (the registry stores strings, the schema validates
 * the format on write).
 *
 * No custom masking like TimeField — the native control handles malformed
 * input. Empty values are surfaced as a plain pill when emptyLabel is set,
 * matching TimeField's "Jederzeit" affordance for the daily-checkout setting.
 */
export function DateField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
  emptyLabel,
}: DateFieldProps) {
  const [display, setDisplay] = useState(value);

  useEffect(() => {
    setDisplay(value);
  }, [value]);

  // When emptyLabel is set and value is empty, show a styled pill button
  // so it's obvious the field is currently unset.
  if (!value && emptyLabel) {
    return (
      <input
        aria-label={ariaLabel}
        type="date"
        value={display}
        onChange={(e) => {
          setDisplay(e.target.value);
          onChange(e.target.value);
        }}
        onBlur={onBlur}
        disabled={disabled}
        placeholder={emptyLabel}
        className="block w-44 rounded-lg border-0 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
      />
    );
  }

  return (
    <input
      aria-label={ariaLabel}
      type="date"
      value={display}
      onChange={(e) => {
        setDisplay(e.target.value);
        onChange(e.target.value);
      }}
      onBlur={onBlur}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.currentTarget.blur();
        }
      }}
      disabled={disabled}
      className="block w-44 rounded-lg border-0 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
