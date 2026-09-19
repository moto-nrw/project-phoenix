"use client";

import { useState, useCallback } from "react";
import { Eye, EyeOff, Pencil } from "lucide-react";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { SpinnerIcon } from "~/components/ui/icons";
import { Input } from "~/components/ui/input";
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

/**
 * Das einzige Einstellungsfeld, das NICHT sofort speichert: ein Passwort
 * oder eine PIN wird erst mit „Speichern“ übernommen. Weil die
 * Einstellungsseite ringsum Sofort-Speichern verspricht, benennt das Feld
 * diese Ausnahme im Bearbeiten-Zustand selbst (Bauart 4 Regel 3, #3117).
 * Knöpfe, Eingabefeld und Auge kommen aus dem Kit.
 */
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
  const [isSaving, setIsSaving] = useState(false);

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

  const stopEditing = useCallback(() => {
    setNewValue("");
    setIsEditing(false);
  }, []);

  const save = useCallback(async () => {
    if (!newValue) return;
    setIsSaving(true);
    try {
      await onChange(newValue);
      stopEditing();
    } finally {
      setIsSaving(false);
    }
  }, [newValue, onChange, stopEditing]);

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
      <div className="flex flex-wrap items-center gap-1.5">
        {/* Pill with value + eye toggle inside */}
        <span
          className={`inline-flex items-center gap-1 rounded-lg bg-gray-100 py-1 pr-1 pl-3 text-sm tabular-nums ${showValue && revealedValue ? "font-mono tracking-widest text-gray-900" : "text-gray-500"}`}
        >
          {isRevealing ? (
            <SpinnerIcon className="my-1.5 h-3.5 w-3.5" />
          ) : (
            <span className="py-1">{displayText}</span>
          )}
          {hasValue && !disabled ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 text-gray-400"
              onClick={() => void handleRevealToggle()}
              aria-label={showValue ? "Wert verbergen" : "Wert anzeigen"}
            >
              {showValue ? (
                <EyeOff className="h-3.5 w-3.5" aria-hidden="true" />
              ) : (
                <Eye className="h-3.5 w-3.5" aria-hidden="true" />
              )}
            </Button>
          ) : (
            <span className="w-2" aria-hidden="true" />
          )}
        </span>

        {/* Edit button */}
        {!disabled && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => {
              setIsEditing(true);
              setShowValue(false);
              setRevealedValue(null);
            }}
            aria-label="Wert ändern"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
          </Button>
        )}
      </div>
    );
  }

  // Edit mode
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <div className={`relative ${isPin ? "w-28" : "w-full sm:w-48"}`}>
          <Input
            aria-label="Neuer Wert"
            type={showValue ? "text" : "password"}
            inputMode={hints.inputMode}
            maxLength={hints.maxLength}
            value={newValue}
            controlSize="compact"
            autoComplete="off"
            onChange={(e) => {
              let val = e.target.value;
              if (isPin) val = val.replace(/\D/g, "");
              setNewValue(val);
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && newValue) {
                void save();
              }
              if (e.key === "Escape") {
                stopEditing();
              }
            }}
            placeholder={hints.placeholder}
            className={
              isPin ? "pr-9 text-center tracking-widest tabular-nums" : "pr-9"
            }
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setShowValue(!showValue)}
            className="absolute inset-y-0 right-1 my-auto h-7 text-gray-400"
            tabIndex={-1}
            aria-label={showValue ? "Wert verbergen" : "Wert anzeigen"}
          >
            {showValue ? (
              <EyeOff className="h-4 w-4" aria-hidden="true" />
            ) : (
              <Eye className="h-4 w-4" aria-hidden="true" />
            )}
          </Button>
        </div>
        <EditActions
          className="justify-start"
          onCancel={stopEditing}
          onSave={() => void save()}
          saving={isSaving}
          disabled={!newValue}
        />
      </div>
      <p className="text-xs text-gray-500">
        Der neue Wert gilt erst nach „Speichern“.
      </p>
    </div>
  );
}
