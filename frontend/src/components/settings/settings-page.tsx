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
  readonly onSave: (key: string, value: unknown) => void;
  readonly onReset: (key: string) => void;
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

export function SettingsContent({ tabKey }: SettingsContentProps) {
  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<string | null>(null);

  const loadSchema = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await fetchSettingsSchema();
      setSchema(data);
    } catch (err) {
      logger.error("load_settings_schema_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Einstellungen konnten nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSchema();
  }, [loadSchema]);

  const handleSave = useCallback(
    async (key: string, value: unknown) => {
      try {
        setSaving(key);
        await setSettingValue(key, value);
        // Re-fetch schema to get updated values + re-evaluate dependencies
        await loadSchema();
        logger.info("setting_value_saved", { key });
      } catch (err) {
        logger.error("save_setting_value_failed", {
          key,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Einstellung konnte nicht gespeichert werden.");
      } finally {
        setSaving(null);
      }
    },
    [loadSchema],
  );

  const handleReset = useCallback(
    async (key: string) => {
      try {
        setSaving(key);
        await resetSettingValue(key);
        // Re-fetch schema after reset (review fix #6: dependencies may change)
        await loadSchema();
        logger.info("setting_value_reset", { key });
      } catch (err) {
        logger.error("reset_setting_value_failed", {
          key,
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Einstellung konnte nicht zurückgesetzt werden.");
      } finally {
        setSaving(null);
      }
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

  if (error && !schema) {
    return (
      <div className="rounded-2xl border border-red-100 bg-red-50 p-6 text-sm text-red-600">
        {error}
      </div>
    );
  }

  const tab = schema?.tabs.find((t) => t.key === tabKey);

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
        <div className="mb-4 rounded-lg border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-600">
          {error}
          <button
            onClick={() => setError(null)}
            className="ml-2 font-medium underline"
          >
            Schließen
          </button>
        </div>
      )}
      {saving && (
        <div className="mb-4 rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-600">
          Speichern...
        </div>
      )}
      <SettingsTabContent tab={tab} onSave={handleSave} onReset={handleReset} />
    </>
  );
}

/**
 * Returns the tab definitions for injecting into SettingsLayout's extraTabs.
 * Each tab renders a SettingsContent component for its key.
 */
export function useSettingsTabs(): {
  tabs: { id: string; label: string; icon: string }[];
  renderTab: (tabId: string) => React.ReactNode;
} | null {
  const [schema, setSchema] = useState<SettingsSchema | null>(null);

  useEffect(() => {
    fetchSettingsSchema()
      .then(setSchema)
      .catch((err) => {
        logger.error("load_settings_tabs_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, []);

  if (!schema || schema.tabs.length === 0) {
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
