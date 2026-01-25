"use client";

import { useCallback, useEffect, useState } from "react";
import { fetchUserSettings, updateUserSetting } from "~/lib/settings-api";
import {
  getCategoryLabel,
  getSourceLabel,
  groupSettingsByCategory,
  type ResolvedSetting,
  type SettingCategory,
} from "~/lib/settings-helpers";
import { useToast } from "~/contexts/ToastContext";

interface PreferencesPanelProps {
  isMobile?: boolean;
}

/**
 * Safely convert a setting value to string for input fields
 */
function valueToString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return "";
}

export function PreferencesPanel({ isMobile = false }: PreferencesPanelProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [settings, setSettings] = useState<ResolvedSetting[]>([]);
  const [categories, setCategories] = useState<SettingCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchUserSettings();
      setSettings(data);
      setCategories(groupSettingsByCategory(data));
    } catch (error) {
      console.error("Failed to load settings:", error);
      toastError("Fehler beim Laden der Einstellungen");
    } finally {
      setLoading(false);
    }
  }, [toastError]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const handleSettingChange = async (key: string, value: unknown) => {
    setSavingKey(key);
    try {
      await updateUserSetting(key, value);

      // Update local state
      setSettings((prev) =>
        prev.map((s) =>
          s.key === key ? { ...s, value, isDefault: false } : s,
        ),
      );

      toastSuccess("Einstellung gespeichert");
    } catch (error) {
      console.error("Failed to update setting:", error);
      toastError("Fehler beim Speichern der Einstellung");
    } finally {
      setSavingKey(null);
    }
  };

  // Check if a setting should be shown based on dependencies
  const isSettingActive = (setting: ResolvedSetting): boolean => {
    if (!setting.dependsOn) return true;

    const dependentSetting = settings.find(
      (s) => s.key === setting.dependsOn?.key,
    );
    if (!dependentSetting) return true;

    const { condition, value: expectedValue } = setting.dependsOn;
    const actualValue = dependentSetting.value;

    switch (condition) {
      case "equals":
        return actualValue === expectedValue;
      case "not_equals":
        return actualValue !== expectedValue;
      case "in":
        return (
          Array.isArray(expectedValue) && expectedValue.includes(actualValue)
        );
      case "not_empty":
        return (
          actualValue !== null &&
          actualValue !== undefined &&
          actualValue !== ""
        );
      default:
        return true;
    }
  };

  const renderSettingInput = (setting: ResolvedSetting) => {
    const isDisabled = !setting.canModify || savingKey === setting.key;
    const isSaving = savingKey === setting.key;

    switch (setting.type) {
      case "bool":
        return (
          <label className="relative inline-flex cursor-pointer items-center">
            <input
              type="checkbox"
              checked={Boolean(setting.value)}
              onChange={(e) =>
                void handleSettingChange(setting.key, e.target.checked)
              }
              disabled={isDisabled}
              className="peer sr-only"
            />
            <div
              className={`peer h-6 w-11 rounded-full bg-gray-200 peer-checked:bg-gray-900 peer-focus:ring-2 peer-focus:ring-gray-300 peer-disabled:cursor-not-allowed peer-disabled:opacity-50 after:absolute after:top-0.5 after:left-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white`}
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
                  void handleSettingChange(setting.key, val);
                }
              }}
              disabled={isDisabled}
              min={setting.validation?.min}
              max={setting.validation?.max}
              className="w-24 rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-gray-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              onChange={(e) =>
                void handleSettingChange(setting.key, e.target.value)
              }
              disabled={isDisabled}
              className="rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-gray-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              onChange={(e) =>
                void handleSettingChange(setting.key, e.target.value)
              }
              disabled={isDisabled}
              className="rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-gray-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              onChange={(e) =>
                void handleSettingChange(setting.key, e.target.value)
              }
              disabled={isDisabled}
              pattern={setting.validation?.pattern}
              className="w-full max-w-xs rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-gray-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
            />
            {isSaving && (
              <span className="text-xs text-gray-500">Speichern...</span>
            )}
          </div>
        );
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-gray-900" />
      </div>
    );
  }

  if (categories.length === 0) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 text-center backdrop-blur-sm">
        <p className="text-gray-600">Keine Einstellungen verfügbar.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {categories.map((category) => (
        <div
          key={category.name}
          className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6"
        >
          <h3 className="mb-4 text-base font-semibold text-gray-900">
            {getCategoryLabel(category.name)}
          </h3>

          <div className="space-y-4">
            {category.groups.map((group) => (
              <div key={group.name} className="space-y-3">
                {group.name !== "_ungrouped" && (
                  <h4 className="text-sm font-medium text-gray-700">
                    {group.name}
                  </h4>
                )}

                {group.settings
                  .filter((setting) => isSettingActive(setting))
                  .map((setting) => (
                    <div
                      key={setting.key}
                      className={`flex ${isMobile ? "flex-col gap-2" : "items-center justify-between"} rounded-lg bg-gray-50/50 p-3 transition-colors ${!setting.canModify ? "opacity-70" : ""}`}
                    >
                      <div className={`${isMobile ? "" : "flex-1"}`}>
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-gray-800">
                            {setting.description ?? setting.key}
                          </span>
                          {!setting.isDefault && (
                            <span className="rounded-full bg-gray-200 px-2 py-0.5 text-xs text-gray-600">
                              Angepasst
                            </span>
                          )}
                        </div>
                        <p className="mt-0.5 text-xs text-gray-500">
                          {getSourceLabel(setting)}
                        </p>
                      </div>

                      <div className={isMobile ? "self-start" : ""}>
                        {renderSettingInput(setting)}
                      </div>
                    </div>
                  ))}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
