"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search, SearchX } from "lucide-react";
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
import { Input } from "~/components/ui/input";
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import {
  normalizeQuery,
  searchTabs,
  visibleCategoryItems,
} from "./settings-filter";
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

// Tab label mapping (German)
const TAB_LABELS: Record<string, string> = {
  operations: "Betrieb",
  reminders: "Erinnerungen",
  gdpr: "Datenschutz",
  devices: "Geräte",
  enrollment: "Anmeldung",
  system: "System",
  general: "Allgemein",
  security: "Sicherheit",
};

function tabLabel(tab: SchemaTab): string {
  return TAB_LABELS[tab.key] ?? tab.label;
}

// Payroll settings (#1417) have their own maintenance page under /payroll —
// rendering the auto-generated tab here would create a second, worse surface
// for the same values. Search skips it for the same reason.
function schemaTabsForPage(schema: SettingsSchema | null | undefined) {
  return (schema?.tabs ?? []).filter((tab) => tab.key !== "abrechnung");
}

interface SettingsTabContentProps {
  readonly tab: SchemaTab;
  /** Every tab of the page; the search box looks across all of them. */
  readonly allTabs: readonly SchemaTab[];
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  readonly onSchemaRefresh: () => void;
}

function expansionKey(tabKey: string, categoryKey: string): string {
  return `${tabKey}:${categoryKey}`;
}

/**
 * One settings tab (#2830): categories start collapsed and show their name
 * plus a one-line summary of what they contain; a person opens the one they
 * need, or expands all. A deep link (`?highlight=<key>`) opens the category
 * that holds the setting. The search box filters across every tab, because
 * nobody knows in advance under which tab a setting lives.
 */
function SettingsTabContent({
  tab,
  allTabs,
  highlightKey,
  onSave,
  onReset,
  onSchemaRefresh,
}: SettingsTabContentProps) {
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(
    () => new Set<string>(),
  );
  // The deep-linked category is expanded once per (tab, key); a later schema
  // revalidation must not re-open it after the person collapsed it.
  const handledHighlightRef = useRef<string | null>(null);

  useEffect(() => {
    if (!highlightKey) return;
    const owner = tab.categories.find((category) =>
      category.items.some((item) => item.key === highlightKey),
    );
    if (!owner) return;
    const key = expansionKey(tab.key, owner.key);
    const marker = `${key}:${highlightKey}`;
    if (handledHighlightRef.current === marker) return;
    handledHighlightRef.current = marker;
    setExpanded((prev) => {
      if (prev.has(key)) return prev;
      const next = new Set(prev);
      next.add(key);
      return next;
    });
  }, [highlightKey, tab]);

  const normalizedQuery = normalizeQuery(query);
  const isFiltering = normalizedQuery !== "";
  const hits = isFiltering ? searchTabs(allTabs, normalizedQuery) : [];
  const hitCount = hits.reduce((sum, hit) => sum + hit.items.length, 0);

  const visibleCategories = tab.categories.filter(
    (category) => visibleCategoryItems(category).length > 0,
  );
  const allExpanded =
    visibleCategories.length > 0 &&
    visibleCategories.every((category) =>
      expanded.has(expansionKey(tab.key, category.key)),
    );

  const toggleCategory = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const setAllExpanded = (open: boolean) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      for (const category of visibleCategories) {
        const key = expansionKey(tab.key, category.key);
        if (open) {
          next.add(key);
        } else {
          next.delete(key);
        }
      }
      return next;
    });
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative w-full sm:max-w-sm">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 z-10 h-4 w-4 -translate-y-1/2 text-gray-400"
            aria-hidden="true"
          />
          <Input
            type="search"
            controlSize="compact"
            className="pl-9"
            placeholder="Einstellung suchen"
            aria-label="Einstellung suchen"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        {isFiltering ? (
          <p className="text-sm text-gray-500" role="status">
            {hitCount === 1 ? "1 Treffer" : `${hitCount} Treffer`} in allen
            Bereichen
          </p>
        ) : (
          visibleCategories.length > 1 && (
            <Button
              type="button"
              variant="surface"
              size="compact"
              className="self-start sm:self-auto"
              onClick={() => setAllExpanded(!allExpanded)}
            >
              {allExpanded ? "Alle einklappen" : "Alle ausklappen"}
            </Button>
          )
        )}
      </div>

      {isFiltering ? (
        hits.length === 0 ? (
          <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
            <EmptyState
              variant="compact"
              icon={<SearchX className="h-5 w-5" aria-hidden="true" />}
              title={`Keine Einstellung passt zu „${query.trim()}“.`}
              description="Versuchen Sie ein anderes Wort, zum Beispiel „Abholung“ oder „Eltern“."
            />
          </div>
        ) : (
          hits.map((hit) => (
            <SettingsCategory
              key={expansionKey(hit.tab.key, hit.category.key)}
              category={hit.category}
              kicker={tabLabel(hit.tab)}
              filterQuery={normalizedQuery}
              onSave={onSave}
              onReset={onReset}
              onSchemaRefresh={onSchemaRefresh}
            />
          ))
        )
      ) : (
        <>
          {tab.key === "enrollment" && <EnrollmentLinkPanel tab={tab} />}
          {tab.categories.map((category) => {
            const key = expansionKey(tab.key, category.key);
            return (
              <SettingsCategory
                key={category.key}
                category={category}
                highlightKey={highlightKey}
                collapsible
                collapsed={!expanded.has(key)}
                onToggle={() => toggleCategory(key)}
                onSave={onSave}
                onReset={onReset}
                onSchemaRefresh={onSchemaRefresh}
              />
            );
          })}
        </>
      )}
    </div>
  );
}

function SettingsSkeleton() {
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Skeleton className="h-10 w-full rounded-lg sm:max-w-sm" />
        <Skeleton className="h-8 w-32 rounded-md" />
      </div>
      {Array.from({ length: 5 }).map((_, idx) => (
        <div
          key={idx}
          className="moto-content-surface rounded-2xl border p-5 shadow-sm"
        >
          <div className="flex items-start gap-3">
            <div className="flex-1 space-y-2">
              <Skeleton className="h-5 w-40 rounded" />
              <Skeleton className="h-4 w-full max-w-md rounded" />
            </div>
            <Skeleton className="h-8 w-8 rounded-md" />
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
        <p className="text-moto-red-strong text-sm">
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
        allTabs={schemaTabsForPage(schema)}
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
        label: TAB_LABELS[key] ?? key,
        icon: tabConcepts[key] ?? defaultTabConcept,
      }))
    : schemaTabsForPage(schema).map((tab) => ({
        id: `settings-${tab.key}`,
        label: tabLabel(tab),
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
