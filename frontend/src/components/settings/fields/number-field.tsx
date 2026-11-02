"use client";

import { useState, useEffect } from "react";

import { Input } from "~/components/ui/input";

interface NumberFieldProps {
  readonly ariaLabel?: string;
  readonly value: number;
  readonly onChange: (value: number) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  readonly min?: number;
  readonly max?: number;
}

export function NumberField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
  min,
  max,
}: NumberFieldProps) {
  // Local string state so the user can freely type, clear, and edit
  const [text, setText] = useState(value.toString());

  // Sync from parent when value changes externally (e.g., after save/reset)
  useEffect(() => {
    setText(value.toString());
  }, [value]);

  return (
    <Input
      aria-label={ariaLabel}
      controlSize="compact"
      type="number"
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        // Update parent with parsed number (for dirty tracking)
        const num = parseFloat(e.target.value);
        if (!isNaN(num)) {
          onChange(num);
        }
      }}
      onBlur={() => {
        const num = parseFloat(text);
        if (isNaN(num)) {
          setText(value.toString());
        }
        onBlur?.();
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.currentTarget.blur();
        }
      }}
      min={min}
      max={max}
      disabled={disabled}
      className="max-w-[8rem] sm:w-32"
    />
  );
}
