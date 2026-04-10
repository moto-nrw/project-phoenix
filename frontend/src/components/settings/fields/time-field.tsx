"use client";

import { useState, useEffect, useCallback, useRef } from "react";

interface TimeFieldProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
}

/**
 * Masked time input that accepts HH:MM format.
 * Auto-inserts the colon separator and validates on blur.
 * Uses a plain text input instead of native type="time" for
 * consistent cross-browser appearance.
 */
export function TimeField({
  value,
  onChange,
  onBlur,
  disabled = false,
}: TimeFieldProps) {
  const [display, setDisplay] = useState(value);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync from parent when value changes externally (e.g., after save/reset)
  useEffect(() => {
    setDisplay(value);
  }, [value]);

  const formatTimeInput = useCallback((raw: string): string => {
    // Strip everything except digits
    const digits = raw.replace(/\D/g, "").slice(0, 4);

    if (digits.length <= 2) return digits;
    return `${digits.slice(0, 2)}:${digits.slice(2)}`;
  }, []);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const formatted = formatTimeInput(e.target.value);
      setDisplay(formatted);

      // Only propagate valid HH:MM to parent
      if (/^\d{2}:\d{2}$/.test(formatted)) {
        const [h, m] = formatted.split(":").map(Number);
        if (h !== undefined && m !== undefined && h <= 23 && m <= 59) {
          onChange(formatted);
        }
      }
    },
    [formatTimeInput, onChange],
  );

  const handleBlur = useCallback(() => {
    // On blur, if incomplete or invalid, revert to last valid value
    if (!/^\d{2}:\d{2}$/.test(display)) {
      setDisplay(value);
    } else {
      const [h, m] = display.split(":").map(Number);
      if (h === undefined || m === undefined || h > 23 || m > 59) {
        setDisplay(value);
      }
    }
    onBlur?.();
  }, [display, value, onBlur]);

  return (
    <input
      ref={inputRef}
      type="text"
      inputMode="numeric"
      placeholder="HH:MM"
      value={display}
      onChange={handleChange}
      onBlur={handleBlur}
      disabled={disabled}
      maxLength={5}
      className="block w-24 rounded-lg border-0 bg-white px-3 py-2 text-center text-sm text-gray-900 tabular-nums shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
