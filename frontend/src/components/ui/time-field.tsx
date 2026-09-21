"use client";

import { useId } from "react";

/**
 * Nur Ziffern und der Doppelpunkt; auch einstellige Stunden bleiben gültig.
 * Geteilt mit dem Uhrzeitfeld der Einstellungen (`SettingsTimeField`), damit
 * es die Maske nicht ein zweites Mal gibt (#3117).
 */
export function normalizeTimeInput(raw: string, previousValue = ""): string {
  const explicitTime = raw.match(/^(\d{1,2}):(\d{0,2})$/);
  if (explicitTime) {
    return `${explicitTime[1]!.padStart(2, "0")}:${explicitTime[2]!}`;
  }

  const rawDigits = raw.replace(/\D/g, "");
  // After completing a one-digit hour, a fourth typed digit turns
  // "01:53" + "0" back into the intended two-digit time "15:30".
  if (
    rawDigits.length === 5 &&
    previousValue === `${rawDigits.slice(0, 2)}:${rawDigits.slice(2, 4)}`
  ) {
    return `${rawDigits.slice(1, 3)}:${rawDigits.slice(3)}`;
  }

  const digits = rawDigits.slice(0, 4);
  if (digits.length <= 2) return digits;
  if (digits.length === 3) {
    // A completed three-digit value is a one-digit hour ("130" → "01:30").
    // Only keep a two-digit hour open when the one-digit variant is invalid.
    if (Number(digits.slice(1)) <= 59) {
      return `0${digits.slice(0, 1)}:${digits.slice(1)}`;
    }
    if (
      previousValue === digits.slice(0, 2) &&
      Number(digits.slice(0, 2)) <= 23
    ) {
      return `${digits.slice(0, 2)}:${digits.slice(2)}`;
    }
    return `0${digits.slice(0, 1)}:${digits.slice(1)}`;
  }
  return `${digits.slice(0, 2)}:${digits.slice(2)}`;
}

/**
 * Ein Uhrzeitfeld mit sichtbarem Format.
 *
 * Das native `<input type="time">` zeigt leer ein rohes "--:--" und oeffnet je
 * nach Browser ein Systemrad. Dieses Feld ist ein normales Textfeld mit
 * sichtbarem Formathinweis und automatischem Doppelpunkt nach zwei Ziffern.
 *
 * Der Wert bleibt "HH:MM" wie beim nativen Feld, damit aufrufender Code und
 * Schnittstelle unveraendert bleiben.
 */
export function TimeField({
  value,
  onChange,
  label,
  hint,
  placeholder,
  required = false,
  invalid = false,
  describedBy,
  inputRef,
}: Readonly<{
  value: string;
  onChange: (value: string) => void;
  label: string;
  /** Sichtbarer Formathinweis, z. B. "Uhrzeit im Format 15:30". */
  hint: string;
  /** Beispielzeit im leeren Feld. Nie das rohe native "--:--". */
  placeholder: string;
  required?: boolean;
  invalid?: boolean;
  describedBy?: string;
  inputRef?: React.Ref<HTMLInputElement>;
}>) {
  const hintId = useId();

  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium text-gray-700">
        {label}
        {required && <span aria-hidden="true"> *</span>}
      </span>
      <input
        ref={inputRef}
        type="text"
        inputMode="numeric"
        autoComplete="off"
        value={value}
        placeholder={placeholder}
        maxLength={6}
        required={required}
        aria-required={required}
        aria-invalid={invalid}
        aria-describedby={[describedBy, hintId].filter(Boolean).join(" ")}
        onChange={(event) =>
          onChange(normalizeTimeInput(event.target.value, value))
        }
        className={`h-10 w-full rounded-lg border px-3 text-base text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
          invalid
            ? "border-parent-red focus-visible:border-parent-red"
            : "border-gray-300 focus-visible:border-gray-400"
        }`}
      />
      <span id={hintId} className="mt-1 block text-xs text-gray-500">
        {hint}
      </span>
    </label>
  );
}
