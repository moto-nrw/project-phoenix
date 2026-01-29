"use client";

import { useEffect, useState } from "react";
import type { ObjectRefOption, ResolvedSetting } from "~/lib/settings-helpers";
import { clientFetchObjectRefOptions } from "~/lib/settings-api";

interface ObjectRefSelectProps {
  setting: ResolvedSetting;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  deviceId?: string;
}

export function ObjectRefSelect({
  setting,
  value,
  onChange,
  disabled = false,
  deviceId,
}: ObjectRefSelectProps) {
  const [options, setOptions] = useState<ObjectRefOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;

    async function loadOptions() {
      setLoading(true);
      setError(null);

      try {
        const fetchedOptions = await clientFetchObjectRefOptions(
          setting.key,
          deviceId,
        );
        if (isMounted) {
          setOptions(fetchedOptions);
        }
      } catch (err) {
        if (isMounted) {
          setError("Fehler beim Laden der Optionen");
          console.error("Error loading object ref options:", err);
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    }

    void loadOptions();

    return () => {
      isMounted = false;
    };
  }, [setting.key, deviceId]);

  if (loading) {
    return (
      <select
        disabled
        className="block w-full rounded-md border-gray-300 bg-gray-100 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
      >
        <option>Laden...</option>
      </select>
    );
  }

  if (error) {
    return (
      <div className="text-sm text-red-600">{error}</div>
    );
  }

  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      className="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 disabled:cursor-not-allowed disabled:bg-gray-100 sm:text-sm"
    >
      <option value="">-- Auswählen --</option>
      {options.map((option) => (
        <option key={option.id} value={option.id}>
          {option.name}
          {option.extra &&
            Object.keys(option.extra).length > 0 &&
            ` (${Object.values(option.extra).join(", ")})`}
        </option>
      ))}
    </select>
  );
}
