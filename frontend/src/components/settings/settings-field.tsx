"use client";

import { useCallback } from "react";
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

interface SettingsFieldProps {
  readonly setting: ResolvedSetting;
  readonly onSave: (key: string, value: unknown) => void;
  readonly onReset: (key: string) => void;
}

export function SettingsField({
  setting,
  onSave,
  onReset,
}: SettingsFieldProps) {
  const handleChange = useCallback(
    (value: unknown) => {
      onSave(setting.key, value);
    },
    [setting.key, onSave],
  );

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
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {renderField(setting, handleChange)}

        {!setting.is_default && setting.writable && (
          <button
            type="button"
            onClick={() => onReset(setting.key)}
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
  onChange: (value: unknown) => void,
) {
  const disabled = !setting.writable;

  switch (setting.type) {
    case "boolean":
      return (
        <BooleanField
          value={Boolean(setting.value)}
          onChange={onChange}
          disabled={disabled}
        />
      );
    case "number":
      return (
        <NumberField
          value={Number(setting.value ?? 0)}
          onChange={onChange}
          disabled={disabled}
          min={setting.validation?.min}
          max={setting.validation?.max}
        />
      );
    case "time":
      return (
        <TimeField
          value={toStr(setting.value)}
          onChange={onChange}
          disabled={disabled}
        />
      );
    case "text":
      return (
        <TextField
          value={toStr(setting.value)}
          onChange={onChange}
          disabled={disabled}
        />
      );
    case "password":
      return (
        <PasswordField
          hasValue={setting.value !== "" && setting.value !== null}
          onChange={onChange}
          disabled={disabled}
        />
      );
    case "select":
      return (
        <SelectField
          value={setting.value}
          onChange={onChange}
          options={setting.options?.static ?? []}
          disabled={disabled}
        />
      );
    default:
      return (
        <TextField
          value={toStr(setting.value)}
          onChange={onChange}
          disabled={disabled}
        />
      );
  }
}
