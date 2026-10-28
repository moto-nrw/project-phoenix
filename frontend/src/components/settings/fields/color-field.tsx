"use client";

import { ColorSwatchInput } from "~/components/ui/color-swatch-input";
import { TextField } from "./text-field";

const HEX_COLOR = /^#[0-9A-Fa-f]{6}$/;

interface ColorFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  /** Shown in the swatch while the typed code is not a complete colour. */
  readonly fallback: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
}

/**
 * Colour setting: the swatch opens the colour picker, the text field still
 * takes a colour code typed or pasted from a logo design.
 */
export function ColorField({
  ariaLabel = "Farbe",
  value,
  fallback,
  onChange,
  onBlur,
  disabled = false,
}: ColorFieldProps) {
  const swatch = HEX_COLOR.test(value) ? value : fallback;
  return (
    <div className="flex items-center gap-3">
      <ColorSwatchInput
        value={swatch}
        onChange={onChange}
        onBlur={onBlur}
        label={ariaLabel}
        disabled={disabled}
      />
      <TextField
        ariaLabel={`${ariaLabel} als Farbcode`}
        value={value}
        onChange={onChange}
        onBlur={onBlur}
        disabled={disabled}
      />
    </div>
  );
}
