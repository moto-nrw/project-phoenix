"use client";

import { useCallback, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { createLogger } from "~/lib/logger";
import {
  applyOptimisticSchemaUpdate,
  setSettingValue,
  resetSettingValue,
} from "~/lib/settings-api";
import { notifySettingsChanged } from "~/lib/settings-broadcast";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";
import type { SettingsSchema, SchemaTab } from "~/lib/settings-api";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import { PersonalizationTab } from "./personalization-tab";
import { EnrollmentLinkPanel } from "./enrollment-link-panel";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useTenantMutate } from "~/lib/swr/hooks";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";

// Settings whose value affects the supervision context (sidebar / mobile nav)
// and therefore require an immediate re-fetch after save/reset instead of
// waiting for the next navigation or re-login.
const SUPERVISION_AFFECTING_KEYS = new Set<string>([
  "operations.admin_supervision_overview",
]);

const logger = createLogger({ component: "SettingsPage" });

interface SettingsTabContentProps {
  readonly tab: SchemaTab;
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  readonly onSchemaRefresh: () => void;
}

function SettingsTabContent({
  tab,
  highlightKey,
  onSave,
  onReset,
  onSchemaRefresh,
}: SettingsTabContentProps) {
  return (
    <div className="space-y-6">
      {tab.key === "enrollment" && <EnrollmentLinkPanel tab={tab} />}
      {tab.categories.map((category) => (
        <SettingsCategory
          key={category.key}
          category={category}
          highlightKey={highlightKey}
          onSave={onSave}
          onReset={onReset}
          onSchemaRefresh={onSchemaRefresh}
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
          className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6"
        >
          <Skeleton className="mb-4 h-5 w-32 rounded" />
          <div className="divide-y divide-gray-100">
            {Array.from({ length: catIdx === 0 ? 3 : 2 }).map((_, i) => (
              <div
                key={i}
                className="flex flex-col gap-3 py-4 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
              >
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-4 w-3/4 rounded sm:w-48" />
                  <Skeleton className="h-3.5 w-full rounded sm:w-72" />
                </div>
                <Skeleton className="h-10 w-24 rounded-lg sm:w-32" />
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
  readonly highlightKey?: string | null;
}

function SettingsContent({ tabKey, highlightKey }: SettingsContentProps) {
  const { refresh: refreshSupervision } = useOptionalSupervision();
  const router = useRouter();
  const tenantMutate = useTenantMutate();
  const {
    data: schema,
    error: fetchError,
    isLoading,
    mutate: revalidate,
  } = useSettingsSchema();
  const [saveError, setSaveError] = useState<string | null>(null);

  // Reminders settings decide whether the header reminders bell shows at all.
  // After saving/resetting one, revalidate THIS tenant's /api/reminders cache
  // so the bell's `enabled` flag flips immediately instead of waiting for the
  // poll. The reminders SWR key is tenant-prefixed ("{slug}:reminders");
  // useTenantMutate applies that prefix, so a different tenant's cache held in
  // another tab is left untouched (no cross-tenant revalidation).
  const revalidateRemindersIfNeeded = useCallback(
    (key: string) => {
      if (!key.startsWith("reminders.")) return;
      void tenantMutate("reminders");
    },
    [tenantMutate],
  );

  const applyOptimistic = useCallback(
    (key: string, value: unknown) => {
      void revalidate(
        (current?: SettingsSchema | null) =>
          current ? applyOptimisticSchemaUpdate(current, key, value) : current,
        { revalidate: true },
      );
    },
    [revalidate],
  );

  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<string | null> => {
      const errorMsg = await setSettingValue(key, value);
      if (errorMsg) {
        if (
          errorMsg.startsWith("Netzwerkfehler") ||
          errorMsg.startsWith("Einstellung konnte nicht")
        ) {
          setSaveError(errorMsg);
        }
        return errorMsg;
      }
      setSaveError(null);
      logger.info("setting_value_saved", { key });
      // Tenant-resolve-affecting keys: refresh the RSC tree so the cached
      // layout picks up the new value (BroadcastChannel only reaches OTHER
      // tabs).
      if (TENANT_RESOLVE_AFFECTING_KEYS.has(key)) {
        router.refresh();
      }
      notifySettingsChanged();
      applyOptimistic(key, value);
      if (SUPERVISION_AFFECTING_KEYS.has(key)) {
        void refreshSupervision({ force: true });
      }
      revalidateRemindersIfNeeded(key);
      return null;
    },
    [applyOptimistic, refreshSupervision, revalidateRemindersIfNeeded, router],
  );

  const handleReset = useCallback(
    async (key: string): Promise<string | null> => {
      const errorMsg = await resetSettingValue(key);
      if (errorMsg) {
        setSaveError(errorMsg);
        return errorMsg;
      }
      setSaveError(null);
      logger.info("setting_value_reset", { key });
      if (TENANT_RESOLVE_AFFECTING_KEYS.has(key)) {
        router.refresh();
      }
      notifySettingsChanged();
      // Reset has no optimistic value — bridge mutate() picks up the
      // registry default on revalidation.
      void revalidate();
      if (SUPERVISION_AFFECTING_KEYS.has(key)) {
        void refreshSupervision({ force: true });
      }
      revalidateRemindersIfNeeded(key);
      return null;
    },
    [refreshSupervision, revalidate, revalidateRemindersIfNeeded, router],
  );

  const handleSchemaRefresh = useCallback(() => {
    notifySettingsChanged();
    void revalidate();
  }, [revalidate]);

  if (isLoading && !schema) {
    return <SettingsSkeleton />;
  }

  // Server error on initial fetch — show retry.
  if (fetchError && !schema) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-12 text-center">
        <p className="text-sm text-[#CC2626]">
          {fetchError instanceof Error
            ? fetchError.message
            : "Einstellungen konnten nicht geladen werden"}
        </p>
        <button
          type="button"
          onClick={() => void revalidate()}
          className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700"
        >
          Erneut versuchen
        </button>
      </div>
    );
  }

  // No access (null from 401/403) — render nothing, tabs won't show.
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
      {saveError && (
        <div className="relative mb-4">
          <Alert type="error" message={saveError} />
          <button
            type="button"
            onClick={() => setSaveError(null)}
            className="absolute top-1/2 right-4 -translate-y-1/2 text-[#CC2626] hover:text-[#9F1F1E]"
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
      <SettingsTabContent
        tab={tab}
        highlightKey={highlightKey}
        onSave={handleSave}
        onReset={handleReset}
        onSchemaRefresh={handleSchemaRefresh}
      />
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
  const searchParams = useSearchParams();
  const { data: schema, error: schemaError, isLoading } = useSettingsSchema();

  if (isLoading) {
    return null;
  }

  // Tab label mapping (German)
  const tabLabels: Record<string, string> = {
    operations: "Betrieb",
    reminders: "Erinnerungen",
    gdpr: "Datenschutz",
    devices: "Geräte",
    enrollment: "Anmeldung",
    system: "System",
    general: "Allgemein",
    security: "Sicherheit",
  };

  // Tab icon mapping (SVG paths for lucide-style icons)
  const defaultTabIcon =
    "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4";
  const tabIcons: Record<string, string> = {
    operations:
      "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z",
    reminders:
      "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9",
    gdpr: "M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z",
    devices:
      "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
    enrollment:
      "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4",
    system: "M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z",
    general:
      "M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4",
    security:
      "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
  };

  // When the schema fetch failed, render placeholder tabs so SettingsContent
  // mounts and can show its own retry UI instead of silently dropping all
  // schema tabs.
  const fallbackTabKeys = ["operations", "gdpr", "devices", "system"];
  const schemaTabs = schemaError
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
  const highlightKey = searchParams.get("highlight");

  const renderTab = (tabId: string) => {
    if (tabId === "settings-personalisierung") {
      return <PersonalizationTab />;
    }
    const settingsKey = tabId.replace("settings-", "");
    return <SettingsContent tabKey={settingsKey} highlightKey={highlightKey} />;
  };

  return { tabs, renderTab };
}
