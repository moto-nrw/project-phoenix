"use client";

import { ROOM_COLOR_PALETTE } from "~/lib/location-helper";

interface RoomColorPickerProps {
  readonly value: unknown;
  readonly onChange: (value: unknown) => void;
  readonly label: string;
  readonly required?: boolean;
}

/**
 * Dumb controlled color picker for rooms. Renders a "Keine Farbe" option plus
 * the curated palette. Callers (e.g. RoomCreateModal) are responsible for
 * pre-filling a sensible default. A stored value that isn't in the palette
 * (e.g. legacy `#4F46E5`) is treated as the "no color picked" state — the
 * cleared swatch lights up as selected, with no separate warning hint.
 */
export function RoomColorPicker({
  value,
  onChange,
  label,
  required,
}: RoomColorPickerProps) {
  const selected = typeof value === "string" ? value.toLowerCase() : "";
  const isInPalette = ROOM_COLOR_PALETTE.some(
    (c) => c.toLowerCase() === selected,
  );
  // Anything not in the palette (null, "", legacy hex) is treated as cleared.
  const treatAsCleared = !isInPalette;

  return (
    <div>
      <label className="mb-1.5 block text-xs font-medium text-gray-700">
        {label}
        {required && <span className="ml-1 text-red-500">*</span>}
      </label>
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => onChange(null)}
          aria-label="Keine Farbe"
          aria-pressed={treatAsCleared}
          title="Keine Farbe (Standardblau)"
          className={`relative flex h-8 w-8 items-center justify-center rounded-full border-2 bg-white transition-transform duration-150 hover:scale-110 ${
            treatAsCleared
              ? "border-gray-900 ring-2 ring-gray-900/20"
              : "border-gray-300 shadow-sm"
          }`}
        >
          <span className="absolute h-[2px] w-5 rotate-45 bg-gray-400" />
        </button>
        {ROOM_COLOR_PALETTE.map((color) => {
          const isSelected = selected === color.toLowerCase();
          return (
            <button
              key={color}
              type="button"
              onClick={() => onChange(color)}
              aria-label={`Farbe ${color}`}
              aria-pressed={isSelected}
              className={`h-8 w-8 rounded-full border-2 transition-transform duration-150 hover:scale-110 ${
                isSelected
                  ? "border-gray-900 ring-2 ring-gray-900/20"
                  : "border-white shadow-sm"
              }`}
              style={{ backgroundColor: color }}
            />
          );
        })}
      </div>
    </div>
  );
}
