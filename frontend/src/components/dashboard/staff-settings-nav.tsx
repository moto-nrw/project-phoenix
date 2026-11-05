"use client";

import { usePathname, useSearchParams } from "next/navigation";
import {
  activeSettingsSectionId,
  settingsSectionHref,
  useSettingsSections,
} from "~/components/settings/settings-nav-items";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink } from "~/components/ui/nav-link";
import {
  PortalOverflowBottomNav,
  type PortalOverflowNavItem,
} from "~/components/ui/portal-overflow-bottom-nav";
import {
  SettingsReturnPathMemory,
  useSettingsReturnHref,
} from "~/components/ui/settings-return-path";
import { Skeleton } from "~/components/ui/skeleton";
import { backConcept } from "~/lib/concepts/back";
import { STAFF_FLAT_PAGES } from "~/lib/section-navigation";
import {
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  SIDEBAR_ICON_CLASSES,
  SIDEBAR_NAV_GAP,
  SIDEBAR_NAV_PADDING,
  sidebarLabelClasses,
  sidebarRowClasses,
} from "./sidebar-geometry";

const RETURN_PATH_KEY = "staff-settings-return-path";
const SETTINGS_PATH = STAFF_FLAT_PAGES.settings.href;
const SETTINGS_LABEL = STAFF_FLAT_PAGES.settings.label;

/** Ob die Adresse, ohne Mandanten-Präfix, in den Einstellungen liegt (#5750). */
export function isStaffSettingsPath(pathname: string): boolean {
  return pathname === SETTINGS_PATH || pathname.startsWith(`${SETTINGS_PATH}/`);
}

function useStaffPathname(): string {
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  return normalizeTenantPathname(rawPathname, tenantSlug, routingMode);
}

/** Ob gerade die Einstellungen offen sind; für Seitenleiste und mobile Leiste. */
export function useIsStaffSettingsPage(): boolean {
  return isStaffSettingsPath(useStaffPathname());
}

/**
 * Merkt sich die zuletzt geöffnete App-Seite. Gespeichert wird die Adresse, wie
 * sie im Browser steht: mit Präfix, wenn die Schule über einen Pfad läuft.
 */
export function StaffSettingsReturnPathMemory() {
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  return (
    <SettingsReturnPathMemory
      storageKey={RETURN_PATH_KEY}
      isSettingsPath={(pathname) =>
        isStaffSettingsPath(
          normalizeTenantPathname(pathname, tenantSlug, routingMode),
        )
      }
    />
  );
}

function useStaffSettingsNav() {
  const tenantPath = useTenantAwarePath();
  const searchParams = useSearchParams();
  const sections = useSettingsSections();
  const returnHref = useSettingsReturnHref(RETURN_PATH_KEY, tenantPath("/"));
  const activeId = sections
    ? activeSettingsSectionId(sections, searchParams.get("tab"))
    : "";
  return {
    loading: sections === null,
    back: {
      key: "zurueck-zur-app",
      href: returnHref,
      label: "Zurück zur App",
      concept: backConcept,
      active: false,
    },
    items: (sections?.tabs ?? []).map((section) => ({
      key: section.id,
      href: tenantPath(settingsSectionHref(section.id)),
      label: section.label,
      concept: section.icon,
      active: section.id === activeId,
    })),
  };
}

/**
 * Der Inhalt der Seitenleiste in den Einstellungen (#5750): dieselbe Leiste wie
 * sonst, auch eingeklappt, aber mit den Bereichen und dem Rückweg zur App. Das
 * Schema lädt erst hier, die übrige Navigation kommt ohne es aus.
 */
export function StaffSettingsSidebarNav({
  collapsed,
  labelsMounted,
  labelsVisible,
}: {
  readonly collapsed: boolean;
  readonly labelsMounted: boolean;
  readonly labelsVisible: boolean;
}) {
  const { loading, back, items } = useStaffSettingsNav();
  const renderRow = (row: PortalOverflowNavItem) => (
    <NavLink
      key={row.key}
      href={row.href}
      data-staff-settings-nav-item={row.key}
      aria-current={row.active ? "page" : undefined}
      className={sidebarRowClasses({ isActive: row.active })}
      {...(collapsed ? { title: row.label, "aria-label": row.label } : {})}
    >
      <MotoNavIcon
        scene
        concept={row.concept}
        active={row.active}
        className={SIDEBAR_ICON_CLASSES}
      />
      {labelsMounted && (
        <span className={sidebarLabelClasses(labelsVisible)}>{row.label}</span>
      )}
    </NavLink>
  );

  return (
    <nav
      aria-label={SETTINGS_LABEL}
      className={`${collapsed ? "scrollbar-hidden" : ""} flex-1 overflow-y-auto ${SIDEBAR_NAV_PADDING}`}
    >
      {renderRow(back)}
      <p
        className={`mt-5 mb-1.5 truncate px-3 text-xs font-semibold tracking-wider text-gray-400 uppercase motion-safe:transition-opacity motion-safe:duration-150 ${labelsVisible ? "opacity-100" : "opacity-0"}`}
        aria-hidden={collapsed}
      >
        {SETTINGS_LABEL}
      </p>
      <div className={SIDEBAR_NAV_GAP}>
        {loading
          ? ["w-24", "w-28", "w-20", "w-32"].map((widthClass) => (
              <div key={widthClass} className="flex h-10 items-center px-3">
                <Skeleton className="h-5 w-5 shrink-0 rounded" />
                {!collapsed && (
                  <Skeleton className={`ml-3 h-4 ${widthClass} rounded`} />
                )}
              </div>
            ))
          : items.map(renderRow)}
      </div>
    </nav>
  );
}

/** Die mobile Leiste in den Einstellungen: Rückweg zur App und Bereiche. */
export function StaffSettingsBottomNav() {
  const { back, items } = useStaffSettingsNav();
  return (
    <PortalOverflowBottomNav
      items={[back, ...items]}
      navItemAttribute="data-staff-settings-nav-item"
    />
  );
}
