"use client";

import { useId } from "react";

/**
 * Ein Uhrzeitfeld mit sichtbarem Format.
 *
 * Das native `<input type="time">` zeigt leer ein rohes "--:--" und oeffnet je
 * nach Browser ein Systemrad, das auf dem Handy schwer zu treffen ist. Dieses
 * Feld ist ein normales Textfeld: 48 px hoch, 17 px Schrift, mit sichtbarem
 * Hinweis auf das Format und automatischem Doppelpunkt nach zwei Ziffern.
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

  // Nur Ziffern und der Doppelpunkt; nach zwei Ziffern setzt das Feld ihn
  // selbst, damit niemand ihn auf einer Zifferntastatur suchen muss.
  const normalize = (raw: string): string => {
    const digits = raw.replace(/\D/g, "").slice(0, 4);
    if (digits.length <= 2) return digits;
    return `${digits.slice(0, 2)}:${digits.slice(2)}`;
  };

  return (
    <label className="block">
      <span className="mb-1 block text-[15px] font-medium text-gray-700">
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
        maxLength={5}
        required={required}
        aria-required={required}
        aria-invalid={invalid}
        aria-describedby={[describedBy, hintId].filter(Boolean).join(" ")}
        onChange={(event) => onChange(normalize(event.target.value))}
        className={`h-12 w-full rounded-lg border px-3 text-[17px] text-gray-900 focus-visible:ring-2 focus-visible:ring-[#5080D8]/40 focus-visible:outline-none ${
          invalid
            ? "border-parent-red focus-visible:border-parent-red"
            : "border-gray-300 focus-visible:border-gray-400"
        }`}
      />
      <span id={hintId} className="mt-1 block text-[15px] text-gray-500">
        {hint}
      </span>
    </label>
  );
}
