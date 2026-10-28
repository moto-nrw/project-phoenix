"use client";

import { Check } from "lucide-react";

/**
 * One colour of a palette, picked like a radio button: a round swatch with
 * a check mark while chosen. Swatches with the same `name` form one choice,
 * so the arrow keys move between them. The colour's name is the accessible
 * label and the tooltip; it stays out of the layout.
 */
export function ColorSwatchRadio(props: {
  name: string;
  /** Hex colour `#RRGGBB` shown in the swatch. */
  color: string;
  label: string;
  checked: boolean;
  onSelect: () => void;
  onBlur?: () => void;
  disabled?: boolean;
}) {
  const {
    name,
    color,
    label,
    checked,
    onSelect,
    onBlur,
    disabled = false,
  } = props;
  return (
    <label
      title={label}
      className={`relative inline-flex shrink-0 ${
        disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer"
      }`}
    >
      <input
        type="radio"
        name={name}
        value={color}
        checked={checked}
        onChange={onSelect}
        onBlur={onBlur}
        disabled={disabled}
        aria-label={label}
        className="peer sr-only"
      />
      <span
        aria-hidden="true"
        className="flex h-9 w-9 items-center justify-center rounded-full border border-gray-300 shadow-sm ring-offset-2 transition peer-checked:ring-2 peer-checked:ring-gray-900 peer-focus-visible:ring-2 peer-focus-visible:ring-gray-400 peer-checked:[&>svg]:opacity-100"
        style={{ backgroundColor: color }}
      >
        <Check className="h-4 w-4 text-white opacity-0" strokeWidth={3} />
      </span>
    </label>
  );
}
