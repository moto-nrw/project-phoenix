"use client";

import { LOCATION_COLORS } from "@/lib/location-helper";

/**
 * Swatch picker for an activity category's colour (#2131).
 *
 * Categories are labels, not rooms: schools pick from the brand palette rather
 * than a free colour wheel, so the value can never drift off
 * `LOCATION_COLORS`. "Ohne Farbe" clears the field; the backend then falls
 * back to its neutral display default.
 */
const CATEGORY_COLORS: ReadonlyArray<{ hex: string; label: string }> = [
  { hex: LOCATION_COLORS.GROUP_ROOM, label: "Grün" },
  { hex: LOCATION_COLORS.OTHER_ROOM, label: "Blau" },
  { hex: LOCATION_COLORS.SCHOOLYARD, label: "Orange" },
  { hex: LOCATION_COLORS.HOME, label: "Rot" },
  { hex: LOCATION_COLORS.SICK, label: "Gelb" },
  { hex: LOCATION_COLORS.EXCUSED, label: "Lila" },
  { hex: LOCATION_COLORS.TRANSIT, label: "Magenta" },
  { hex: LOCATION_COLORS.UNKNOWN, label: "Grau" },
];

export function CategoryColorField(props: {
  value: unknown;
  onChange: (value: unknown) => void;
  label: string;
  required?: boolean;
}) {
  const { value, onChange, label, required } = props;
  const selected =
    typeof value === "string" && value.length > 0 ? value.toUpperCase() : null;

  return (
    <fieldset>
      <legend className="mb-1.5 block text-xs font-medium text-gray-700">
        {label}
        {required && "*"}
      </legend>
      <div className="flex flex-wrap items-center gap-2">
        {CATEGORY_COLORS.map((color) => {
          const isSelected = selected === color.hex.toUpperCase();
          return (
            <button
              key={color.hex}
              type="button"
              onClick={() => onChange(color.hex)}
              aria-label={color.label}
              aria-pressed={isSelected}
              title={color.label}
              className={`h-8 w-8 rounded-full border transition ${
                isSelected
                  ? "border-gray-900 ring-2 ring-gray-900/20"
                  : "border-gray-300 hover:ring-2 hover:ring-gray-300"
              }`}
              style={{ backgroundColor: color.hex }}
            />
          );
        })}
        <button
          type="button"
          onClick={() => onChange("")}
          aria-pressed={selected === null}
          className={`h-8 rounded-md border px-2.5 text-xs font-medium transition ${
            selected === null
              ? "border-gray-900 bg-gray-100 text-gray-900"
              : "border-gray-300 text-gray-600 hover:bg-gray-50"
          }`}
        >
          Ohne Farbe
        </button>
      </div>
    </fieldset>
  );
}
