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
 * When value is empty, shows a "Jederzeit" label with an edit button.
 * When editing, auto-inserts the colon separator and validates on blur.
 */
export function TimeField({
  value,
  onChange,
  onBlur,
  disabled = false,
}: TimeFieldProps) {
  const [display, setDisplay] = useState(value);
  const [isEditing, setIsEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync from parent when value changes externally
  useEffect(() => {
    setDisplay(value);
    if (value === "") {
      setIsEditing(false);
    }
  }, [value]);

  const formatTimeInput = useCallback((raw: string): string => {
    const digits = raw.replace(/\D/g, "").slice(0, 4);
    if (digits.length <= 2) return digits;
    return `${digits.slice(0, 2)}:${digits.slice(2)}`;
  }, []);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const formatted = formatTimeInput(e.target.value);
      setDisplay(formatted);

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
    if (!/^\d{2}:\d{2}$/.test(display)) {
      // Incomplete input — revert
      setDisplay(value);
      if (value === "") {
        setIsEditing(false);
      }
    } else {
      const [h, m] = display.split(":").map(Number);
      if (h === undefined || m === undefined || h > 23 || m > 59) {
        setDisplay(value);
        if (value === "") {
          setIsEditing(false);
        }
      }
    }
    onBlur?.();
  }, [display, value, onBlur]);

  const handleClear = useCallback(() => {
    setDisplay("");
    setIsEditing(false);
    onChange("");
  }, [onChange]);

  const handleStartEditing = useCallback(() => {
    setIsEditing(true);
    // Focus the input after it renders
    setTimeout(() => inputRef.current?.focus(), 0);
  }, []);

  // Empty value, not editing — show "Jederzeit" with edit button
  if (!value && !isEditing) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-sm text-gray-500">Jederzeit</span>
        {!disabled && (
          <button
            type="button"
            onClick={handleStartEditing}
            className="text-sm font-medium text-gray-700 hover:text-gray-900"
          >
            Ändern
          </button>
        )}
      </div>
    );
  }

  // Has value or editing — show input
  return (
    <div className="flex items-center gap-2">
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
      {value && !disabled && (
        <button
          type="button"
          onClick={handleClear}
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          Entfernen
        </button>
      )}
    </div>
  );
}
