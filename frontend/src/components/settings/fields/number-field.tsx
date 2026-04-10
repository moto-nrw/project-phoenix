"use client";

import { useState, useEffect } from "react";

interface NumberFieldProps {
  readonly value: number;
  readonly onChange: (value: number) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  readonly min?: number;
  readonly max?: number;
}

export function NumberField({
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
    <input
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
        // On blur, if empty or invalid, reset to current value
        const num = parseFloat(text);
        if (isNaN(num)) {
          setText(value.toString());
        }
        onBlur?.();
      }}
      min={min}
      max={max}
      disabled={disabled}
      className="block w-32 rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
