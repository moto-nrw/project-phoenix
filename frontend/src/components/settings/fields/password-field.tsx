"use client";

import { useState, useCallback } from "react";
import { revealSettingValue } from "~/lib/settings-api";

interface PasswordFieldProps {
  readonly hasValue: boolean;
  readonly settingKey: string;
  readonly onChange: (value: unknown) => Promise<void>;
  readonly disabled?: boolean;
  readonly pattern?: string;
  // revealFn overrides how the unmasked value is fetched. Defaults to the
  // tenant-scoped reveal endpoint. Operator-side callers should pass a
  // function bound to their school ID so reveal hits the operator API
  // instead. Signature matches (key) => Promise<string | null>.
  readonly revealFn?: (key: string) => Promise<string | null>;
}

function getInputHints(pattern?: string): {
  inputMode: "text" | "numeric";
  maxLength?: number;
  placeholder: string;
} {
  if (!pattern) {
    return { inputMode: "text", placeholder: "Neuen Wert eingeben" };
  }
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
  settingKey,
  onChange,
  disabled = false,
  pattern,
  revealFn = revealSettingValue,
}: PasswordFieldProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [newValue, setNewValue] = useState("");
  const [showValue, setShowValue] = useState(false);
  const [revealedValue, setRevealedValue] = useState<string | null>(null);
  const [isRevealing, setIsRevealing] = useState(false);

  const hints = getInputHints(pattern);
  const isPin = hints.inputMode === "numeric";

  const handleRevealToggle = useCallback(async () => {
    if (showValue) {
      setShowValue(false);
      setRevealedValue(null);
      return;
    }
    // Only show spinner if the fetch takes longer than 150ms (avoids flicker)
    const spinnerTimeout = setTimeout(() => setIsRevealing(true), 150);
    const value = await revealFn(settingKey);
    clearTimeout(spinnerTimeout);
    setRevealedValue(value);
    setShowValue(true);
    setIsRevealing(false);
  }, [showValue, settingKey, revealFn]);

  // Display mode
  if (!isEditing) {
    const displayText =
      showValue && revealedValue != null
        ? revealedValue
        : hasValue
          ? isPin
            ? "••••"
            : "••••••"
          : "Nicht gesetzt";

    return (
      <div className="flex items-center gap-1.5">
        {/* Pill with value + eye toggle inside */}
        <span
          className={`inline-flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-2 text-sm tabular-nums ${showValue && revealedValue ? "font-mono tracking-widest text-gray-900" : "text-gray-500"}`}
        >
          {isRevealing ? (
            <svg
              className="h-3.5 w-3.5 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
          ) : (
            displayText
          )}
          {hasValue && !disabled && (
            <button
              type="button"
              onClick={handleRevealToggle}
              className="text-gray-400 transition-colors hover:text-gray-600"
              aria-label={showValue ? "Wert verbergen" : "Wert anzeigen"}
            >
              {showValue ? (
                <svg
                  className="h-3.5 w-3.5"
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
                  className="h-3.5 w-3.5"
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
          )}
        </span>

        {/* Edit button */}
        {!disabled && (
          <button
            type="button"
            onClick={() => {
              setIsEditing(true);
              setShowValue(false);
              setRevealedValue(null);
            }}
            className="rounded-lg p-2.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
            aria-label="Wert ändern"
          >
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
                d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
              />
            </svg>
          </button>
        )}
      </div>
    );
  }

  // Edit mode
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="relative">
        <input
          type={showValue ? "text" : "password"}
          inputMode={hints.inputMode}
          maxLength={hints.maxLength}
          value={newValue}
          onChange={(e) => {
            let val = e.target.value;
            if (isPin) val = val.replace(/\D/g, "");
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
          className={`block rounded-lg border-0 bg-white px-3 py-2.5 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 ${
            isPin
              ? "w-28 pr-9 text-center tracking-widest tabular-nums"
              : "w-full pr-9 sm:w-48"
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
        className="px-2 text-sm text-gray-500 hover:text-gray-700"
      >
        Abbrechen
      </button>
    </div>
  );
}
