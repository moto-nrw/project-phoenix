// components/dashboard/sidebar.tsx
"use client";

import { Suspense, useCallback, useEffect, useMemo } from "react";
import Link from "next/link";
import useSWR from "swr";
import { usePathname, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  usePresenceMode,
  useTenantSlugSafe,
} from "~/components/tenant/tenant-provider";
import { useSession } from "next-auth/react";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import { hasRole, isCaregiver } from "~/lib/auth-utils";
import { operatorPath } from "~/lib/operator-url";
import { useSidebarAccordion } from "~/lib/hooks/use-sidebar-accordion";
import { useSuggestionsUnread } from "~/lib/hooks/use-suggestions-unread";
import { useOperatorSuggestionsUnread } from "~/lib/hooks/use-operator-suggestions-unread";
import { useGroupAttendanceCounts } from "~/lib/group-attendance-count-context";
import { SidebarAccordionSection } from "~/components/dashboard/sidebar-accordion-section";
import { SidebarSubItem } from "~/components/dashboard/sidebar-sub-item";
import { navigationIcons } from "~/lib/navigation-icons";
import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
} from "~/lib/settings-api";

// Type für Navigation Items
interface NavItem {
  href: string;
  label: string;
  icon: string;
  requiresAdmin?: boolean;
  alwaysShow?: boolean;
  hideForAdmin?: boolean;
  comingSoon?: boolean;
  bottomPinned?: boolean;
  activeColor?: string;
}

// Flat navigation items (excludes accordion sections: ogs-groups, active-supervisions, database)
const NAV_ITEMS: NavItem[] = [
  {
    href: "/dashboard",
    label: "Home",
    icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
    activeColor: "text-[#5080D8]",
    requiresAdmin: true,
  },
  {
    href: "/students/search",
    label: "Kindersuche",
    icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
    activeColor: "text-[#5080D8]",
    alwaysShow: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    activeColor: "text-[#FF3130]",
    alwaysShow: true,
  },
  {
    href: "/rooms",
    label: "Räume",
    icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    activeColor: "text-indigo-500",
    alwaysShow: true,
  },
  {
    href: "/staff",
    label: "Mitarbeiter",
    icon: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2",
    activeColor: "text-[#F78C10]",
    alwaysShow: true,
  },
  {
    href: "/substitutions",
    label: "Vertretungen",
    icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15",
    activeColor: "text-pink-500",
    requiresAdmin: true,
  },
  {
    href: "/timetables",
    label: "Stundenplan",
    icon: navigationIcons.calendar,
    activeColor: "text-[#5080D8]",
    requiresAdmin: true,
  },
  {
    href: "/time-tracking",
    label: "Zeiterfassung",
    icon: "M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0Z",
    activeColor: "text-sky-500",
    alwaysShow: true,
  },
  // Coming soon features - shown to all users
  {
    href: "#",
    label: "Nachrichten",
    icon: "M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z",
    alwaysShow: true,
    comingSoon: true,
  },
  {
    href: "#",
    label: "Mittagessen",
    icon: "M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13",
    alwaysShow: true,
    comingSoon: true,
  },
  // Coming soon features - caregivers only
  {
    href: "#",
    label: "Erinnerungen",
    icon: "M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9",
    alwaysShow: true,
    hideForAdmin: true,
    comingSoon: true,
  },
  // Coming soon features - admin only
  {
    href: "#",
    label: "Dienstpläne",
    icon: "M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z",
    requiresAdmin: true,
    comingSoon: true,
  },
  {
    href: "#",
    label: "Berichte",
    icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
    requiresAdmin: true,
    comingSoon: true,
  },
  {
    href: "/suggestions",
    label: "Feedback",
    icon: "M10.34 15.84c-.688-.06-1.386-.09-2.09-.09H7.5a4.5 4.5 0 110-9h.75c.704 0 1.402-.03 2.09-.09m0 9.18c.253.962.584 1.892.985 2.783.247.55.06 1.21-.463 1.511l-.657.38c-.551.318-1.26.117-1.527-.461a20.845 20.845 0 01-1.44-4.282m3.102.069a18.03 18.03 0 01-.59-4.59c0-1.586.205-3.124.59-4.59m0 9.18a23.848 23.848 0 018.835 2.535M10.34 6.66a23.847 23.847 0 008.835-2.535m0 0A23.74 23.74 0 0018.795 3m.38 1.125a23.91 23.91 0 011.014 5.395m-1.014 8.855c-.118.38-.245.754-.38 1.125m.38-1.125a23.91 23.91 0 001.014-5.395m0-3.46c.495.413.811 1.035.811 1.73 0 .695-.316 1.317-.811 1.73m0-3.46a24.347 24.347 0 010 3.46",
    activeColor: "text-teal-500",
    alwaysShow: true,
    bottomPinned: true,
  },
  {
    href: "/settings",
    label: "Einstellungen",
    icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM15 12a3 3 0 11-6 0 3 3 0 016 0z",
    activeColor: "text-gray-500",
    requiresAdmin: true,
    bottomPinned: true,
  },
];

// Shared accent for all Verwaltung items. Source of truth:
// LOCATION_COLORS.OTHER_ROOM in ~/lib/location-helper (#5080D8). Tailwind's
// JIT scans for literal class strings, so the hex is inlined here rather
// than interpolated at runtime. If LOCATION_COLORS.OTHER_ROOM ever changes,
// update this literal to match.
const OPERATOR_VERWALTUNG_ACTIVE_COLOR = "text-[#5080D8]";

interface OperatorNavSection {
  readonly label: string;
  readonly items: readonly NavItem[];
}

// Operator navigation — grouped into static labeled sections. Icons sourced
// from ~/lib/navigation-icons so the desktop sidebar and the mobile bottom
// nav/overflow drawer stay visually consistent per page.
const OPERATOR_NAV_SECTIONS: readonly OperatorNavSection[] = [
  {
    label: "VERWALTUNG",
    items: [
      {
        href: "/operator/organizations",
        label: "Träger",
        icon: navigationIcons.rooms,
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/schools",
        label: "Schulen",
        icon: navigationIcons.buildingOffice,
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/accounts",
        label: "Konten",
        icon: navigationIcons.profile,
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/devices",
        label: "Geräte",
        icon: navigationIcons.device,
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/persons",
        label: "Personen",
        icon: navigationIcons.userSingle,
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
    ],
  },
  {
    label: "KOMMUNIKATION",
    items: [
      {
        href: "/operator/suggestions",
        label: "Feedback",
        icon: navigationIcons.feedback,
        activeColor: "text-teal-500",
        alwaysShow: true,
      },
      {
        href: "/operator/announcements",
        label: "Ankündigungen",
        icon: navigationIcons.bell,
        activeColor: "text-amber-500",
        alwaysShow: true,
      },
    ],
  },
  {
    label: "TEAM",
    items: [
      {
        href: "/operator/operators",
        label: "Operatoren",
        icon: navigationIcons.group,
        activeColor: "text-violet-500",
        alwaysShow: true,
      },
    ],
  },
];

const OPERATOR_BOTTOM_ITEM: NavItem = {
  href: "/operator/settings",
  label: "Einstellungen",
  icon: navigationIcons.settings,
  activeColor: "text-gray-500",
  bottomPinned: true,
  alwaysShow: true,
};

// Static sub-pages for Datenverwaltung accordion
const DATABASE_SUB_PAGES = [
  { href: "/database/students", label: "Kinder" },
  { href: "/database/personal", label: "Personal" },
  { href: "/database/rooms", label: "Räume" },
  { href: "/database/activities", label: "Aktivitäten" },
  { href: "/database/groups", label: "Gruppen" },
  { href: "/database/roles", label: "Rollen" },
  { href: "/database/devices", label: "Geräte" },
  { href: "/database/permissions", label: "Berechtigungen" },
];

// Static sub-pages for Anmeldungen accordion (admin only). Order is
// most-used first: review queue → setup pages. The first sub-item
// is labeled "Übersicht" rather than "Anmeldungen" to avoid clashing
// with the parent section label.
const ENROLLMENTS_SUB_PAGES = [
  { href: "/admin/enrollments", label: "Übersicht" },
  { href: "/enrollment-phases", label: "Phasen" },
  { href: "/care-offerings", label: "Betreuungsangebote" },
  { href: "/enrollment-form", label: "Formular" },
];

/** Determine if a group sub-item should be highlighted as active */
function isGroupSubItemActive(
  childGroupId: string | null,
  groupId: string,
  pathname: string,
  currentGroupParam: string | null,
  index: number,
): boolean {
  if (childGroupId) return childGroupId === groupId;
  if (!pathname.startsWith("/ogs-groups")) return false;
  if (currentGroupParam) return currentGroupParam === groupId;
  return index === 0;
}

/** Determine if a room sub-item should be highlighted as active */
function isRoomSubItemActive(
  childRoomId: string | null,
  roomId: string,
  pathname: string,
  currentRoomParam: string | null,
  index: number,
): boolean {
  if (childRoomId) return childRoomId === roomId;
  if (!pathname.startsWith("/active-supervisions")) return false;
  if (currentRoomParam) return currentRoomParam === roomId;
  return index === 0;
}

interface SidebarProps {
  readonly className?: string;
}

function SidebarContent({ className = "" }: SidebarProps) {
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const { data: session } = useSession();
  const { mode } = useShellAuth();

  // Strip tenant prefix so all path checks use unprefixed paths (e.g. "/database").
  // useTenantRouter().push() produces paths like "/school-a/database" while <Link href="/database">
  // goes through proxy rewrite and keeps "/database". Normalizing here avoids mismatches.
  // When tenantSlug is null (operator mode), no stripping needed.
  const pathname =
    tenantSlug && rawPathname.startsWith(`/${tenantSlug}/`)
      ? rawPathname.slice(tenantSlug.length + 1)
      : tenantSlug && rawPathname === `/${tenantSlug}`
        ? "/"
        : rawPathname;

  // Get supervision state
  const {
    isLoadingGroups,
    isLoadingSupervision,
    groups,
    supervisedRooms,
    adminOverviewEnabled,
  } = useOptionalSupervision();

  // Get unread suggestions count for badge (teacher mode)
  const { unreadCount: suggestionsUnreadCount } = useSuggestionsUnread();
  // Get unread suggestions count for badge (operator mode)
  const { unreadCount: operatorUnreadCount } = useOperatorSuggestionsUnread();

  // Accordion state — pass `from` param so child pages (e.g. student detail)
  // keep the originating accordion section open
  const fromParam = searchParams.get("from");
  const { expanded, toggle } = useSidebarAccordion(pathname, fromParam);

  const userIsAdmin = hasRole(session, "admin");
  const userIsCaregiver = isCaregiver(session);
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";
  const { counts: groupAttendanceCounts } = useGroupAttendanceCounts();
  const canShowGroupAttendanceCounts = pathname.startsWith("/ogs-groups");
  const { data: settingsSchema } = useSWR(
    userIsAdmin ? SETTINGS_SCHEMA_SWR_KEY : null,
    fetchSettingsSchema,
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    },
  );

  const timetableEnabled =
    settingsSchema?.tabs
      .flatMap((tab) => tab.categories)
      .flatMap((category) => category.items)
      .find((item) => item.key === "timetable.enabled")?.value === true;

  const formatGroupAttendanceCount = (groupId: string | number) => {
    if (!canShowGroupAttendanceCounts) return undefined;
    const count = groupAttendanceCounts[groupId.toString()];
    return count ? `${count.present}/${count.total}` : undefined;
  };

  // Nav items hidden in binary-mode tenants. Rooms and Activities are room/visit
  // concepts with no operational meaning when the tenant only tracks
  // in-school/out-of-school on active.attendance. The Aktuelle-Aufsicht
  // accordion is gated separately below (it's not in NAV_ITEMS).
  const BINARY_HIDDEN_HREFS = new Set<string>(["/rooms", "/activities"]);

  // Filter flat navigation items based on permissions
  const filteredNavItems = NAV_ITEMS.filter((item) => {
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) return false;
    if (isBinaryMode && BINARY_HIDDEN_HREFS.has(item.href)) return false;
    if (item.href === "/timetables" && !timetableEnabled) {
      return false;
    }
    if (item.alwaysShow) return true;
    if (item.requiresAdmin && !userIsAdmin) return false;
    return true;
  });

  // Helper to determine active href for student detail pages based on referrer
  const getStudentDetailActiveHref = (from: string | null): string => {
    if (!from) return "/students/search";
    if (from.startsWith("/ogs-groups")) return "/ogs-groups";
    if (from.startsWith("/active-supervisions")) return "/active-supervisions";
    // Drill-in from a room ("Kinder im Raum") — both the legacy subpage
    // /rooms/{id} and the modal URL /rooms?room={id} count, so the
    // sidebar reflects the actual entry path in either flow.
    if (from.startsWith("/rooms/") || from.startsWith("/rooms?"))
      return "/rooms";
    if (from.startsWith("/students/search")) return "/students/search";
    return "/students/search";
  };

  // Operator drill-in highlight: hierarchy-based, not tab-based.
  // The sidebar reflects WHERE in the tree the user is, not which tab they
  // happen to have open. Tabs scope a view; they don't change the section.
  //
  // - Anywhere under /organizations or /organizations/{slug}      → Träger
  // - Anywhere under /organizations/{slug}/schools/{schoolSlug}   → Schulen
  //
  // Pathname may include or omit the /operator prefix depending on host
  // (operator subdomain strips it via operatorPath; tenant subdomains keep
  // it). Both forms are matched.
  const ORG_AREA_RE = /^(?:\/operator)?\/organizations(\/|$)/;
  const SCHOOL_DRILLIN_RE =
    /^(?:\/operator)?\/organizations\/[^/]+\/schools\/[^/]+/;

  const getOperatorDrillInActiveHref = (): string | null => {
    if (SCHOOL_DRILLIN_RE.test(pathname)) {
      return operatorPath("/operator/schools");
    }
    if (ORG_AREA_RE.test(pathname)) {
      return operatorPath("/operator/organizations");
    }
    return null;
  };

  // Check if a navigation link should be highlighted as active
  const isActiveLink = (href: string) => {
    const isStudentDetailPage =
      pathname.startsWith("/students/") && pathname !== "/students/search";
    if (isStudentDetailPage) {
      const from = searchParams.get("from");
      return getStudentDetailActiveHref(from) === href;
    }
    const operatorDrillInHref = getOperatorDrillInActiveHref();
    if (operatorDrillInHref) {
      return href === operatorDrillInHref;
    }
    if (href === "/dashboard") return pathname === "/dashboard";
    return pathname.startsWith(href);
  };

  // Check if a section's parent header should be highlighted
  // Parent highlights only when on the section page WITHOUT a sub-item selected
  const isAccordionSectionActive = (
    parentHref: string,
    hasSubItemSelected: boolean,
  ) => {
    const isStudentDetailPage =
      pathname.startsWith("/students/") && pathname !== "/students/search";
    if (isStudentDetailPage) {
      const from = searchParams.get("from");
      if (getStudentDetailActiveHref(from) !== parentHref) return false;
      // If a sub-item is highlighted on the child page, don't highlight the parent
      return !hasSubItemSelected;
    }
    if (!pathname.startsWith(parentHref)) return false;
    // If a sub-item is selected, don't highlight the parent
    return !hasSubItemSelected;
  };

  const getLinkClasses = (href: string, comingSoon?: boolean) => {
    const baseClasses =
      "flex items-center px-3 py-2.5 text-sm lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm rounded-lg transition-colors";

    if (comingSoon) {
      return `${baseClasses} text-gray-400 cursor-not-allowed`;
    }

    const activeClasses = "bg-gray-100 text-gray-900 font-semibold";
    const inactiveClasses =
      "text-gray-600 hover:bg-gray-50 hover:text-gray-900 font-medium";

    return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
  };

  // Split items into main (scrollable) and bottom (pinned) sections
  const mainNavItems = filteredNavItems.filter((item) => !item.bottomPinned);
  const bottomNavItems = filteredNavItems.filter((item) => item.bottomPinned);

  const getIconClasses = (item: NavItem) => {
    const base =
      "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5 transition-colors";
    if (!item.comingSoon && item.activeColor && isActiveLink(item.href)) {
      return `${base} ${item.activeColor}`;
    }
    return base;
  };

  const renderNavItem = (item: NavItem) => (
    <div key={item.comingSoon ? item.label : item.href}>
      {item.comingSoon ? (
        <div
          className={`group ${getLinkClasses(item.href, true)}`}
          title="Bald verfügbar"
        >
          <svg
            className={getIconClasses(item)}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d={item.icon}
            />
          </svg>
          <span>{item.label}</span>
          <span className="ml-2 rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 opacity-0 transition-opacity group-hover:opacity-100">
            Bald
          </span>
        </div>
      ) : (
        <Link href={item.href} className={getLinkClasses(item.href)}>
          <svg
            className={getIconClasses(item)}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d={item.icon}
            />
          </svg>
          <span className="flex flex-1 items-center justify-between">
            {item.label}
            {item.href === "/suggestions" && suggestionsUnreadCount > 0 && (
              <span className="ml-2 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-[10px] font-semibold text-white">
                {suggestionsUnreadCount > 99 ? "99+" : suggestionsUnreadCount}
              </span>
            )}
          </span>
        </Link>
      )}
    </div>
  );

  // Determine which flat items come before / after the accordion insertion points
  // Order: Home (admin) → [Groups accordion] → [Supervisions accordion] → Kindersuche → Aktivitaten → Raume → Mitarbeiter → Vertretungen (admin) → [Database accordion] → coming soon → bottom pinned
  const beforeAccordionItems = mainNavItems.filter(
    (item) =>
      item.href === "/dashboard" ||
      (item.href === "/students/search" && !item.comingSoon),
  );

  // Items between Kindersuche and Database accordion
  const middleItems = mainNavItems.filter(
    (item) =>
      !item.comingSoon &&
      item.href !== "/dashboard" &&
      item.href !== "/students/search" &&
      item.href !== "/substitutions",
  );

  // Vertretungen (admin only, flat)
  const substitutionsItem = mainNavItems.find(
    (item) => item.href === "/substitutions",
  );

  // Coming soon items
  const comingSoonItems = mainNavItems.filter((item) => item.comingSoon);

  // Get current search params for group/room selection
  const currentGroupParam = searchParams.get("group");
  const currentRoomParam = searchParams.get("room");

  // On child pages (e.g. student detail with ?from=/ogs-groups), determine
  // which sub-item should stay highlighted using the last selection from localStorage.
  const isChildOfAccordion =
    pathname.startsWith("/students/") && pathname !== "/students/search";
  const childFromParam = isChildOfAccordion ? fromParam : null;
  const childGroupId =
    childFromParam?.startsWith("/ogs-groups") && globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-group")
      : null;
  const childRoomId =
    childFromParam?.startsWith("/active-supervisions") &&
    globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-room")
      : null;

  // Persist last selected sub-item per accordion section to localStorage.
  // Pages read this on mount to restore the user's last selection.
  useEffect(() => {
    if (pathname.startsWith("/ogs-groups") && currentGroupParam) {
      localStorage.setItem("sidebar-last-group", currentGroupParam);
      const groupName = groups.find(
        (g) => g.id.toString() === currentGroupParam,
      )?.name;
      if (groupName) {
        localStorage.setItem("sidebar-last-group-name", groupName);
      }
    }
  }, [pathname, currentGroupParam, groups]);

  useEffect(() => {
    if (pathname.startsWith("/active-supervisions") && currentRoomParam) {
      localStorage.setItem("sidebar-last-room", currentRoomParam);
      const roomName = supervisedRooms.find(
        (r) => r.id === currentRoomParam,
      )?.name;
      if (roomName) {
        localStorage.setItem("sidebar-last-room-name", roomName);
      }
    }
  }, [pathname, currentRoomParam, supervisedRooms]);

  useEffect(() => {
    if (
      pathname.startsWith("/database/") &&
      DATABASE_SUB_PAGES.some((p) => pathname === p.href)
    ) {
      localStorage.setItem("sidebar-last-database", pathname);
    }
  }, [pathname]);

  // Toggle accordion AND navigate to the correct URL (with last-selected sub-item).
  // Reads localStorage at click-time so the page loads with the right param immediately.
  const handleGroupsToggle = useCallback(() => {
    toggle("groups");
    if (!pathname.startsWith("/ogs-groups")) {
      const savedGroupId = localStorage.getItem("sidebar-last-group");
      const targetGroup = savedGroupId
        ? groups.find((g) => g.id.toString() === savedGroupId)
        : groups[0];
      const groupId = targetGroup?.id ?? groups[0]?.id;
      if (groupId) {
        router.push(`/ogs-groups?group=${groupId}`);
      } else {
        router.push("/ogs-groups");
      }
    }
  }, [toggle, pathname, groups, router]);

  const handleSupervisionsToggle = useCallback(() => {
    toggle("supervisions");
    if (!pathname.startsWith("/active-supervisions")) {
      const savedRoomId = localStorage.getItem("sidebar-last-room");
      const targetRoom = savedRoomId
        ? supervisedRooms.find((r) => r.id === savedRoomId)
        : supervisedRooms[0];
      const roomId = targetRoom?.id ?? supervisedRooms[0]?.id;
      if (roomId) {
        router.push(`/active-supervisions?room=${roomId}`);
      } else {
        router.push("/active-supervisions");
      }
    }
  }, [toggle, pathname, supervisedRooms, router]);

  const handleDatabaseToggle = useCallback(() => {
    if (!pathname.startsWith("/database")) {
      // Not on any database page → expand accordion + navigate to hub
      toggle("database");
      router.push("/database");
    } else if (pathname === "/database") {
      // On hub page → just toggle (collapse/expand)
      toggle("database");
    } else {
      // On a sub-page like /database/rooms → navigate back to hub (accordion stays open)
      router.push("/database");
    }
  }, [toggle, pathname, router]);

  const isOnEnrollmentsPage = ENROLLMENTS_SUB_PAGES.some((p) =>
    pathname.startsWith(p.href),
  );

  const handleEnrollmentsToggle = useCallback(() => {
    // Hub = the request review queue. Mirrors handleDatabaseToggle's
    // navigate-on-expand behavior so clicking the section label always
    // ends up on a useful page rather than just toggling chevrons.
    const onSection = ENROLLMENTS_SUB_PAGES.some((p) =>
      pathname.startsWith(p.href),
    );
    if (!onSection) {
      toggle("enrollments");
      router.push("/admin/enrollments");
    } else if (pathname === "/admin/enrollments") {
      toggle("enrollments");
    } else {
      router.push("/admin/enrollments");
    }
  }, [toggle, pathname, router]);

  // Caregiver accordions are driven by the explicit caregiver role.
  // Show staff accordions for caregivers (user/teacher role) and for admins
  // only when the admin_supervision_overview setting is confirmed enabled
  // (adminOverviewEnabled avoids the synthetic Schulhof entry triggering the
  // accordion when the setting is off).
  const showStaffAccordions =
    userIsCaregiver || (userIsAdmin && adminOverviewEnabled);

  // Resolve operator nav hrefs once (operatorPath is deterministic for the page lifetime)
  const resolvedOperatorSections = useMemo(
    () =>
      OPERATOR_NAV_SECTIONS.map((section) => ({
        label: section.label,
        items: section.items.map((item) => ({
          ...item,
          href: operatorPath(item.href),
        })),
      })),
    [],
  );
  const resolvedOperatorBottomItem = useMemo(
    () => ({
      ...OPERATOR_BOTTOM_ITEM,
      href: operatorPath(OPERATOR_BOTTOM_ITEM.href),
    }),
    [],
  );
  const operatorSuggestionsHref = useMemo(
    () => operatorPath("/operator/suggestions"),
    [],
  );

  // Operator mode: sectioned navigation (static labels, no accordions)
  if (mode === "operator") {
    const renderOperatorItem = (item: NavItem) => (
      <Link
        key={item.href}
        href={item.href}
        className={getLinkClasses(item.href)}
      >
        <svg
          className={getIconClasses(item)}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d={item.icon}
          />
        </svg>
        <span className="flex flex-1 items-center justify-between">
          {item.label}
          {item.href === operatorSuggestionsHref && operatorUnreadCount > 0 && (
            <span className="ml-2 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1.5 text-[10px] font-semibold text-white">
              {operatorUnreadCount > 99 ? "99+" : operatorUnreadCount}
            </span>
          )}
        </span>
      </Link>
    );

    return (
      <aside
        className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
      >
        <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
          <nav className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3">
            {resolvedOperatorSections.map((section, index) => (
              <div
                key={section.label}
                className={index > 0 ? "mt-5" : undefined}
              >
                <p className="mb-1.5 px-3 text-[10px] font-semibold tracking-wider text-gray-400 uppercase lg:px-4 xl:px-3">
                  {section.label}
                </p>
                <div className="space-y-1">
                  {section.items.map(renderOperatorItem)}
                </div>
              </div>
            ))}
          </nav>

          <nav className="space-y-1 border-t border-gray-200 p-3 lg:p-4 xl:p-3">
            {renderOperatorItem(resolvedOperatorBottomItem)}
          </nav>
        </div>
      </aside>
    );
  }

  // Parent mode: minimal sidebar — Übersicht (dashboard) + bottom
  // logout. The dashboard itself surfaces the "Neue Anmeldung" CTA
  // and the per-child links, so the sidebar is intentionally tiny.
  if (mode === "parent") {
    return (
      <aside
        className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
      >
        <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
          <nav className="flex-1 space-y-1 p-3 lg:p-4 xl:p-3">
            <Link href="/parents" className={getLinkClasses("/parents")}>
              <svg
                className="mr-3 h-5 w-5 text-gray-400 group-hover:text-gray-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
                />
              </svg>
              <span>Übersicht</span>
            </Link>
          </nav>
        </div>
      </aside>
    );
  }

  return (
    <aside
      className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
    >
      <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
        {/* Main navigation — scrollable */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-3 lg:p-4 xl:p-3">
          {/* Home (admin only) */}
          {beforeAccordionItems
            .filter((item) => item.href === "/dashboard")
            .map(renderNavItem)}

          {/* Meine Gruppen accordion (staff only) */}
          {showStaffAccordions && (
            <SidebarAccordionSection
              icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
              label={
                groups.length > 1
                  ? "Meine Gruppen"
                  : [
                      "Meine Gruppe",
                      formatGroupAttendanceCount(groups[0]?.id ?? ""),
                    ]
                      .filter(Boolean)
                      .join(" ")
              }
              activeColor="text-[#83CD2D]"
              isExpanded={expanded === "groups"}
              onToggle={handleGroupsToggle}
              isActive={isAccordionSectionActive(
                "/ogs-groups",
                Boolean(currentGroupParam) ||
                  Boolean(childGroupId) ||
                  groups.length > 0,
              )}
              isIconActive={
                pathname.startsWith("/ogs-groups") || Boolean(childGroupId)
              }
              isLoading={isLoadingGroups}
              emptyText="Keine Gruppen zugeordnet"
              hasChildren={groups.length > 0}
            >
              {groups.map((group, index) => (
                <SidebarSubItem
                  key={group.id}
                  href={`/ogs-groups?group=${group.id}`}
                  label={group.name}
                  count={formatGroupAttendanceCount(group.id)}
                  isActive={isGroupSubItemActive(
                    childGroupId,
                    group.id.toString(),
                    pathname,
                    currentGroupParam,
                    index,
                  )}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Aktuelle Aufsicht accordion (staff only; hidden in binary mode
              because room-level supervision has no meaning without visits) */}
          {showStaffAccordions && !isBinaryMode && (
            <SidebarAccordionSection
              icon="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              label={
                supervisedRooms.length > 1
                  ? "Aktuelle Aufsichten"
                  : "Aktuelle Aufsicht"
              }
              activeColor="text-violet-500"
              isExpanded={expanded === "supervisions"}
              onToggle={handleSupervisionsToggle}
              isActive={isAccordionSectionActive(
                "/active-supervisions",
                Boolean(currentRoomParam) ||
                  Boolean(childRoomId) ||
                  supervisedRooms.length > 0,
              )}
              isIconActive={
                pathname.startsWith("/active-supervisions") ||
                Boolean(childRoomId)
              }
              isLoading={isLoadingSupervision}
              emptyText="Keine aktive Aufsicht"
              hasChildren={supervisedRooms.length > 0}
            >
              {supervisedRooms.map((room, index) => (
                <SidebarSubItem
                  key={`${room.id}-${room.groupId ?? index}`}
                  href={
                    room.isSchulhof
                      ? `/active-supervisions?room=schulhof`
                      : `/active-supervisions?room=${room.id}`
                  }
                  label={room.name}
                  isActive={isRoomSubItemActive(
                    childRoomId,
                    room.isSchulhof ? "schulhof" : room.id,
                    pathname,
                    currentRoomParam,
                    index,
                  )}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Kindersuche (flat) */}
          {beforeAccordionItems
            .filter((item) => item.href === "/students/search")
            .map(renderNavItem)}

          {/* Flat middle items: Aktivitaten, Raume, Mitarbeiter */}
          {middleItems.map(renderNavItem)}

          {/* Vertretungen (admin, flat) */}
          {substitutionsItem && renderNavItem(substitutionsItem)}

          {/* Datenverwaltung accordion (admin only) */}
          {userIsAdmin && (
            <SidebarAccordionSection
              icon="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4"
              label="Datenverwaltung"
              activeColor="text-gray-500"
              isExpanded={expanded === "database"}
              onToggle={handleDatabaseToggle}
              isActive={isAccordionSectionActive(
                "/database",
                DATABASE_SUB_PAGES.some((p) => pathname === p.href),
              )}
              isIconActive={pathname.startsWith("/database")}
              hasChildren={DATABASE_SUB_PAGES.length > 0}
            >
              {DATABASE_SUB_PAGES.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={page.href}
                  label={page.label}
                  isActive={pathname === page.href}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Anmeldungen accordion (admin only). Bundles the four
              enrollment-management surfaces — request review queue,
              phases, care offerings, and form schema — that admins
              use across the parent enrollment lifecycle. */}
          {userIsAdmin && (
            <SidebarAccordionSection
              icon="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              label="Anmeldungen"
              activeColor="text-[#83CD2D]"
              isExpanded={expanded === "enrollments"}
              onToggle={handleEnrollmentsToggle}
              isActive={isOnEnrollmentsPage}
              isIconActive={isOnEnrollmentsPage}
              hasChildren={ENROLLMENTS_SUB_PAGES.length > 0}
            >
              {ENROLLMENTS_SUB_PAGES.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={page.href}
                  label={page.label}
                  isActive={pathname.startsWith(page.href)}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Coming soon items */}
          {comingSoonItems.map(renderNavItem)}
        </nav>

        {/* Bottom pinned items */}
        {bottomNavItems.length > 0 && (
          <nav className="space-y-1 border-t border-gray-200 p-3 lg:p-4 xl:p-3">
            {bottomNavItems.map(renderNavItem)}
          </nav>
        )}
      </div>
    </aside>
  );
}

export function Sidebar({ className = "" }: SidebarProps) {
  return (
    <Suspense
      fallback={
        <aside
          className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
        >
          <div className="sticky top-[73px] p-3">
            <nav className="space-y-0.5">
              {/* Skeleton placeholders matching nav item height */}
              <div className="flex items-center px-3 py-2">
                <div className="mr-3 h-5 w-5 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-3 py-2">
                <div className="mr-3 h-5 w-5 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-28 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-3 py-2">
                <div className="mr-3 h-5 w-5 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-20 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-3 py-2">
                <div className="mr-3 h-5 w-5 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
              </div>
            </nav>
          </div>
        </aside>
      }
    >
      <SidebarContent className={className} />
    </Suspense>
  );
}
