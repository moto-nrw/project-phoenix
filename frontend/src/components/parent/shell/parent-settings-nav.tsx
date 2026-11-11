"use client";

import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Suspense } from "react";
import { PortalOverflowBottomNav } from "~/components/ui/portal-overflow-bottom-nav";
import { PortalSettingsSidebarEntries } from "~/components/ui/portal-sidebar";
import {
  SettingsReturnPathMemory,
  useSettingsReturnHref,
} from "~/components/ui/settings-return-path";
import { backConcept } from "~/lib/concepts/back";
import { moreConcept } from "~/lib/concepts/more";
import { parentPath } from "~/lib/parent-url";
import { isParentSettingsPath } from "./parent-nav-active";
import {
  PARENT_SETTINGS_SECTIONS,
  activeParentSettingsSection,
} from "./parent-nav-items";

const RETURN_PATH_KEY = "parent-settings-return-path";
const NAV_ITEM_ATTRIBUTE = "data-parent-nav-item";

/** Merkt sich die zuletzt geoeffnete App-Seite fuer „Zurück zur App" (#5750). */
export function ParentSettingsReturnPathMemory() {
  return (
    <Suspense fallback={null}>
      <SettingsReturnPathMemory
        storageKey={RETURN_PATH_KEY}
        isSettingsPath={isParentSettingsPath}
      />
    </Suspense>
  );
}

function useParentSettingsNav() {
  const t = useTranslations("parentNav");
  const searchParams = useSearchParams();
  const active = activeParentSettingsSection(searchParams.get("tab"));
  return {
    back: {
      key: "zurueck-zur-app",
      href: useSettingsReturnHref(RETURN_PATH_KEY, parentPath("/parents")),
      label: t("backToApp"),
      concept: backConcept,
      active: false,
    },
    groupLabel: t("settings"),
    moreLabel: t("more"),
    items: PARENT_SETTINGS_SECTIONS.map((section) => ({
      key: `settings-${section.key}`,
      href: `${parentPath("/parents/settings")}?tab=${section.key}`,
      label: t(section.tKey),
      concept: section.concept,
      active: section.key === active.key,
    })),
  };
}

function ParentSettingsSidebarEntriesContent() {
  const { back, groupLabel, items } = useParentSettingsNav();
  return (
    <PortalSettingsSidebarEntries
      back={back}
      groupLabel={groupLabel}
      items={items}
      navItemAttribute={NAV_ITEM_ATTRIBUTE}
    />
  );
}

/** Die Eintraege der Seitenleiste in den Einstellungen. */
export function ParentSettingsSidebarEntries() {
  return (
    <Suspense fallback={null}>
      <ParentSettingsSidebarEntriesContent />
    </Suspense>
  );
}

function ParentSettingsBottomNavContent() {
  const { back, items, moreLabel } = useParentSettingsNav();
  return (
    <PortalOverflowBottomNav
      items={[back, ...items]}
      navItemAttribute={NAV_ITEM_ATTRIBUTE}
      moreConcept={moreConcept}
      moreLabel={moreLabel}
    />
  );
}

/** Die mobile Leiste in den Einstellungen: Rueckweg zur App und Bereiche. */
export function ParentSettingsBottomNav() {
  return (
    <Suspense fallback={null}>
      <ParentSettingsBottomNavContent />
    </Suspense>
  );
}
