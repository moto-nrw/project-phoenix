"use client";

import type { ReactNode } from "react";
import { Checkbox } from "~/components/ui/checkbox";
import { cn } from "~/lib/utils";

/**
 * Eine Auswahlzeile mit Fläche: Kästchen links, Beschriftung und optionaler
 * Hinweis rechts, die ganze Zeile klickbar.
 *
 * Drei Stellen im Anmeldungs-Modul haben sich diese Zeile jeweils selbst
 * gebaut — mit unterschiedlichem Radius, unterschiedlicher Höhe und einmal
 * mit grüner statt dunkler Füllung. Das Kästchen kommt jetzt aus `Checkbox`,
 * die Fläche von hier.
 */
export function CheckboxCard({
  checked,
  onChange,
  label,
  hint,
  disabled = false,
  className,
}: Readonly<{
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: ReactNode;
  hint?: ReactNode;
  disabled?: boolean;
  className?: string;
}>) {
  return (
    <label
      className={cn(
        "flex min-h-11 items-start gap-3 rounded-xl border px-3 py-2.5 text-sm transition-colors",
        "focus-within:ring-2 focus-within:ring-gray-300",
        disabled
          ? "cursor-not-allowed border-gray-100 bg-gray-50 opacity-60"
          : checked
            ? "cursor-pointer border-gray-300 bg-gray-50"
            : "cursor-pointer border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50",
        className,
      )}
    >
      <Checkbox
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-0.5"
      />
      <span className="min-w-0 flex-1 leading-snug">
        <span className="block font-medium text-gray-900">{label}</span>
        {hint ? (
          <span className="mt-0.5 block text-xs font-normal text-gray-500">
            {hint}
          </span>
        ) : null}
      </span>
    </label>
  );
}
