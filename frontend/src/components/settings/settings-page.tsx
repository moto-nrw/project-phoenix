"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search, SearchX } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { createLogger } from "~/lib/logger";
import {
  applyOptimisticSchemaUpdate,
  clearSettingValue,
  saveSettingValue,
} from "~/lib/settings-api";
import { notifySettingsChanged } from "~/lib/settings-broadcast";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";
import type { SettingsSchema, SchemaTab } from "~/lib/settings-api";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import {
  displayTabLabel,
  normalizeQuery,
  searchTabs,
  visibleCategoryItems,
} from "./settings-filter";
import {
  schemaTabsForPage,
  useSettingsSections,
  type SettingsSection,
} from "./settings-nav-items";

import { HomeBlocksTab } from "./home-blocks-tab";
import { PersonalizationTab } from "./personalization-tab";
import { EnrollmentLinkPanel } from "./enrollment-link-panel";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useNFCEnabled } from "~/lib/tenant-context";
import { useTenantMutate } from "~/lib/swr/hooks";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";

// Settings whose value affects the supervision context (sidebar / mobile nav)
// and therefore require an immediate re-fetch after save/reset instead of
// waiting for the next navigation or re-login.
const SUPERVISION_AFFECTING_KEYS = new Set<string>([
  "operations.operational_overview_scope",
]);

const logger = createLogger({ component: "SettingsPage" });

interface SettingsTabContentProps {
  readonly tab: SchemaTab;
  /** Every tab of the page; the search box looks across all of them. */
  readonly allTabs: readonly SchemaTab[];
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<void>;
  readonly onReset: (key: string) => Promise<void>;
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

  // Ein Gitter wächst nicht mit dem Seitenrumpf. Die Bereiche sind eine
  // Liste einzelner Karten; die letzte zog sich sonst auf einem kurzen Reiter
  // als leere Fläche bis zur Unterkante (#3893).
  return (
    <div className="grid grid-cols-1 gap-4">
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
              tooltip={
                allExpanded
                  ? "Klappt alle Bereiche wieder zu."
                  : "Zeigt alle Einstellungen dieser Seite auf einmal."
              }
            >
              {allExpanded ? "Alle einklappen" : "Alle ausklappen"}
            </Button>
          )
        )}
      </div>

      {isFiltering ? (
        hits.length === 0 ? (
          <SectionCard>
            <EmptyState
              variant="compact"
              icon={<SearchX className="h-5 w-5" aria-hidden="true" />}
              title={`Keine Einstellung passt zu „${query.trim()}“.`}
              description="Versuchen Sie ein anderes Wort, zum Beispiel „Abholung“ oder „Eltern“."
            />
          </SectionCard>
        ) : (
          hits.map((hit) => (
            <SettingsCategory
              key={expansionKey(hit.tab.key, hit.category.key)}
              category={hit.category}
              tabLabel={displayTabLabel(hit.tab)}
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
        <Skeleton shape="control" className="h-10 w-full sm:max-w-sm" />
        <Skeleton className="h-8 w-32" />
      </div>
      {Array.from({ length: 5 }).map((_, idx) => (
        <SectionCard key={idx}>
          <div className="flex items-start gap-3">
            <div className="flex-1 space-y-2">
              <Skeleton className="h-5 w-40" />
              <Skeleton className="h-4 w-full max-w-md" />
            </div>
            <Skeleton className="h-8 w-8" />
          </div>
        </SectionCard>
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
  const nfcEnabled = useNFCEnabled();
  const router = useRouter();
  const tenantMutate = useTenantMutate();
  const {
    data: schema,
    error: fetchError,
    isLoading,
    mutate: revalidate,
  } = useSettingsSchema();
  // Ladefehler vor Ort mit Wiederholen (#2517), nie als Leerzustand.
  const loadError = useSwrLoadError(
    fetchError,
    "die Liste der Einstellungen",
    () => revalidate(),
  );

  // Reminders settings decide whether the header reminders bell shows at all.
  // After saving/resetting one, revalidate THIS tenant's /api/reminders cache
  // so the bell's `enabled` flag flips immediately instead of waiting for the
  // poll. The reminders SWR key is scoped ("{slug}:{account}:reminders");
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

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path (#2517).
  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<void> => {
      await saveSettingValue(key, value);
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
    },
    [applyOptimistic, refreshSupervision, revalidateRemindersIfNeeded, router],
  );

  const handleReset = useCallback(
    async (key: string): Promise<void> => {
      await clearSettingValue(key);
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

  // Ladefehler: bis der Katalogtext da ist, bleibt das Skelett stehen.
  if (fetchError && !schema) {
    return loadError ? (
      <LoadErrorAlert error={loadError} />
    ) : (
      <SettingsSkeleton />
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
      <SettingsTabContent
        tab={tab}
        allTabs={schemaTabsForPage(schema, nfcEnabled)}
        highlightKey={highlightKey}
        onSave={handleSave}
        onReset={handleReset}
        onSchemaRefresh={handleSchemaRefresh}
      />
    </>
  );
}

/**
 * The sections of the settings page with the content of each. The section list
 * itself lives in `settings-nav-items.ts`, which the navigation shares.
 * Returns null silently if user has no access or schema is empty.
 */
export function useSettingsTabs(): {
  tabs: readonly SettingsSection[];
  renderTab: (tabId: string) => React.ReactNode;
  highlightTabId: string | null;
  highlightNeedsNfc: boolean;
} | null {
  const searchParams = useSearchParams();
  const sections = useSettingsSections();
  if (!sections) {
    return null;
  }
  const highlightKey = searchParams.get("highlight");

  const renderTab = (tabId: string) => {
    if (tabId === "settings-personalisierung") {
      return <PersonalizationTab />;
    }
    if (tabId === "settings-startseite") {
      return <HomeBlocksTab />;
    }
    const settingsKey = tabId.replace("settings-", "");
    return <SettingsContent tabKey={settingsKey} highlightKey={highlightKey} />;
  };

  return { ...sections, renderTab };
}
