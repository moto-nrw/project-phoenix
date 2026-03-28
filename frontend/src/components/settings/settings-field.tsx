"use client";

import { useState, useCallback, useEffect } from "react";
import type { ResolvedSetting } from "~/lib/settings-api";
import { BooleanField } from "./fields/boolean-field";
import { NumberField } from "./fields/number-field";
import { TimeField } from "./fields/time-field";
import { TextField } from "./fields/text-field";
import { PasswordField } from "./fields/password-field";
import { SelectField } from "./fields/select-field";

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
}

function validateLocally(
  setting: ResolvedSetting,
  value: unknown,
): string | null {
  if (setting.type === "number") {
    const num = Number(value);
    if (isNaN(num)) return "Bitte eine Zahl eingeben.";
    if (setting.validation?.min != null && num < setting.validation.min) {
      return `Minimum: ${setting.validation.min}`;
    }
    if (setting.validation?.max != null && num > setting.validation.max) {
      return `Maximum: ${setting.validation.max}`;
    }
  }
  return null;
}

interface SettingsFieldProps {
  readonly setting: ResolvedSetting;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
}

export function SettingsField({
  setting,
  onSave,
  onReset,
}: SettingsFieldProps) {
  // Local state for text-like fields (save on blur, not on every keystroke)
  const [localValue, setLocalValue] = useState<unknown>(setting.value);
  const [isDirty, setIsDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Sync local state when setting value changes from server (e.g., after save/reset)
  useEffect(() => {
    setLocalValue(setting.value);
    setIsDirty(false);
    setError(null);
  }, [setting.value]);

  // Immediate save — for booleans and selects (one action = one save)
  const handleImmediateSave = useCallback(
    async (value: unknown) => {
      const errorMsg = await onSave(setting.key, value);
      setError(errorMsg);
    },
    [setting.key, onSave],
  );

  // Local change — for text/number/time (update local state, save on blur)
  const handleLocalChange = useCallback((value: unknown) => {
    setLocalValue(value);
    setIsDirty(true);
    setError(null);
  }, []);

  // Save on blur — only if value actually changed
  const handleBlur = useCallback(async () => {
    if (isDirty) {
      const localError = validateLocally(setting, localValue);
      if (localError) {
        setError(localError);
        return;
      }
      const errorMsg = await onSave(setting.key, localValue);
      setError(errorMsg);
      setIsDirty(false);
    }
  }, [isDirty, setting, localValue, onSave]);

  const handleReset = useCallback(async () => {
    const errorMsg = await onReset(setting.key);
    if (!errorMsg) {
      setError(null);
    }
  }, [setting.key, onReset]);

  if (!setting.visible) {
    return null;
  }

  return (
    <div className="flex items-start justify-between gap-4 py-4">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h4 className="text-sm font-medium text-gray-900">{setting.label}</h4>
          {setting.is_default && (
            <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
              Standard
            </span>
          )}
          {!setting.writable && (
            <span className="rounded bg-yellow-50 px-1.5 py-0.5 text-xs text-yellow-700">
              Nur Lesen
            </span>
          )}
        </div>
        {setting.description && (
          <p className="mt-0.5 text-sm text-gray-500">{setting.description}</p>
        )}
        {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {renderField(
          setting,
          localValue,
          handleImmediateSave,
          handleLocalChange,
          handleBlur,
        )}

        {!setting.is_default && setting.writable && (
          <button
            type="button"
            onClick={handleReset}
            className="text-xs text-gray-400 hover:text-gray-600"
            title="Auf Standard zurücksetzen"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
          </button>
        )}
      </div>
    </div>
  );
}

function renderField(
  setting: ResolvedSetting,
  localValue: unknown,
  onImmediateSave: (value: unknown) => Promise<void>,
  onLocalChange: (value: unknown) => void,
  onBlur: () => Promise<void>,
) {
  const disabled = !setting.writable;

  switch (setting.type) {
    case "boolean":
      // Boolean saves immediately (toggle = one action)
      return (
        <BooleanField
          value={Boolean(localValue)}
          onChange={onImmediateSave}
          disabled={disabled}
        />
      );
    case "number":
      // Number saves on blur
      return (
        <NumberField
          value={Number(localValue ?? 0)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
          min={setting.validation?.min}
          max={setting.validation?.max}
        />
      );
    case "time":
      // Time saves on blur
      return (
        <TimeField
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
    case "text":
      // Text saves on blur
      return (
        <TextField
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
    case "password":
      // Password has its own save button
      return (
        <PasswordField
          hasValue={localValue !== "" && localValue !== null}
          onChange={onImmediateSave}
          disabled={disabled}
        />
      );
    case "select":
      // Select saves immediately (pick = one action)
      return (
        <SelectField
          value={localValue}
          onChange={onImmediateSave}
          options={setting.options?.static ?? []}
          disabled={disabled}
        />
      );
    default:
      return (
        <TextField
          value={toStr(localValue)}
          onChange={onLocalChange}
          onBlur={onBlur}
          disabled={disabled}
        />
      );
  }
}
