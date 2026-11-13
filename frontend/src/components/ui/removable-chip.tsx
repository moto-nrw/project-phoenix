"use client";

import { X } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "~/lib/utils";

/**
 * Ein gewählter Eintrag, der sich wieder entfernen lässt: Filter, Klasse,
 * Datum, Datei. Ohne `onRemove` steht der Eintrag nur da, in derselben Form.
 *
 * Das Kreuz bleibt klein, damit der Chip so hoch bleibt wie sein Text. Unter
 * einem Finger reicht seine unsichtbare Trefferfläche auf 44 px; die Fläche
 * endet dort, wo die des Chips in der nächsten Reihe beginnt.
 */
export function RemovableChip({
  label,
  detail,
  icon,
  onRemove,
  removeLabel,
  disabled = false,
}: Readonly<
  {
    label: string;
    /** Leiser Zusatz hinter dem Text, z. B. die Dateigröße. */
    detail?: string;
    icon?: ReactNode;
    disabled?: boolean;
  } & (
    | { onRemove: () => void; removeLabel: string }
    | { onRemove?: never; removeLabel?: never }
  )
>) {
  return (
    <span
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-full border border-gray-200 bg-gray-50 py-1 pl-3 text-sm font-medium text-gray-700",
        onRemove ? "pr-1" : "pr-3",
      )}
    >
      {icon}
      <span className="min-w-0 truncate">{label}</span>
      {detail ? (
        <span className="shrink-0 text-xs font-normal text-gray-500">
          {detail}
        </span>
      ) : null}
      {onRemove ? (
        <button
          type="button"
          onClick={onRemove}
          disabled={disabled}
          aria-label={removeLabel}
          className="relative flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-gray-500 transition-colors hover:bg-gray-200 hover:text-gray-700 focus-visible:ring-2 focus-visible:ring-gray-600 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 pointer-coarse:after:absolute pointer-coarse:after:-inset-3"
        >
          <X className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      ) : null}
    </span>
  );
}
