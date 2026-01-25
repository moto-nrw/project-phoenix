"use client";

import { useCallback, useEffect, useState } from "react";
import {
  fetchOGSettings,
  resetOGSetting,
  updateOGSetting,
} from "~/lib/settings-api";
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
import { SettingHistory } from "./setting-history";

interface OGSettingsPanelProps {
  ogId: string;
  ogName?: string;
  showHistory?: boolean;
}

export function OGSettingsPanel({
  ogId,
  ogName,
  showHistory = false,
}: OGSettingsPanelProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [settings, setSettings] = useState<ResolvedSetting[]>([]);
  const [categories, setCategories] = useState<SettingCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchOGSettings(ogId);
      setSettings(data);
      setCategories(groupSettingsByCategory(data));
    } catch (error) {
      console.error("Failed to load OG settings:", error);
      toastError("Fehler beim Laden der Einstellungen");
    } finally {
      setLoading(false);
    }
  }, [ogId, toastError]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const handleSettingChange = async (key: string, value: unknown) => {
    setSavingKey(key);
    try {
      await updateOGSetting(ogId, key, value);
      setSettings((prev) =>
        prev.map((s) =>
          s.key === key ? { ...s, value, isDefault: false } : s,
        ),
      );
      toastSuccess("Einstellung gespeichert");
    } catch (error) {
      console.error("Failed to update OG setting:", error);
      toastError("Fehler beim Speichern der Einstellung");
    } finally {
      setSavingKey(null);
    }
  };

  const handleResetSetting = async (key: string) => {
    setSavingKey(key);
    try {
      await resetOGSetting(ogId, key);
      await loadSettings();
      toastSuccess("Einstellung zurückgesetzt");
    } catch (error) {
      console.error("Failed to reset OG setting:", error);
      toastError("Fehler beim Zurücksetzen der Einstellung");
    } finally {
      setSavingKey(null);
    }
  };

  if (loading) {
    return <SettingLoadingSpinner variant="green" />;
  }

  if (categories.length === 0) {
    return (
      <div className="rounded-xl border border-gray-100 bg-green-50/30 p-4 text-center">
        <p className="text-sm text-gray-600">
          Keine Einstellungen für diese Gruppe verfügbar.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {ogName && (
        <h3 className="text-sm font-semibold text-gray-900">
          Einstellungen für {ogName}
        </h3>
      )}

      {categories.map((category) => (
        <div
          key={category.name}
          className="rounded-xl border border-gray-100 bg-green-50/30 p-3 md:p-4"
        >
          <h4 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
            <svg
              className="h-3.5 w-3.5 text-green-600 md:h-4 md:w-4"
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
          </h4>

          <div className="space-y-3">
            {category.groups.map((group) => (
              <div key={group.name} className="space-y-2">
                {group.name !== "_ungrouped" && (
                  <h5 className="text-xs font-medium text-gray-600">
                    {group.name}
                  </h5>
                )}

                {group.settings
                  .filter((setting) => isSettingActive(setting, settings))
                  .map((setting) => (
                    <div
                      key={setting.key}
                      className={`flex flex-col gap-2 rounded-lg bg-white/50 p-2.5 ${!setting.canModify ? "opacity-70" : ""}`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1">
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs font-medium text-gray-800">
                              {setting.description ?? setting.key}
                            </span>
                            {!setting.isDefault && (
                              <span className="rounded-full bg-green-100 px-1.5 py-0.5 text-[10px] text-green-700">
                                Angepasst
                              </span>
                            )}
                          </div>
                          <p className="mt-0.5 text-[10px] text-gray-500">
                            {getSourceLabel(setting)}
                          </p>
                        </div>

                        <SettingInput
                          setting={setting}
                          isSaving={savingKey === setting.key}
                          isDisabled={
                            !setting.canModify || savingKey === setting.key
                          }
                          variant="green"
                          onChange={(key, value) =>
                            void handleSettingChange(key, value)
                          }
                        />
                      </div>

                      {!setting.isDefault && setting.canModify && (
                        <button
                          onClick={() => void handleResetSetting(setting.key)}
                          disabled={savingKey === setting.key}
                          className="self-end text-[10px] text-gray-500 hover:text-green-600"
                        >
                          Zurücksetzen
                        </button>
                      )}
                    </div>
                  ))}
              </div>
            ))}
          </div>
        </div>
      ))}

      {showHistory && (
        <SettingHistory ogId={ogId} limit={10} title="Letzte Änderungen" />
      )}
    </div>
  );
}
