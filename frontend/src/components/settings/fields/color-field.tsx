"use client";

import { useId, useState } from "react";
import { ColorSwatchInput } from "~/components/ui/color-swatch-input";
import { ColorSwatchRadio } from "~/components/ui/color-swatch-radio";
import { Radio } from "~/components/ui/radio";
import { TextField } from "./text-field";

const HEX_COLOR = /^#[0-9A-Fa-f]{6}$/;

/** A named colour the setting offers to pick (registry `options.static`). */
export interface ColorPreset {
  readonly label: string;
  readonly value: string;
}

interface ColorFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  /** Shown in the swatch while the typed code is not a complete colour. */
  readonly fallback: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /**
   * Colours to pick without a colour code (#5239). With presets the picker
   * and the code field sit behind „Eigene Farbe“; without, they show alone.
   */
  readonly presets?: readonly ColorPreset[];
}

function sameColor(a: string, b: string): boolean {
  return a.toUpperCase() === b.toUpperCase();
}

/**
 * Colour setting: the swatch opens the colour picker, the text field still
 * takes a colour code typed or pasted from a logo design.
 */
export function ColorField(props: ColorFieldProps) {
  const { presets = [] } = props;
  if (presets.length === 0) return <CustomColor {...props} />;
  return <ColorPalette {...props} presets={presets} />;
}

/**
 * Presets first, „Eigene Farbe“ last. A stored colour outside the presets
 * shows as „Eigene Farbe“. `customFor` keeps „Eigene Farbe“ open for the
 * value the person chose or entered there, even when it equals a preset; a
 * value from outside (reset, reload) shows where it belongs.
 */
function ColorPalette({
  ariaLabel = "Farbe",
  value,
  onChange,
  onBlur,
  disabled = false,
  presets,
  ...rest
}: ColorFieldProps & { readonly presets: readonly ColorPreset[] }) {
  const name = useId();
  const [customFor, setCustomFor] = useState<string | null>(null);
  const isPreset = presets.some((preset) => sameColor(preset.value, value));
  const custom = !isPreset || customFor === value;

  const enterCustom = (next: string) => {
    setCustomFor(next);
    onChange(next);
  };

  return (
    <div className="space-y-3">
      <div
        role="radiogroup"
        aria-label={ariaLabel}
        className="flex flex-wrap items-center gap-3"
      >
        {presets.map((preset) => (
          <ColorSwatchRadio
            key={preset.value}
            name={name}
            color={preset.value}
            label={preset.label}
            checked={!custom && sameColor(preset.value, value)}
            onSelect={() => {
              setCustomFor(null);
              onChange(preset.value);
            }}
            onBlur={onBlur}
            disabled={disabled}
          />
        ))}
        <label
          htmlFor={`${name}-custom`}
          className={`inline-flex items-center gap-2 text-sm text-gray-700 ${
            disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer"
          }`}
        >
          <Radio
            id={`${name}-custom`}
            name={name}
            checked={custom}
            onChange={() => setCustomFor(value)}
            disabled={disabled}
          />
          Eigene Farbe
        </label>
      </div>
      {custom && (
        <CustomColor
          {...rest}
          ariaLabel={`${ariaLabel}: Eigene Farbe`}
          value={value}
          onChange={enterCustom}
          onBlur={onBlur}
          disabled={disabled}
        />
      )}
    </div>
  );
}

function CustomColor({
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
