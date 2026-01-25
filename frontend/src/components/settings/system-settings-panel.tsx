"use client";

import { useCallback, useEffect, useState } from "react";
import {
  fetchSystemSettings,
  resetSystemSetting,
  updateSystemSetting,
} from "~/lib/settings-api";
import {
  getCategoryLabel,
  getSourceLabel,
  groupSettingsByCategory,
  type ResolvedSetting,
  type SettingCategory,
} from "~/lib/settings-helpers";
import { useToast } from "~/contexts/ToastContext";
import { SettingHistory } from "./setting-history";

interface SystemSettingsPanelProps {
  isMobile?: boolean;
  showHistory?: boolean;
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

export function SystemSettingsPanel({
  isMobile = false,
  showHistory = true,
}: SystemSettingsPanelProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [settings, setSettings] = useState<ResolvedSetting[]>([]);
  const [categories, setCategories] = useState<SettingCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchSystemSettings();
      setSettings(data);
      setCategories(groupSettingsByCategory(data));
    } catch (error) {
      console.error("Failed to load system settings:", error);
      toastError("Fehler beim Laden der Systemeinstellungen");
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
      await updateSystemSetting(key, value);

      // Update local state
      setSettings((prev) =>
        prev.map((s) =>
          s.key === key ? { ...s, value, isDefault: false } : s,
        ),
      );

      toastSuccess("Systemeinstellung gespeichert");
    } catch (error) {
      console.error("Failed to update system setting:", error);
      toastError("Fehler beim Speichern der Systemeinstellung");
    } finally {
      setSavingKey(null);
    }
  };

  const handleResetSetting = async (key: string) => {
    setSavingKey(key);
    try {
      await resetSystemSetting(key);

      // Reload settings to get default value
      await loadSettings();

      toastSuccess("Einstellung auf Standardwert zurückgesetzt");
    } catch (error) {
      console.error("Failed to reset system setting:", error);
      toastError("Fehler beim Zurücksetzen der Einstellung");
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
              className={`peer h-6 w-11 rounded-full bg-gray-200 peer-checked:bg-purple-600 peer-focus:ring-2 peer-focus:ring-purple-300 peer-disabled:cursor-not-allowed peer-disabled:opacity-50 after:absolute after:top-0.5 after:left-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:after:translate-x-full peer-checked:after:border-white`}
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
              className="w-24 rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-purple-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              className="rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-purple-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              className="rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-purple-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
              className="w-full max-w-xs rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:ring-purple-300 focus:outline-none disabled:bg-gray-50 disabled:text-gray-500"
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
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-purple-600" />
      </div>
    );
  }

  if (categories.length === 0) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 text-center backdrop-blur-sm">
        <p className="text-gray-600">Keine Systemeinstellungen verfügbar.</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Admin Notice */}
      <div className="rounded-xl border border-purple-100 bg-purple-50/50 p-4">
        <div className="flex items-start gap-3">
          <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-purple-100">
            <svg
              className="h-4 w-4 text-purple-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <div>
            <p className="text-sm font-medium text-purple-900">
              Administratorbereich
            </p>
            <p className="mt-0.5 text-xs text-purple-700">
              Änderungen hier wirken sich auf das gesamte System aus. Alle
              Einstellungen werden automatisch gespeichert.
            </p>
          </div>
        </div>
      </div>

      {categories.map((category) => (
        <div
          key={category.name}
          className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6"
        >
          <h3 className="mb-4 flex items-center gap-2 text-base font-semibold text-gray-900">
            <svg
              className="h-4 w-4 text-purple-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
              />
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
            </svg>
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
                      className={`rounded-lg bg-gray-50/50 p-3 transition-colors ${!setting.canModify ? "opacity-70" : ""}`}
                    >
                      <div
                        className={`flex ${isMobile ? "flex-col gap-2" : "items-center justify-between"}`}
                      >
                        <div className={`${isMobile ? "" : "flex-1"}`}>
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-sm font-medium text-gray-800">
                              {setting.description ?? setting.key}
                            </span>
                            {!setting.isDefault && (
                              <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs text-purple-700">
                                Angepasst
                              </span>
                            )}
                          </div>
                          <p className="mt-0.5 text-xs text-gray-500">
                            {getSourceLabel(setting)}
                          </p>
                          <p className="mt-1 font-mono text-[10px] text-gray-400">
                            {setting.key}
                          </p>
                        </div>

                        <div className={isMobile ? "self-start" : ""}>
                          {renderSettingInput(setting)}
                        </div>
                      </div>

                      {!setting.isDefault && setting.canModify && (
                        <div className="mt-2 border-t border-gray-100 pt-2">
                          <button
                            onClick={() => void handleResetSetting(setting.key)}
                            disabled={savingKey === setting.key}
                            className="text-xs text-gray-500 hover:text-purple-600 disabled:opacity-50"
                          >
                            Auf Standardwert zurücksetzen
                          </button>
                        </div>
                      )}
                    </div>
                  ))}
              </div>
            ))}
          </div>
        </div>
      ))}

      {showHistory && (
        <SettingHistory
          filters={{ scopeType: "system" }}
          limit={20}
          title="Letzte Systemänderungen"
        />
      )}
    </div>
  );
}
