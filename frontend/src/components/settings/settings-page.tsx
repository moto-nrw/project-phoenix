"use client";

import { useState, useEffect, useCallback } from "react";
import { createLogger } from "~/lib/logger";
import {
  fetchSettingsSchema,
  setSettingValue,
  resetSettingValue,
} from "~/lib/settings-api";
import type { SettingsSchema, SchemaTab } from "~/lib/settings-api";
import { SettingsCategory } from "./settings-category";

const logger = createLogger({ component: "SettingsPage" });

interface SettingsTabContentProps {
  readonly tab: SchemaTab;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
}

function SettingsTabContent({ tab, onSave, onReset }: SettingsTabContentProps) {
  return (
    <div className="space-y-6">
      {tab.categories.map((category) => (
        <SettingsCategory
          key={category.key}
          category={category}
          onSave={onSave}
          onReset={onReset}
        />
      ))}
    </div>
  );
}

interface SettingsContentProps {
  readonly tabKey: string;
}

function SettingsContent({ tabKey }: SettingsContentProps) {
  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadSchema = useCallback(async () => {
    setLoading(true);
    setError(null);
    const data = await fetchSettingsSchema();
    setSchema(data);
    setLoading(false);
  }, []);

  useEffect(() => {
    void loadSchema();
  }, [loadSchema]);

  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<string | null> => {
      const errorMsg = await setSettingValue(key, value);
      if (errorMsg) {
        if (
          errorMsg.startsWith("Netzwerkfehler") ||
          errorMsg.startsWith("Einstellung konnte nicht")
        ) {
          setError(errorMsg);
        }
      } else {
        setError(null);
        logger.info("setting_value_saved", { key });
        // Update schema state locally so values persist across tab switches
        // and field components aren't remounted (preserves green border).
        setSchema((prev) => {
          if (!prev) return prev;
          // Build a value map for DependsOn evaluation
          const valueMap = new Map<string, unknown>();
          for (const tab of prev.tabs) {
            for (const cat of tab.categories) {
              for (const item of cat.items) {
                valueMap.set(item.key, item.key === key ? value : item.value);
              }
            }
          }
          return {
            ...prev,
            tabs: prev.tabs.map((tab) => ({
              ...tab,
              categories: tab.categories.map((cat) => ({
                ...cat,
                items: cat.items.map((item) => {
                  const updated =
                    item.key === key
                      ? { ...item, value, is_default: false }
                      : item;
                  // Re-evaluate DependsOn visibility
                  if (updated.depends_on) {
                    const parentVal = valueMap.get(updated.depends_on.key);
                    const cond = updated.depends_on.condition;
                    const expected = updated.depends_on.value;
                    let visible = true;
                    if (cond === "eq")
                      visible =
                        JSON.stringify(parentVal) === JSON.stringify(expected);
                    if (cond === "neq")
                      visible =
                        JSON.stringify(parentVal) !== JSON.stringify(expected);
                    if (cond === "not_empty")
                      visible = parentVal != null && parentVal !== "";
                    return { ...updated, visible };
                  }
                  return updated;
                }),
              })),
            })),
          };
        });
      }
      return errorMsg;
    },
    [],
  );

  const handleReset = useCallback(
    async (key: string): Promise<string | null> => {
      const errorMsg = await resetSettingValue(key);
      if (errorMsg) {
        setError(errorMsg);
      } else {
        setError(null);
        await loadSchema();
        logger.info("setting_value_reset", { key });
      }
      return errorMsg;
    },
    [loadSchema],
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-gray-200 border-t-gray-900" />
      </div>
    );
  }

  // No access (null from 401/403) — render nothing, tabs won't show
  if (!schema) {
    return null;
  }

  const tab = schema.tabs?.find((t) => t.key === tabKey);

  if (!tab) {
    return (
      <div className="py-8 text-center text-sm text-gray-500">
        Keine Einstellungen verfügbar.
      </div>
    );
  }

  return (
    <>
      {error && (
        <div className="mb-4 flex items-center justify-between rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <span>{error}</span>
          <button
            onClick={() => setError(null)}
            className="ml-3 font-medium text-red-500 hover:text-red-700"
          >
            &times;
          </button>
        </div>
      )}
      <SettingsTabContent tab={tab} onSave={handleSave} onReset={handleReset} />
    </>
  );
}

/**
 * Returns the tab definitions for injecting into SettingsLayout's extraTabs.
 * Each tab renders a SettingsContent component for its key.
 * Returns null silently if user has no access or schema is empty.
 */
export function useSettingsTabs(): {
  tabs: { id: string; label: string; icon: string }[];
  renderTab: (tabId: string) => React.ReactNode;
} | null {
  const [schema, setSchema] = useState<SettingsSchema | null>(null);

  useEffect(() => {
    void fetchSettingsSchema().then((data) => {
      if (data) setSchema(data);
    });
  }, []);

  if (!schema?.tabs || schema.tabs.length === 0) {
    return null;
  }

  // Tab label mapping (German)
  const tabLabels: Record<string, string> = {
    operations: "Betrieb",
    gdpr: "Datenschutz",
    security: "Sicherheit",
    general: "Allgemein",
  };

  // Tab icon mapping (SVG paths for lucide-style icons)
  const defaultTabIcon =
    "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4";
  const tabIcons: Record<string, string> = {
    operations:
      "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
    gdpr: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
    security:
      "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
    general:
      "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4",
  };

  const tabs = schema.tabs.map((tab) => ({
    id: `settings-${tab.key}`,
    label: tabLabels[tab.key] ?? tab.label,
    icon: tabIcons[tab.key] ?? defaultTabIcon,
  }));

  const renderTab = (tabId: string) => {
    const settingsKey = tabId.replace("settings-", "");
    return <SettingsContent tabKey={settingsKey} />;
  };

  return { tabs, renderTab };
}
