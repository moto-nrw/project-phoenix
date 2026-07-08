"use client";

import { useId } from "react";

/**
 * Generic controlled color field: a round swatch that hosts the native
 * `<input type="color">` (works on touch, no dependency) plus the hex value
 * and an optional reset button. Extracted from the Database RoomColorField
 * pattern so any form (shift types, …) can reuse the same picker without the
 * room-specific copy. Value is normalised to upper-case hex.
 */
export function ColorPickerField({
  value,
  onChange,
  label,
  fallbackColor,
  helpText,
  required = false,
  allowClear = false,
}: {
  readonly value: string | null;
  readonly onChange: (value: string | null) => void;
  readonly label: string;
  /** Preview color shown when value is empty (also the picker's start value). */
  readonly fallbackColor: string;
  readonly helpText?: string;
  readonly required?: boolean;
  /** Show a "Zurücksetzen" button that clears the value to null. */
  readonly allowClear?: boolean;
}) {
  const inputId = useId();

  const stringValue =
    typeof value === "string" && value.length > 0 ? value.toUpperCase() : null;
  const displayHex = stringValue ?? fallbackColor;

  return (
    <div>
      <label
        htmlFor={inputId}
        className="mb-1.5 block text-xs font-medium text-gray-700"
      >
        {label}
        {required && "*"}
      </label>
      <div className="flex items-center gap-3">
        <label
          htmlFor={inputId}
          className="relative inline-block h-9 w-9 shrink-0 cursor-pointer rounded-full border border-gray-300 shadow-sm ring-1 ring-gray-200/50 transition hover:ring-gray-300"
          style={{ backgroundColor: displayHex }}
          aria-label={`${label} auswählen`}
        >
          <input
            id={inputId}
            type="color"
            value={displayHex}
            onChange={(e) => onChange(e.target.value.toUpperCase())}
            className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
            aria-label={label}
          />
        </label>
        <span className="font-mono text-sm text-gray-700">
          {stringValue ?? "Standard"}
        </span>
        {allowClear && stringValue && (
          <button
            type="button"
            onClick={() => onChange(null)}
            className="text-xs text-gray-500 underline hover:text-gray-700"
          >
            Zurücksetzen
          </button>
        )}
      </div>
      {helpText && <p className="mt-1 text-xs text-gray-500">{helpText}</p>}
    </div>
  );
}
