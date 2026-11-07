"use client";

import { useId, useState } from "react";
import { Radio } from "~/components/ui/radio";
import { TextField } from "./text-field";

/** A value the text setting offers to pick (registry `options.static`). */
export interface TextPreset {
  readonly label: string;
  readonly value: string;
}

interface TextPresetFieldProps {
  readonly ariaLabel: string;
  readonly value: string;
  /** A typed value: saved after a pause, like any text setting. */
  readonly onChange: (value: string) => void;
  /** A picked preset is a finished value and saves right away. */
  readonly onPick: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  readonly presets: readonly TextPreset[];
  /** Label of the choice that opens the text field. */
  readonly customLabel: string;
  /** The id of the hint that names what is wrong with the own value. */
  readonly errorId?: string;
}

/**
 * Text setting with presets (#6042), built like the colour palette (#5239):
 * the presets first, the own value last. A stored value outside the presets
 * shows as the own value. `customFor` keeps the text field open for the
 * value typed there, even when it equals a preset.
 */
export function TextPresetField({
  ariaLabel,
  value,
  onChange,
  onPick,
  onBlur,
  disabled = false,
  presets,
  customLabel,
  errorId,
}: TextPresetFieldProps) {
  const name = useId();
  const [customFor, setCustomFor] = useState<string | null>(null);
  const isPreset = presets.some((preset) => preset.value === value);
  const custom = !isPreset || customFor === value;

  const enterCustom = (next: string) => {
    setCustomFor(next);
    onChange(next);
  };

  return (
    <div className="w-full min-w-0 space-y-3">
      <div
        role="radiogroup"
        aria-label={ariaLabel}
        className="flex flex-wrap items-center gap-x-5 gap-y-2"
      >
        {presets.map((preset) => (
          <label
            key={preset.value}
            className={`inline-flex items-center gap-2 py-1 text-sm text-gray-700 ${
              disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer"
            }`}
          >
            <Radio
              name={name}
              value={preset.value}
              checked={!custom && preset.value === value}
              onChange={() => {
                setCustomFor(null);
                onPick(preset.value);
              }}
              disabled={disabled}
            />
            {preset.label}
          </label>
        ))}
        <label
          htmlFor={`${name}-custom`}
          className={`inline-flex items-center gap-2 py-1 text-sm text-gray-700 ${
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
          {customLabel}
        </label>
      </div>
      {custom && (
        <TextField
          ariaLabel={`${ariaLabel}: ${customLabel}`}
          value={value}
          onChange={enterCustom}
          onBlur={onBlur}
          disabled={disabled}
          errorId={errorId}
        />
      )}
    </div>
  );
}
