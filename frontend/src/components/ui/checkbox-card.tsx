"use client";

import type { ReactNode } from "react";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { cn } from "~/lib/utils";

/**
 * Eine Auswahlzeile mit Fläche: `ChoiceTile` mit `Checkbox` links, Beschriftung
 * und optionalem Hinweis rechts, die ganze Zeile klickbar.
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
    <ChoiceTile
      selected={checked}
      disabled={disabled}
      className={cn("min-h-11 items-start", className)}
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
          <span className="mt-0.5 block text-sm font-normal text-gray-500">
            {hint}
          </span>
        ) : null}
      </span>
    </ChoiceTile>
  );
}
