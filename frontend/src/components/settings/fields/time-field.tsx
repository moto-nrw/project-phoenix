"use client";

import { useState, useEffect, useCallback, useRef } from "react";

interface TimeFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /** Label shown when value is empty (e.g., "Jederzeit"). If not set, empty values show the HH:MM placeholder. */
  readonly emptyLabel?: string;
}

/**
 * Masked time input that accepts HH:MM format.
 * When emptyLabel is set and value is empty, shows a styled pill button.
 * When editing, auto-inserts the colon separator and validates on blur.
 */
export function TimeField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
  emptyLabel,
}: TimeFieldProps) {
  const [display, setDisplay] = useState(value);
  const [isEditing, setIsEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

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

  const handleStartEditing = useCallback(() => {
    setIsEditing(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  }, []);

  // Empty value with emptyLabel, not editing — show styled pill
  if (!value && !isEditing && emptyLabel) {
    return (
      <button
        aria-label={`${ariaLabel}: ${emptyLabel}`}
        type="button"
        onClick={handleStartEditing}
        disabled={disabled}
        className="inline-flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-2 text-sm text-gray-500 transition-colors hover:bg-gray-200 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {emptyLabel}
        {!disabled && (
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
            />
          </svg>
        )}
      </button>
    );
  }

  // Has value or editing — show input
  return (
    <div className="flex items-center gap-2">
      <input
        aria-label={ariaLabel}
        ref={inputRef}
        type="text"
        inputMode="numeric"
        placeholder="HH:MM"
        value={display}
        onChange={handleChange}
        onBlur={handleBlur}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.currentTarget.blur();
          }
        }}
        disabled={disabled}
        maxLength={5}
        className="block w-24 rounded-lg border-0 bg-white px-3 py-2.5 text-center text-sm text-gray-900 tabular-nums shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
      />
    </div>
  );
}
