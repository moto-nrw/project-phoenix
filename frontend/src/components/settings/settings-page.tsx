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
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import { PersonalizationTab } from "./personalization-tab";
import { EnrollmentLinkPanel } from "./enrollment-link-panel";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useTenantMutate } from "~/lib/swr/hooks";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import type { MotoConceptKey } from "~/lib/moto-concepts";

// Settings whose value affects the supervision context (sidebar / mobile nav)
// and therefore require an immediate re-fetch after save/reset instead of
// waiting for the next navigation or re-login.
const SUPERVISION_AFFECTING_KEYS = new Set<string>([
  "operations.operational_overview_scope",
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
      <Alert
        type="error"
        message={
          fetchError instanceof Error
            ? fetchError.message
            : "Einstellungen konnten nicht geladen werden"
        }
        action={
          <Button
            type="button"
            variant="surface"
            size="md"
            onClick={() => void revalidate()}
          >
            Erneut versuchen
          </Button>
        }
      />
    );
  }

  // No access (null from 401/403) — render nothing, tabs won't show.
  if (!schema) {
    return null;
  }

  const tab = schema.tabs?.find((t) => t.key === tabKey);

  if (!tab) {
    return (
      <EmptyState
        title="Keine Einstellungen verfügbar."
        description="Für diesen Bereich sind derzeit keine Einstellungen freigeschaltet."
      />
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
            className="text-moto-red hover:text-moto-red-strong absolute top-1/2 right-4 -translate-y-1/2"
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
  tabs: { id: string; label: string; icon: MotoConceptKey }[];
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

  // Tab icon mapping (MOTO-Konzepte statt SVG-Pfaden)
  const defaultTabConcept: MotoConceptKey = "settings";
  const tabConcepts: Record<string, MotoConceptKey> = {
    operations: "settings",
    reminders: "notifications",
    notifications: "notifications",
    gdpr: "permissions",
    devices: "devices",
    enrollment: "enrollments",
    system: "settings",
    general: "settings",
    security: "permissions",
  };

  // When the schema fetch failed, render placeholder tabs so SettingsContent
  // mounts and can show its own retry UI instead of silently dropping all
  // schema tabs.
  const fallbackTabKeys = ["operations", "gdpr", "devices", "system"];
  const schemaTabs = schemaError
    ? fallbackTabKeys.map((key) => ({
        id: `settings-${key}`,
        label: tabLabels[key] ?? key,
        icon: tabConcepts[key] ?? defaultTabConcept,
      }))
    : (schema?.tabs ?? [])
        // Payroll settings (#1417) have their own maintenance page under
        // /payroll — rendering the auto-generated tab here would create a
        // second, worse surface for the same values.
        .filter((tab) => tab.key !== "abrechnung")
        .map((tab) => ({
          id: `settings-${tab.key}`,
          label: tabLabels[tab.key] ?? tab.label,
          icon: tabConcepts[tab.key] ?? defaultTabConcept,
        }));

  // Personalisierung is always available (permission-gated inside the component)
  const personalizationTab: {
    id: string;
    label: string;
    icon: MotoConceptKey;
  } = {
    id: "settings-personalisierung",
    label: "Personalisierung",
    icon: "settings",
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
