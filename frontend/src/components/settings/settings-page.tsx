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
import { Skeleton } from "~/components/ui/skeleton";
import { SettingsCategory } from "./settings-category";
import {
  normalizeQuery,
  searchTabs,
  visibleCategoryItems,
} from "./settings-filter";
import { useSession } from "next-auth/react";

import { hasPermission } from "~/lib/auth-utils";

import { HomeBlocksTab } from "./home-blocks-tab";
import { PersonalizationTab } from "./personalization-tab";
import { EnrollmentLinkPanel } from "./enrollment-link-panel";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useNFCEnabled } from "~/lib/tenant-context";
import { useTenantMutate } from "~/lib/swr/hooks";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
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
  system: "Kalender und Export",
  general: "Allgemein",
  security: "Sicherheit",
};

function tabLabel(tab: SchemaTab): string {
  return TAB_LABELS[tab.key] ?? tab.label;
}

// Payroll settings (#1417) have their own maintenance page under /payroll —
// rendering the auto-generated tab here would create a second, worse surface
// for the same values. The birthday switches (#3737) live on the hand-written
// "Startseite für alle" tab next to the birthday card. Search skips both.
const TABS_RENDERED_ELSEWHERE = new Set(["abrechnung", "startseite"]);

// Every field on "Geräte" configures the NFC tablets (#3735). A school without
// NFC does not see the tab at all instead of an empty one.
const NFC_ONLY_TABS = new Set(["devices"]);

function schemaTabsForPage(
  schema: SettingsSchema | null | undefined,
  nfcEnabled: boolean,
) {
  return (schema?.tabs ?? []).filter(
    (tab) =>
      !TABS_RENDERED_ELSEWHERE.has(tab.key) &&
      (nfcEnabled || !NFC_ONLY_TABS.has(tab.key)),
  );
}

/** The tab of the schema that holds a setting key, if any. */
function schemaTabKeyOf(
  schema: SettingsSchema | null | undefined,
  settingKey: string,
): string | null {
  const owner = (schema?.tabs ?? []).find((tab) =>
    tab.categories.some((category) =>
      category.items.some((item) => item.key === settingKey),
    ),
  );
  return owner?.key ?? null;
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
              tabLabel={tabLabel(hit.tab)}
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

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path (#2517).
  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<string | null> => {
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
      return null;
    },
    [applyOptimistic, refreshSupervision, revalidateRemindersIfNeeded, router],
  );

  const handleReset = useCallback(
    async (key: string): Promise<string | null> => {
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
 * Returns the tab definitions for injecting into SettingsLayout's extraTabs.
 * Each tab renders a SettingsContent component for its key.
 * Returns null silently if user has no access or schema is empty.
 *
 * `highlightTabId` is the tab that holds the `?highlight=` setting, so a deep
 * link opens the right tab even without `?tab=`. `highlightNeedsNfc` is set
 * when that setting sits on a tab this school does not see because it has no
 * NFC tablets (#3735), so the page can say why instead of landing nowhere.
 */
export function useSettingsTabs(): {
  tabs: { id: string; label: string; icon: MotoConceptKey }[];
  renderTab: (tabId: string) => React.ReactNode;
  highlightTabId: string | null;
  highlightNeedsNfc: boolean;
} | null {
  const searchParams = useSearchParams();
  const { data: session } = useSession();
  const { data: schema, error: schemaError, isLoading } = useSettingsSchema();
  const nfcEnabled = useNFCEnabled();
  const canManageHomeBlocks = hasPermission(session, "config:update");

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
  const fallbackTabKeys = ["operations", "gdpr", "devices", "system"].filter(
    (key) => nfcEnabled || !NFC_ONLY_TABS.has(key),
  );
  const schemaTabs = schemaError
    ? fallbackTabKeys.map((key) => ({
        id: `settings-${key}`,
        label: TAB_LABELS[key] ?? key,
        icon: tabConcepts[key] ?? defaultTabConcept,
      }))
    : schemaTabsForPage(schema, nfcEnabled).map((tab) => ({
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

  // "Startseite für alle" is what the school prescribes for everybody (#2875).
  // The name carries the "für alle" on purpose: the start page also has a
  // personal "Startseite anpassen" dialog, and two labels sharing a word stem
  // are read as two names for the same thing unless the screen itself says
  // otherwise. It is a hand-written tab rather than a registry entry: the
  // choice is one of three states per start page block, and the block
  // catalogue lives in the frontend.
  //
  // Unlike Personalisierung the tab only exists for whoever may actually write
  // the prescription. A tab that opens onto "you are not allowed to change
  // this" is a dead end; everybody else adjusts their own start page from the
  // start page itself.
  const homeBlocksTab: {
    id: string;
    label: string;
    icon: MotoConceptKey;
  } = {
    id: "settings-startseite",
    label: "Startseite für alle",
    icon: "settings",
  };

  const tabs = [
    ...schemaTabs,
    ...(canManageHomeBlocks ? [homeBlocksTab] : []),
    personalizationTab,
  ];
  const highlightKey = searchParams.get("highlight");
  const highlightSchemaTab = highlightKey
    ? schemaTabKeyOf(schema, highlightKey)
    : null;
  let highlightTabId: string | null = null;
  if (highlightSchemaTab === "startseite") {
    highlightTabId = canManageHomeBlocks ? homeBlocksTab.id : null;
  } else if (highlightSchemaTab) {
    const candidate = `settings-${highlightSchemaTab}`;
    highlightTabId = tabs.some((tab) => tab.id === candidate)
      ? candidate
      : null;
  }
  const highlightNeedsNfc =
    highlightSchemaTab !== null &&
    NFC_ONLY_TABS.has(highlightSchemaTab) &&
    !nfcEnabled;

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

  return { tabs, renderTab, highlightTabId, highlightNeedsNfc };
}
