"use client";

import { useState, useCallback } from "react";
import type { ResolvedSetting } from "~/lib/settings-helpers";
import { ObjectRefSelect } from "./object-ref-select";

interface SettingControlProps {
  setting: ResolvedSetting;
  onChange: (value: string) => void;
  disabled?: boolean;
  deviceId?: string;
}

export function SettingControl({
  setting,
  onChange,
  disabled = false,
  deviceId,
}: SettingControlProps) {
  const [localValue, setLocalValue] = useState(setting.effectiveValue);
  const { definition } = setting;
  const isDisabled = disabled || !setting.canEdit;

  // For sensitive settings, show placeholder
  const displayValue = definition.isSensitive ? "" : localValue;

  const handleChange = useCallback(
    (newValue: string) => {
      setLocalValue(newValue);
      onChange(newValue);
    },
    [onChange],
  );

  // Boolean toggle
  if (definition.valueType === "bool") {
    return (
      <label className="relative inline-flex cursor-pointer items-center">
        <input
          type="checkbox"
          checked={localValue === "true"}
          onChange={(e) => handleChange(e.target.checked ? "true" : "false")}
          disabled={isDisabled}
          className="peer sr-only"
        />
        <div className="peer h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-blue-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 peer-disabled:cursor-not-allowed peer-disabled:opacity-50"></div>
        <span className="ml-3 text-sm font-medium text-gray-900">
          {localValue === "true" ? "Ja" : "Nein"}
        </span>
      </label>
    );
  }

  // Enum select - use enumOptions for labels if available, otherwise fall back to enumValues
  if (definition.valueType === "enum" && (definition.enumOptions ?? definition.enumValues)) {
    // Prefer enumOptions (with labels), fall back to enumValues (value-only)
    const options = definition.enumOptions
      ? definition.enumOptions.map((opt) => ({ value: opt.value, label: opt.label }))
      : (definition.enumValues ?? []).map((v) => ({ value: v, label: v }));

    return (
      <select
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    );
  }

  // Object reference select
  if (definition.valueType === "objectRef") {
    return (
      <ObjectRefSelect
        setting={setting}
        value={displayValue}
        onChange={handleChange}
        disabled={isDisabled}
        deviceId={deviceId}
      />
    );
  }

  // Integer input
  if (definition.valueType === "int") {
    return (
      <input
        type="number"
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        min={definition.validation?.min}
        max={definition.validation?.max}
        step={1}
        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
      />
    );
  }

  // Float input
  if (definition.valueType === "float") {
    return (
      <input
        type="number"
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        min={definition.validation?.min}
        max={definition.validation?.max}
        step="any"
        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
      />
    );
  }

  // Time input
  if (definition.valueType === "time") {
    return (
      <input
        type="time"
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
      />
    );
  }

  // Duration input (e.g., "1h30m")
  if (definition.valueType === "duration") {
    return (
      <input
        type="text"
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        placeholder="z.B. 1h30m, 30m, 2h"
        className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
      />
    );
  }

  // JSON textarea
  if (definition.valueType === "json") {
    return (
      <textarea
        value={displayValue}
        onChange={(e) => handleChange(e.target.value)}
        disabled={isDisabled}
        rows={4}
        className="block w-full rounded-md border-gray-300 font-mono text-sm shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100"
        placeholder="{}"
      />
    );
  }

  // Default: text input
  return (
    <input
      type={definition.isSensitive ? "password" : "text"}
      value={displayValue}
      onChange={(e) => handleChange(e.target.value)}
      disabled={isDisabled}
      placeholder={definition.isSensitive ? "••••••••" : undefined}
      className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
    />
  );
}
