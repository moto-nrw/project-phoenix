"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import {
  PortalBottomNav,
  PortalBottomNavLink,
} from "~/components/ui/portal-bottom-nav";
import {
  PortalSettingsSidebarEntries,
  PortalSidebar,
} from "~/components/ui/portal-sidebar";
import {
  SettingsReturnPathMemory,
  useSettingsReturnHref,
} from "~/components/ui/settings-return-path";
import { backConcept } from "~/lib/concepts/back";
import { schoolPath } from "~/lib/school-url";
import { isSchoolSettingsPath } from "./school-nav-active";
import {
  SCHOOL_SETTINGS_ENTRY,
  SCHOOL_SETTINGS_SECTIONS,
  activeSchoolSettingsSection,
} from "./school-nav-items";

const RETURN_PATH_KEY = "school-settings-return-path";
const NAV_ITEM_ATTRIBUTE = "data-school-nav-item";
const BOTTOM_NAV_PADDING = "px-4";
const BOTTOM_NAV_LABEL =
  "hidden text-sm font-semibold whitespace-nowrap min-[480px]:inline";

/** Merkt sich die zuletzt geöffnete App-Seite für „Zurück zur App" (#5750). */
export function SchoolSettingsReturnPathMemory() {
  return (
    <Suspense fallback={null}>
      <SettingsReturnPathMemory
        storageKey={RETURN_PATH_KEY}
        isSettingsPath={isSchoolSettingsPath}
      />
    </Suspense>
  );
}

function useSchoolSettingsNav() {
  const searchParams = useSearchParams();
  const active = activeSchoolSettingsSection(searchParams.get("tab"));
  return {
    back: {
      key: "zurueck-zur-app",
      href: useSettingsReturnHref(RETURN_PATH_KEY, schoolPath("/school")),
      label: "Zurück zur App",
      concept: backConcept,
      active: false,
    },
    items: SCHOOL_SETTINGS_SECTIONS.map((section) => ({
      key: section.key,
      href: `${schoolPath(SCHOOL_SETTINGS_ENTRY.href)}?tab=${section.key}`,
      label: section.label,
      concept: section.concept,
      active: section.key === active.key,
    })),
  };
}

function SchoolSettingsSidebarContent() {
  const { back, items } = useSchoolSettingsNav();
  return (
    <PortalSidebar navLabel={SCHOOL_SETTINGS_ENTRY.label}>
      <PortalSettingsSidebarEntries
        back={back}
        groupLabel={SCHOOL_SETTINGS_ENTRY.label}
        items={items}
        navItemAttribute={NAV_ITEM_ATTRIBUTE}
      />
    </PortalSidebar>
  );
}

export function SchoolSettingsSidebar() {
  return (
    <Suspense
      fallback={
        <PortalSidebar navLabel={SCHOOL_SETTINGS_ENTRY.label}>
          {null}
        </PortalSidebar>
      }
    >
      <SchoolSettingsSidebarContent />
    </Suspense>
  );
}

function SchoolSettingsBottomNavContent() {
  const { back, items } = useSchoolSettingsNav();
  return (
    <PortalBottomNav listClassName="gap-0">
      {[back, ...items].map((item) => (
        <PortalBottomNavLink
          key={item.key}
          href={item.href}
          label={item.label}
          concept={item.concept}
          active={item.active}
          data-school-nav-item={item.key}
          paddingClassName={BOTTOM_NAV_PADDING}
          labelClassName={BOTTOM_NAV_LABEL}
        />
      ))}
    </PortalBottomNav>
  );
}

export function SchoolSettingsBottomNav() {
  return (
    <Suspense fallback={null}>
      <SchoolSettingsBottomNavContent />
    </Suspense>
  );
}
