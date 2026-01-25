"use client";

import { type ResolvedSetting, valueToString } from "~/lib/settings-helpers";

export type SettingColorVariant = "gray" | "green" | "purple";

interface SettingInputProps {
  setting: ResolvedSetting;
  isSaving: boolean;
  isDisabled: boolean;
  variant?: SettingColorVariant;
  onChange: (key: string, value: unknown) => void;
}

const variantStyles: Record<
  SettingColorVariant,
  { toggle: string; focus: string; spinner: string }
> = {
  gray: {
    toggle: "peer-checked:bg-gray-900 peer-focus:ring-gray-300",
    focus: "focus:ring-gray-300",
    spinner: "border-t-gray-900",
  },
  green: {
    toggle: "peer-checked:bg-green-600 peer-focus:ring-green-300",
    focus: "focus:ring-green-300",
    spinner: "border-t-green-600",
  },
  purple: {
    toggle: "peer-checked:bg-purple-600 peer-focus:ring-purple-300",
    focus: "focus:ring-purple-300",
    spinner: "border-t-purple-600",
  },
};

export function SettingInput({
  setting,
  isSaving,
  isDisabled,
  variant = "gray",
  onChange,
}: SettingInputProps) {
  const styles = variantStyles[variant];

  const handleChange = (value: unknown) => {
    onChange(setting.key, value);
  };

  switch (setting.type) {
    case "bool":
      return (
        <label className="relative inline-flex cursor-pointer items-center">
          <input
            type="checkbox"
            checked={Boolean(setting.value)}
            onChange={(e) => handleChange(e.target.checked)}
            disabled={isDisabled}
            className="peer sr-only"
          />
          <div
            className={`peer h-6 w-11 rounded-full bg-gray-200 ${styles.toggle} peer-focus:ring-2 peer-disabled:cursor-not-allowed peer-disabled:opacity-50 after:absolute after:top-0.5 after:left-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white`}
          />
          {isSaving && (
            <span className="ml-2 text-xs text-gray-500">Speichern...</span>
          )}
        </label>
      );

    case "int":
      return (
        <div className="flex items-center gap-2">
          <input
            type="number"
            value={valueToString(setting.value)}
            onChange={(e) => {
              const val = parseInt(e.target.value, 10);
              if (!isNaN(val)) {
                handleChange(val);
              }
            }}
            disabled={isDisabled}
            min={setting.validation?.min}
            max={setting.validation?.max}
            className={`w-24 rounded-lg border border-gray-200 px-3 py-2 text-sm ${styles.focus} focus:ring-2 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500`}
          />
          {isSaving && (
            <span className="text-xs text-gray-500">Speichern...</span>
          )}
        </div>
      );

    case "enum":
      return (
        <div className="flex items-center gap-2">
          <select
            value={valueToString(setting.value)}
            onChange={(e) => handleChange(e.target.value)}
            disabled={isDisabled}
            className={`rounded-lg border border-gray-200 px-3 py-2 text-sm ${styles.focus} focus:ring-2 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500`}
          >
            {setting.validation?.options?.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
          {isSaving && (
            <span className="text-xs text-gray-500">Speichern...</span>
          )}
        </div>
      );

    case "time":
      return (
        <div className="flex items-center gap-2">
          <input
            type="time"
            value={valueToString(setting.value)}
            onChange={(e) => handleChange(e.target.value)}
            disabled={isDisabled}
            className={`rounded-lg border border-gray-200 px-3 py-2 text-sm ${styles.focus} focus:ring-2 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500`}
          />
          {isSaving && (
            <span className="text-xs text-gray-500">Speichern...</span>
          )}
        </div>
      );

    case "string":
    default:
      return (
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={valueToString(setting.value)}
            onChange={(e) => handleChange(e.target.value)}
            disabled={isDisabled}
            pattern={setting.validation?.pattern}
            className={`w-full max-w-xs rounded-lg border border-gray-200 px-3 py-2 text-sm ${styles.focus} focus:ring-2 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500`}
          />
          {isSaving && (
            <span className="text-xs text-gray-500">Speichern...</span>
          )}
        </div>
      );
  }
}

interface SettingLoadingSpinnerProps {
  variant?: SettingColorVariant;
}

export function SettingLoadingSpinner({
  variant = "gray",
}: SettingLoadingSpinnerProps) {
  const styles = variantStyles[variant];
  return (
    <div className="flex items-center justify-center py-8">
      <div
        className={`h-8 w-8 animate-spin rounded-full border-2 border-gray-300 ${styles.spinner}`}
      />
    </div>
  );
}
