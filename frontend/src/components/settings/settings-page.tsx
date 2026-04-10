"use client";

import { useState, useEffect, useCallback } from "react";
import { useSession } from "next-auth/react";
import { createLogger } from "~/lib/logger";
import {
  fetchSettingsSchema,
  setSettingValue,
  resetSettingValue,
} from "~/lib/settings-api";
import type { SettingsSchema, SchemaTab } from "~/lib/settings-api";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import { PersonalizationTab } from "./personalization-tab";

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

function SettingsSkeleton() {
  return (
    <div className="space-y-6">
      {Array.from({ length: 2 }).map((_, catIdx) => (
        <div
          key={catIdx}
          className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm"
        >
          <Skeleton className="mb-4 h-5 w-32 rounded" />
          <div className="divide-y divide-gray-100">
            {Array.from({ length: catIdx === 0 ? 3 : 2 }).map((_, i) => (
              <div
                key={i}
                className="flex items-start justify-between gap-4 py-4"
              >
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-48 rounded" />
                  <Skeleton className="h-3.5 w-72 rounded" />
                </div>
                <Skeleton className="h-9 w-32 rounded-lg" />
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

interface SettingsContentProps {
  readonly tabKey: string;
}

function SettingsContent({ tabKey }: SettingsContentProps) {
  const { status: sessionStatus } = useSession();
  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadSchema = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchSettingsSchema();
      setSchema(data);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Einstellungen konnten nicht geladen werden",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (sessionStatus === "authenticated") {
      void loadSchema();
    }
  }, [loadSchema, sessionStatus]);

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
        // Update schema state locally so values persist across tab switches.
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
                  const optimisticValue =
                    item.type === "password" ? "••••••" : value;
                  const updated =
                    item.key === key
                      ? { ...item, value: optimisticValue, is_default: false }
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

        // Background sync: silently re-fetch to pick up server-side changes.
        // Only updates state if the server data actually differs from local state.
        setTimeout(() => {
          void fetchSettingsSchema().then((fresh) => {
            if (!fresh) return;
            setSchema((prev) => {
              // Skip update if values are identical (prevents unnecessary re-render)
              if (JSON.stringify(prev) === JSON.stringify(fresh)) return prev;
              return fresh;
            });
          });
        }, 6000);
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
        logger.info("setting_value_reset", { key });

        // Background sync: re-fetch schema to get the default value
        // without blocking the UI or showing a loading spinner.
        setTimeout(() => {
          void fetchSettingsSchema().then((fresh) => {
            if (!fresh) return;
            setSchema((prev) => {
              if (JSON.stringify(prev) === JSON.stringify(fresh)) return prev;
              return fresh;
            });
          });
        }, 500);
      }
      return errorMsg;
    },
    [],
  );

  if (loading) {
    return <SettingsSkeleton />;
  }

  // Server error — show error message with retry
  if (error && !schema) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-12 text-center">
        <p className="text-sm text-red-600">{error}</p>
        <button
          type="button"
          onClick={() => void loadSchema()}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700"
        >
          Erneut versuchen
        </button>
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
        <div className="relative mb-4">
          <Alert type="error" message={error} />
          <button
            onClick={() => setError(null)}
            className="absolute top-1/2 right-4 -translate-y-1/2 text-red-600 hover:text-red-800"
            aria-label="Fehler schließen"
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
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
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
  const { status: sessionStatus } = useSession();
  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [schemaFetchFailed, setSchemaFetchFailed] = useState(false);
  const [schemaLoaded, setSchemaLoaded] = useState(false);

  useEffect(() => {
    if (sessionStatus === "authenticated") {
      void fetchSettingsSchema()
        .then((data) => {
          if (data) setSchema(data);
        })
        .catch(() => {
          // Schema fetch failed — mark so we still render placeholder tabs.
          // Inner SettingsContent has its own fetch with error display and retry button.
          setSchemaFetchFailed(true);
        })
        .finally(() => setSchemaLoaded(true));
    }
  }, [sessionStatus]);

  if (!schemaLoaded) {
    return null;
  }

  // Tab label mapping (German)
  const tabLabels: Record<string, string> = {
    operations: "Betrieb",
    gdpr: "Datenschutz",
    security: "Sicherheit",
    devices: "Geräte",
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
    devices:
      "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
    general:
      "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4",
  };

  // Schema-driven tabs (may be empty if user has no config:read permission).
  // When the schema fetch failed, render placeholder tabs so SettingsContent
  // mounts and can show its own error/retry UI instead of silently dropping
  // all schema tabs.
  const fallbackTabKeys = ["operations", "gdpr", "security"];
  const schemaTabs = schemaFetchFailed
    ? fallbackTabKeys.map((key) => ({
        id: `settings-${key}`,
        label: tabLabels[key] ?? key,
        icon: tabIcons[key] ?? defaultTabIcon,
      }))
    : (schema?.tabs ?? []).map((tab) => ({
        id: `settings-${tab.key}`,
        label: tabLabels[tab.key] ?? tab.label,
        icon: tabIcons[tab.key] ?? defaultTabIcon,
      }));

  // Personalisierung is always available (permission-gated inside the component)
  const personalizationTab = {
    id: "settings-personalisierung",
    label: "Personalisierung",
    icon: "M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z",
  };

  const tabs = [...schemaTabs, personalizationTab];

  const renderTab = (tabId: string) => {
    if (tabId === "settings-personalisierung") {
      return <PersonalizationTab />;
    }
    const settingsKey = tabId.replace("settings-", "");
    return <SettingsContent tabKey={settingsKey} />;
  };

  return { tabs, renderTab };
}
