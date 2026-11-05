"use client";

import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { accountsConcept } from "~/lib/concepts/accounts";
import { calendarConcept } from "~/lib/concepts/calendar";
import { dashboardConcept } from "~/lib/concepts/dashboard";
import { devicesConcept } from "~/lib/concepts/devices";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { listsConcept } from "~/lib/concepts/lists";
import { mealPlanConcept } from "~/lib/concepts/mealPlan";
import { notificationsConcept } from "~/lib/concepts/notifications";
import { permissionsConcept } from "~/lib/concepts/permissions";
import { privacyConcept } from "~/lib/concepts/privacy";
import { settingsConcept } from "~/lib/concepts/settings";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import type { MotoConcept } from "~/lib/moto-concepts";
import type { SchemaTab, SettingsSchema } from "~/lib/settings-api";
import { useNFCEnabled } from "~/lib/tenant-context";
import {
  TAB_LABELS,
  displayTabLabel,
  visibleCategoryItems,
} from "./settings-filter";

/**
 * Die Bereiche der OGS-Einstellungen, die Seite und Seitenleiste teilen
 * (#5750). Diese Datei bleibt frei von den Inhalten der Bereiche: die immer
 * geladene Navigation darf `settings-page.tsx` nicht mitziehen.
 */

// Payroll settings (#1417) have their own maintenance page under /payroll —
// rendering the auto-generated tab here would create a second, worse surface
// for the same values. The birthday switches (#3737) live on the hand-written
// "Startseite für alle" tab next to the birthday card. The holiday settings
// of the Träger (#5621) concern only the Träger-Büro and live in the
// Träger-Portal. Search skips all three.
const TABS_RENDERED_ELSEWHERE = new Set(["abrechnung", "startseite", "ferien"]);

// Every field on "Geräte" configures the NFC tablets (#3735). A school without
// NFC does not see the tab at all instead of an empty one.
const NFC_ONLY_TABS = new Set(["devices"]);

/**
 * In der Seitenleiste stehen die Bereiche untereinander: jeder trägt ein
 * eigenes Symbol, auf dem Telefon ist es sogar der einzige Hinweis.
 */
const TAB_CONCEPTS: Record<string, MotoConcept> = {
  operations: settingsConcept,
  meals: mealPlanConcept,
  reminders: notificationsConcept,
  notifications: notificationsConcept,
  gdpr: privacyConcept,
  devices: devicesConcept,
  enrollment: enrollmentsConcept,
  system: calendarConcept,
  general: settingsConcept,
  security: permissionsConcept,
  modules: listsConcept,
};

// A tab whose settings are all hidden (DependsOn, access) would open onto the
// search box and nothing else (#3925). It is left out like the NFC tabs.
function hasVisibleSetting(tab: SchemaTab): boolean {
  return tab.categories.some(
    (category) => visibleCategoryItems(category).length > 0,
  );
}

export function schemaTabsForPage(
  schema: SettingsSchema | null | undefined,
  nfcEnabled: boolean,
) {
  return (schema?.tabs ?? []).filter(
    (tab) =>
      !TABS_RENDERED_ELSEWHERE.has(tab.key) &&
      (nfcEnabled || !NFC_ONLY_TABS.has(tab.key)) &&
      hasVisibleSetting(tab),
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

export interface SettingsSection {
  /** `settings-<Schlüssel>`; der Schlüssel steht als `?tab=` in der Adresse. */
  readonly id: string;
  readonly label: string;
  readonly icon: MotoConcept;
}

export interface SettingsSections {
  readonly tabs: readonly SettingsSection[];
  /** The tab that holds the `?highlight=` setting, if the school sees it. */
  readonly highlightTabId: string | null;
  /**
   * The `?highlight=` setting sits on a tab this school does not see because
   * it has no NFC tablets (#3735), so the page can say why.
   */
  readonly highlightNeedsNfc: boolean;
}

/**
 * The sections of the school's settings; `null` while the schema loads. When
 * the schema fetch failed, placeholder sections keep the page mounted so it
 * can show its own retry instead of silently dropping them.
 */
export function useSettingsSections(): SettingsSections | null {
  const searchParams = useSearchParams();
  const { data: session } = useSession();
  const { data: schema, error: schemaError, isLoading } = useSettingsSchema();
  const nfcEnabled = useNFCEnabled();
  const canManageHomeBlocks = hasPermission(session, "config:update");

  if (isLoading) {
    return null;
  }

  const conceptFor = (key: string) => TAB_CONCEPTS[key] ?? settingsConcept;
  const fallbackTabKeys = ["operations", "gdpr", "devices", "system"].filter(
    (key) => nfcEnabled || !NFC_ONLY_TABS.has(key),
  );
  const schemaTabs: SettingsSection[] = schemaError
    ? fallbackTabKeys.map((key) => ({
        id: `settings-${key}`,
        label: TAB_LABELS[key] ?? key,
        icon: conceptFor(key),
      }))
    : schemaTabsForPage(schema, nfcEnabled).map((tab) => ({
        id: `settings-${tab.key}`,
        label: displayTabLabel(tab),
        icon: conceptFor(tab.key),
      }));

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
  const homeBlocksTab: SettingsSection = {
    id: "settings-startseite",
    label: "Startseite für alle",
    icon: dashboardConcept,
  };

  // Personalisierung is always available (permission-gated inside the component)
  const personalizationTab: SettingsSection = {
    id: "settings-personalisierung",
    label: "Personalisierung",
    icon: accountsConcept,
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

  return { tabs, highlightTabId, highlightNeedsNfc };
}

/**
 * Welcher Bereich offen ist: der aus `?tab=`, sonst der mit der verlinkten
 * Einstellung, sonst der erste.
 */
export function activeSettingsSectionId(
  sections: Pick<SettingsSections, "tabs"> &
    Partial<Pick<SettingsSections, "highlightTabId">>,
  requestedTab: string | null,
): string {
  const isShown = (tabId: string | null | undefined): tabId is string =>
    tabId !== null &&
    tabId !== undefined &&
    sections.tabs.some((tab) => tab.id === tabId);
  const requestedId = requestedTab ? `settings-${requestedTab}` : null;
  if (isShown(requestedId)) return requestedId;
  if (isShown(sections.highlightTabId)) return sections.highlightTabId;
  return sections.tabs[0]?.id ?? "";
}

/** Die Adresse eines Bereichs, ohne Mandanten-Präfix. */
export function settingsSectionHref(sectionId: string): string {
  return `/settings?tab=${sectionId.replace(/^settings-/, "")}`;
}
