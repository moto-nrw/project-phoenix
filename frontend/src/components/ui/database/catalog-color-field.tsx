"use client";

import { useId } from "react";
import { Button } from "@/components/ui/button";
import { LOCATION_COLORS } from "@/lib/location-helper";

/**
 * Custom field component for picking a catalog color: a room's own badge
 * color, a room category's color and the other school catalogs. Wraps the
 * native `<input type="color">` (works on touch devices, no extra dependency)
 * plus a clear button so admins can reset to the default.
 *
 * Colors reserved for status badges (Schulhof, Unterwegs, eigener
 * Gruppenraum, etc.) are rejected by the backend with 400; the form shows
 * that through its error path. Since #4352 rooms may share a color.
 *
 * Display value is normalised to upper-case hex so stored values compare
 * consistently.
 */
export function CatalogColorField(props: {
  value: unknown;
  onChange: (value: unknown) => void;
  label: string;
  required?: boolean;
  /**
   * Colour previewed while none is set. Defaults to the OTHER_ROOM blue that
   * an ordinary room's badge falls back to; the Schulhof passes its own
   * default so the preview matches what the badge actually renders (#2405).
   */
  defaultHex?: string;
  /** Overrides the hint below the picker. */
  hint?: string;
}) {
  const { value, onChange, label, required, defaultHex, hint } = props;
  const inputId = useId();

  const stringValue =
    typeof value === "string" && value.length > 0 ? value.toUpperCase() : null;

  // Show the badge's own fallback as the preview so picking it intentionally
  // is impossible (the backend reserves every such hex anyway) and the unset
  // state still looks like the badge users see today.
  const displayHex = stringValue ?? defaultHex ?? LOCATION_COLORS.OTHER_ROOM;

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
        {/* Round preview that hosts the native color picker on click. The
            native input itself is invisible but full-size on top, so the
            click target stays accessible and the OS picker still opens. */}
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
        {stringValue && (
          <Button
            type="button"
            onClick={() => onChange(null)}
            variant="ghost"
            size="compact"
            className="h-auto px-0 py-0 text-gray-500 underline hover:text-gray-700"
          >
            Zurücksetzen
          </Button>
        )}
      </div>
      <p className="mt-1 text-xs text-gray-500">
        {hint ?? "Damit färben Sie die Ortsangabe dieses Raums ein."}
      </p>
    </div>
  );
}
