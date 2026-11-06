"use client";

import { useId } from "react";
import { Button } from "@/components/ui/button";
import { ColorSwatchInput } from "@/components/ui/color-swatch-input";
import { FieldLabel } from "@/components/ui/field-label";
import { LOCATION_COLORS } from "@/lib/location-helper";

/**
 * Custom field component for picking a room's badge color in the Database
 * Rooms edit form. Wraps the shared `ColorSwatchInput` plus a clear button so admins can reset to
 * the system default blue.
 *
 * Backend rules the field surfaces by example, not by enforcement:
 *   - Colors reserved for status badges (Schulhof, Unterwegs, eigener
 *     Gruppenraum, etc.) are rejected with 400 + German message; the toast in
 *     rooms.config.tsx surfaces that.
 *   - Two rooms cannot share a color in the same school; backend returns 409
 *     and the toast tells the user to pick a different color.
 *
 * Display value is normalised to upper-case hex so the audit log and the
 * uniqueness index stay consistent.
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
      <FieldLabel
        htmlFor={inputId}
        required={required}
        className="mb-1.5 text-xs"
      >
        {label}
      </FieldLabel>
      <div className="flex items-center gap-3">
        <ColorSwatchInput
          id={inputId}
          value={displayHex}
          onChange={onChange}
          label={label}
        />
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
        {hint ??
          "Damit kannst du das Badge dieses Raums einfärben. Wähle eine Farbe, die du noch nicht für einen anderen Raum benutzt."}
      </p>
    </div>
  );
}
