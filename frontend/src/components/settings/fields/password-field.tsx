"use client";

import { useState } from "react";

interface PasswordFieldProps {
  readonly hasValue: boolean;
  readonly onChange: (value: unknown) => Promise<void>;
  readonly disabled?: boolean;
  /** Regex pattern string for validation (e.g., "^\\d{4}$" for 4-digit PIN) */
  readonly pattern?: string;
}

/** Derive input hints from the pattern */
function getInputHints(pattern?: string): {
  inputMode: "text" | "numeric";
  maxLength?: number;
  placeholder: string;
} {
  if (!pattern) {
    return { inputMode: "text", placeholder: "Neuen Wert eingeben" };
  }
  // Detect digit-only patterns like ^\d{4}$
  const digitMatch = pattern.match(/^[\^]?\\d\{(\d+)\}\$?$/);
  if (digitMatch?.[1]) {
    const len = parseInt(digitMatch[1], 10);
    return {
      inputMode: "numeric",
      maxLength: len,
      placeholder: "•".repeat(len),
    };
  }
  return { inputMode: "text", placeholder: "Neuen Wert eingeben" };
}

export function PasswordField({
  hasValue,
  onChange,
  disabled = false,
  pattern,
}: PasswordFieldProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [newValue, setNewValue] = useState("");
  const [showValue, setShowValue] = useState(false);

  const hints = getInputHints(pattern);
  const isPin = hints.inputMode === "numeric";

  // Display mode — clickable pill opens edit
  if (!isEditing) {
    return (
      <button
        type="button"
        onClick={() => {
          setIsEditing(true);
          setShowValue(false);
        }}
        disabled={disabled}
        className="inline-flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-2 text-sm text-gray-500 tabular-nums transition-colors hover:bg-gray-200 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {hasValue ? (isPin ? "••••" : "••••••") : "Nicht gesetzt"}
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

  // Edit mode — input with eye toggle, save, cancel
  return (
    <div className="flex items-center gap-2">
      <div className="relative">
        <input
          type={showValue ? "text" : "password"}
          inputMode={hints.inputMode}
          maxLength={hints.maxLength}
          value={newValue}
          onChange={(e) => {
            let val = e.target.value;
            if (isPin) {
              val = val.replace(/\D/g, "");
            }
            setNewValue(val);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && newValue) {
              void onChange(newValue).then(() => {
                setNewValue("");
                setIsEditing(false);
              });
            }
            if (e.key === "Escape") {
              setNewValue("");
              setIsEditing(false);
            }
          }}
          placeholder={hints.placeholder}
          autoFocus
          className={`block rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 ${
            isPin
              ? "w-28 pr-9 text-center tracking-widest tabular-nums"
              : "w-48 pr-9"
          }`}
        />
        <button
          type="button"
          onClick={() => setShowValue(!showValue)}
          className="absolute inset-y-0 right-0 flex items-center pr-2.5 text-gray-400 hover:text-gray-600"
          tabIndex={-1}
          aria-label={showValue ? "Wert verbergen" : "Wert anzeigen"}
        >
          {showValue ? (
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
              />
            </svg>
          ) : (
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              />
            </svg>
          )}
        </button>
      </div>
      <button
        type="button"
        onClick={async () => {
          if (newValue) {
            await onChange(newValue);
            setNewValue("");
            setIsEditing(false);
          }
        }}
        className="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white transition-all hover:bg-gray-700"
      >
        Speichern
      </button>
      <button
        type="button"
        onClick={() => {
          setNewValue("");
          setIsEditing(false);
        }}
        className="text-sm text-gray-500 hover:text-gray-700"
      >
        Abbrechen
      </button>
    </div>
  );
}
