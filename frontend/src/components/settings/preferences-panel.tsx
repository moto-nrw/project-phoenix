"use client";

import { useCallback, useEffect, useState } from "react";
import { fetchUserSettings, updateUserSetting } from "~/lib/settings-api";
import {
  getCategoryLabel,
  getSourceLabel,
  groupSettingsByCategory,
  isSettingActive,
  type ResolvedSetting,
  type SettingCategory,
} from "~/lib/settings-helpers";
import { useToast } from "~/contexts/ToastContext";
import { SettingInput, SettingLoadingSpinner } from "./setting-input";

interface PreferencesPanelProps {
  isMobile?: boolean;
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

  if (loading) {
    return <SettingLoadingSpinner variant="gray" />;
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
                  .filter((setting) => isSettingActive(setting, settings))
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
                        <SettingInput
                          setting={setting}
                          isSaving={savingKey === setting.key}
                          isDisabled={
                            !setting.canModify || savingKey === setting.key
                          }
                          variant="gray"
                          onChange={(key, value) =>
                            void handleSettingChange(key, value)
                          }
                        />
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
