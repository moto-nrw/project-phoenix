// components/dashboard/mobile-bottom-nav.tsx
// Ultra-minimalist mobile navigation following Instagram/Twitter/Uber patterns
"use client";

import React, {
  useRef,
  useCallback,
  useState,
  useEffect,
  useMemo,
} from "react";
import Link from "next/link";
import useSWR from "swr";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import { hasRole, isCaregiver } from "~/lib/auth-utils";
import { navigationIcons } from "~/lib/navigation-icons";
import { operatorPath } from "~/lib/operator-url";
import {
  useNFCEnabled,
  usePresenceMode,
} from "~/components/tenant/tenant-provider";
import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
} from "~/lib/settings-api";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";

// Icon component for consistent SVG rendering
const Icon = ({ path, className }: { path: string; className?: string }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Animation timing constant for initial mount transition delay
// This ensures the sliding indicator position is set before enabling smooth transitions
const INITIAL_MOUNT_DELAY_MS = 100;

interface NavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  requiresAdmin?: boolean;
  requiresGroups?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
  comingSoon?: boolean;
  // Additional pathname prefixes that should highlight this nav entry as
  // active, used when one bottom-nav slot represents a group of related
  // routes (e.g. the five Verwaltung pages).
  activePaths?: string[];
}

// Static base definitions; actual main items are computed per session
// Admins don't have assigned groups or supervision duties (#608)
const ADMIN_MAIN_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Home", iconKey: "home", alwaysShow: true },
  {
    href: "/students/search",
    label: "Suchen",
    iconKey: "search",
    alwaysShow: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
  { href: "/rooms", label: "Räume", iconKey: "rooms", alwaysShow: true },
];

const STAFF_MAIN_ITEMS: NavItem[] = [
  { href: "/ogs-groups", label: "Gruppe", iconKey: "group", alwaysShow: true },
  {
    href: "/active-supervisions",
    label: "Aufsicht",
    iconKey: "supervision",
    alwaysShow: true,
  },
  {
    href: "/students/search",
    label: "Suchen",
    iconKey: "search",
    alwaysShow: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
];

// Order mirrors the desktop sidebar sections: VERWALTUNG → KOMMUNIKATION → TEAM.
const OPERATOR_MAIN_ITEMS: NavItem[] = [
  {
    href: "/operator/organizations",
    label: "Verwaltung",
    iconKey: "rooms",
    alwaysShow: true,
    // Highlight when on any of the five Verwaltung pages.
    activePaths: [
      "/operator/organizations",
      "/operator/schools",
      "/operator/accounts",
      "/operator/devices",
      "/operator/persons",
      "/operator/unregistered-tags",
      "/operator/provisioning",
    ],
  },
  {
    href: "/operator/suggestions",
    label: "Feedback",
    iconKey: "feedback",
    alwaysShow: true,
  },
  {
    href: "/operator/announcements",
    label: "Ankündigungen",
    iconKey: "bell",
    alwaysShow: true,
  },
  {
    href: "/operator/operators",
    label: "Operatoren",
    iconKey: "group",
    alwaysShow: true,
  },
];

// Additional navigation items that appear in the overflow menu
interface AdditionalNavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  requiresAdmin?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
  hideForAdmin?: boolean; // Hide from admin users (for caregiver-specific features)
  comingSoon?: boolean; // Show as grayed out "coming soon" feature
  activePaths?: string[];
  newTab?: boolean; // Open in a new browser tab (e.g. public help guide)
}

// Operator-mode overflow items, everything reachable from the sidebar on
// desktop that isn't already a main bottom-nav slot. The 5 sibling Verwaltung
// pages (Schulen/Konten/Geräte/Personen/Unbekannte RFID) belong here since
// the bottom nav has only one "Verwaltung" slot that lands on
// /operator/organizations.
const OPERATOR_ADDITIONAL_ITEMS: AdditionalNavItem[] = [
  {
    href: "/operator/schools",
    label: "Schulen",
    iconKey: "buildingOffice",
    alwaysShow: true,
  },
  {
    href: "/operator/accounts",
    label: "Konten",
    iconKey: "profile",
    alwaysShow: true,
  },
  {
    href: "/operator/devices",
    label: "Geräte",
    iconKey: "device",
    alwaysShow: true,
  },
  {
    href: "/operator/persons",
    label: "Personen",
    iconKey: "userSingle",
    alwaysShow: true,
  },
  {
    href: "/operator/unregistered-tags",
    label: "Unbekannte RFID",
    iconKey: "security",
    alwaysShow: true,
  },
];

// `tKey` is the parentNav catalog key; the German `label` is the fallback used
// only when rendered outside an intl context. Mapping on a stable key (not the
// German label or href, which are "#" for coming-soon items) keeps the
// translation correct even if the fallback wording changes.
const PARENT_MAIN_ITEMS: readonly (NavItem & { tKey: string })[] = [
  {
    href: "/parents",
    label: "Start",
    tKey: "start",
    iconKey: "home",
    alwaysShow: true,
  },
  {
    href: "/parents/children",
    label: "Meine Kinder",
    tKey: "children",
    iconKey: "group",
    alwaysShow: true,
  },
  {
    href: "#",
    label: "Kalender",
    tKey: "calendar",
    iconKey: "calendar",
    alwaysShow: true,
    comingSoon: true,
  },
];

const PARENT_ADDITIONAL_ITEMS: readonly (AdditionalNavItem & {
  tKey: string;
})[] = [
  {
    href: "/parents/messages",
    label: "Nachrichten",
    tKey: "messages",
    iconKey: "chat",
    alwaysShow: true,
  },
  {
    href: "#",
    label: "Kontaktdaten",
    tKey: "contactData",
    iconKey: "profile",
    alwaysShow: true,
    comingSoon: true,
  },
];

const additionalNavItems: AdditionalNavItem[] = [
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
  { href: "/staff", label: "Mitarbeiter", iconKey: "staff", alwaysShow: true },
  { href: "/rooms", label: "Räume", iconKey: "rooms", alwaysShow: true },
  {
    href: "/substitutions",
    label: "Vertretungen",
    iconKey: "substitutions",
    requiresAdmin: true,
  },
  {
    href: "/timetables",
    label: "Betreuungsplan",
    iconKey: "calendar",
    requiresAdmin: true,
  },
  {
    href: "/database",
    label: "Datenverwaltung",
    iconKey: "database",
    requiresAdmin: true,
  },
  {
    href: "/admin/enrollments",
    label: "Anmeldungen",
    iconKey: "enrollments",
    requiresAdmin: true,
    activePaths: [
      "/admin/enrollments",
      "/enrollment-phases",
      "/care-offerings",
      "/enrollment-form",
    ],
  },
  {
    href: "/time-tracking",
    label: "Zeiterfassung",
    iconKey: "clock",
    alwaysShow: true,
  },
  {
    href: "/emergency",
    label: "Notfall",
    iconKey: "emergency",
    alwaysShow: true,
  },
  {
    href: "/help",
    label: "Hilfe",
    iconKey: "book",
    alwaysShow: true,
    newTab: true,
  },
  {
    href: "/suggestions",
    label: "Feedback",
    iconKey: "feedback",
    alwaysShow: true,
  },
  {
    href: "/settings",
    label: "Einstellungen",
    iconKey: "settings",
    requiresAdmin: true,
  },
  {
    href: "/messages",
    label: "Nachrichten",
    iconKey: "chat",
    alwaysShow: true,
  },
  // Coming soon features - shown to all users
  {
    href: "#",
    label: "Mittagessen",
    iconKey: "utensils",
    alwaysShow: true,
    comingSoon: true,
  },
  // Coming soon features - caregivers only
  {
    href: "#",
    label: "Erinnerungen",
    iconKey: "bell",
    alwaysShow: true,
    hideForAdmin: true,
    comingSoon: true,
  },
  {
    href: "#",
    label: "Berichte",
    iconKey: "chart",
    requiresAdmin: true,
    comingSoon: true,
  },
];

const NFC_ONLY_HREFS = new Set<string>(["/activities"]);

interface MobileBottomNavProps {
  readonly className?: string;
}

export function MobileBottomNav({ className = "" }: MobileBottomNavProps) {
  const tParentNav = useTranslations("parentNav");
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);

  // Refs for sliding indicator
  const navRefs = useRef<(HTMLAnchorElement | null)[]>([]);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });
  const [indicatorVisible, setIndicatorVisible] = useState(false);
  const isInitialMount = useRef(true);

  // Get session for role checking
  const { data: session } = useSession();

  // Get supervision state
  const {
    hasGroups,
    isSupervising,
    isLoadingGroups,
    isLoadingSupervision,
    adminOverviewEnabled,
  } = useOptionalSupervision();

  // Get shell auth mode
  const { mode } = useShellAuth();

  // Check if current path matches nav item
  const isActiveRoute = useCallback(
    (href: string, activePaths?: string[]) => {
      if (href === "#") {
        return false;
      }
      if (href === "/parents") {
        return pathname === "/parents" || pathname === "/";
      }
      if (href === "/dashboard") {
        return pathname === "/dashboard" || pathname === "/";
      }
      // Check if we came from this page via the 'from' query parameter
      if (
        pathname.startsWith("/students/") &&
        searchParams.get("from")?.startsWith(href)
      ) {
        return true;
      }
      if (activePaths?.some((p) => pathname.startsWith(p))) {
        return true;
      }
      return pathname.startsWith(href);
    },
    [pathname, searchParams],
  );

  const closeOverflowMenu = () => {
    setIsOverflowMenuOpen(false);
  };

  // Compute main navigation items per role and mode
  // operatorPath is deterministic for the page lifetime, memoize to avoid per-render churn.
  // activePaths must also go through operatorPath: on the operator subdomain pathname is
  // the clean URL (e.g. /schools), so comparing against /operator/schools would never match.
  const resolvedOperatorMainItems = useMemo(
    () =>
      OPERATOR_MAIN_ITEMS.map((item) => ({
        ...item,
        href: operatorPath(item.href),
        activePaths: item.activePaths?.map(operatorPath),
      })),
    [],
  );
  const resolvedOperatorAdditionalItems = useMemo(
    () =>
      OPERATOR_ADDITIONAL_ITEMS.map((item) => ({
        ...item,
        href: operatorPath(item.href),
      })),
    [],
  );
  const parentMainItems = useMemo(
    () =>
      PARENT_MAIN_ITEMS.map((item) => ({
        ...item,
        label: tParentNav(item.tKey),
      })),
    [tParentNav],
  );
  const parentAdditionalItems = useMemo(
    () =>
      PARENT_ADDITIONAL_ITEMS.map((item) => ({
        ...item,
        label: tParentNav(item.tKey),
      })),
    [tParentNav],
  );
  const baseMain =
    mode === "parent"
      ? parentMainItems
      : mode === "operator"
        ? resolvedOperatorMainItems
        : isCaregiver(session)
          ? STAFF_MAIN_ITEMS
          : hasRole(session, "admin")
            ? ADMIN_MAIN_ITEMS
            : STAFF_MAIN_ITEMS;
  // Admins with supervision overview: inject "Aufsicht" tab dynamically.
  // Gate on adminOverviewEnabled (confirmed via /supervisors/all 200) rather
  // than just isSupervising so a synthetic Schulhof entry does not surface
  // the admin tab when the setting is off. Dual-role teacher-admins see the
  // tab too, the tenant setting is the explicit opt-in signal.
  // STAFF_MAIN_ITEMS already contains /active-supervisions, so only inject
  // when it is missing (i.e. for admin-only users whose baseline is
  // ADMIN_MAIN_ITEMS) to avoid duplicate React keys.
  const alreadyHasSupervisionTab = baseMain.some(
    (item) => item.href === "/active-supervisions",
  );
  const filteredMainItems =
    hasRole(session, "admin") &&
    !isLoadingSupervision &&
    adminOverviewEnabled &&
    !alreadyHasSupervisionTab
      ? [
          ...baseMain.slice(0, 1),
          {
            href: "/active-supervisions",
            label: "Aufsicht",
            iconKey: "supervision" as const,
            alwaysShow: true,
          },
          ...baseMain.slice(1),
        ]
      : baseMain;

  // Pre-compute permission flags to reduce complexity in filter
  const userIsAdmin = hasRole(session, "admin");
  const userIsCaregiver = isCaregiver(session);
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const showActivityNav = nfcEnabled && presenceMode !== "binary";
  const { data: settingsSchema } = useSWR(
    userIsAdmin && mode !== "operator" && mode !== "parent"
      ? SETTINGS_SCHEMA_SWR_KEY
      : null,
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
  const hasGroupSupervision = !isLoadingGroups && hasGroups;
  const hasRoomSupervision = !isLoadingSupervision && isSupervising;

  // Filter additional navigation items based on permissions
  const filteredMainItemsByMode = filteredMainItems.filter(
    (item) => showActivityNav || !NFC_ONLY_HREFS.has(item.href),
  );

  const filteredAdditionalItems = additionalNavItems.filter((item) => {
    // Hide items marked as hideForAdmin for admin users
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) {
      return false;
    }
    if (!showActivityNav && NFC_ONLY_HREFS.has(item.href)) return false;
    if (item.alwaysShow) return true;
    if (item.href === "/timetables" && !timetableEnabled) {
      return false;
    }
    if (item.requiresAdmin) return userIsAdmin;
    if (item.requiresSupervision && !userIsAdmin) {
      return hasGroupSupervision || hasRoomSupervision;
    }
    if (item.requiresActiveSupervision && !userIsAdmin) {
      return hasRoomSupervision;
    }
    return true;
  });

  // Static navigation - 4 main items + overflow drawer. Operator mode uses a
  // dedicated item list (the 4 sibling Verwaltung pages + Einstellungen) since
  // the bottom nav has only one Verwaltung slot.
  const displayMainItems: NavItem[] = filteredMainItemsByMode;
  const showOverflowMenu = true;
  // Avoid duplicates between main and additional
  const mainHrefs = new Set(
    displayMainItems.filter((i) => i.href !== "#").map((i) => i.href),
  );
  const displayAdditionalItems =
    mode === "parent"
      ? parentAdditionalItems.filter((i) => !mainHrefs.has(i.href))
      : mode === "operator"
        ? resolvedOperatorAdditionalItems.filter((i) => !mainHrefs.has(i.href))
        : filteredAdditionalItems.filter((i) => !mainHrefs.has(i.href));

  // Check if any additional nav item is active
  const isAnyAdditionalNavActive = displayAdditionalItems.some((item) =>
    isActiveRoute(item.href, item.activePaths),
  );

  // Update sliding indicator position when route changes
  useEffect(() => {
    const updateIndicator = () => {
      // Find active nav item index
      const activeIndex = displayMainItems.findIndex((item) =>
        isActiveRoute(item.href, item.activePaths),
      );

      if (activeIndex !== -1 && navRefs.current[activeIndex]) {
        const activeElement = navRefs.current[activeIndex];
        if (activeElement) {
          const { offsetLeft, offsetWidth } = activeElement;
          setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
          setIndicatorVisible(true);
        }
      } else if (isAnyAdditionalNavActive && moreButtonRef.current) {
        // "Mehr" button is active
        const { offsetLeft, offsetWidth } = moreButtonRef.current;
        setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
        setIndicatorVisible(true);
      } else {
        // No active item found - hide indicator
        setIndicatorVisible(false);
      }
    };

    // Small delay to ensure DOM is ready
    const timer = setTimeout(updateIndicator, 10);
    return () => clearTimeout(timer);
  }, [pathname, displayMainItems, isAnyAdditionalNavActive, isActiveRoute]);

  // Enable transitions after initial position is set and rendered
  useEffect(() => {
    const timer = setTimeout(() => {
      isInitialMount.current = false;
    }, INITIAL_MOUNT_DELAY_MS);
    return () => clearTimeout(timer);
  }, []);

  // Force indicator update on mount and when refs change
  useEffect(() => {
    const timer = setTimeout(() => {
      const activeIndex = displayMainItems.findIndex((item) =>
        isActiveRoute(item.href, item.activePaths),
      );

      if (activeIndex !== -1 && navRefs.current[activeIndex]) {
        const activeElement = navRefs.current[activeIndex];
        if (activeElement) {
          const { offsetLeft, offsetWidth } = activeElement;
          setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
          setIndicatorVisible(true);
        }
      }
    }, 50);

    return () => clearTimeout(timer);
  }, [displayMainItems, isActiveRoute]);

  return (
    <>
      {/* Spacer to prevent content from being hidden behind fixed nav */}
      <div className="h-16 lg:hidden" />

      {/* shadcn/UI Drawer - Full-width on mobile */}
      <Drawer open={isOverflowMenuOpen} onOpenChange={setIsOverflowMenuOpen}>
        <DrawerContent className="bg-white">
          <div className="w-full">
            {/* Hidden header for accessibility only */}
            <DrawerHeader className="sr-only">
              <DrawerTitle>Navigation</DrawerTitle>
              <DrawerDescription>Wähle eine Seite</DrawerDescription>
            </DrawerHeader>
            <div className="px-4 pt-6 pb-4">
              <div className="space-y-2">
                {displayAdditionalItems.map((item) => {
                  const isActive = isActiveRoute(item.href, item.activePaths);

                  // Coming soon items are not clickable
                  if (item.comingSoon) {
                    return (
                      <div
                        key={item.label}
                        className="flex items-center gap-3 rounded-xl bg-gray-50 px-4 py-3 opacity-50"
                      >
                        <Icon
                          path={
                            navigationIcons[item.iconKey] ??
                            navigationIcons.home
                          }
                          className="h-5 w-5 text-gray-400"
                        />
                        <span className="flex-1 text-base font-medium text-gray-400">
                          {item.label}
                        </span>
                        <span className="rounded bg-gray-200 px-2 py-0.5 text-xs text-gray-500">
                          Bald verfügbar
                        </span>
                      </div>
                    );
                  }

                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={closeOverflowMenu}
                      {...(item.newTab
                        ? { target: "_blank", rel: "noopener noreferrer" }
                        : {})}
                      className={`flex items-center gap-3 rounded-xl px-4 py-3 transition-all ${
                        isActive
                          ? "bg-gray-900 text-white"
                          : "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200"
                      } `}
                    >
                      <Icon
                        path={
                          navigationIcons[item.iconKey] ?? navigationIcons.home
                        }
                        className={`h-5 w-5 ${isActive ? "text-white" : "text-gray-600"}`}
                      />
                      <span className="text-base font-medium">
                        {item.label}
                      </span>
                    </Link>
                  );
                })}
              </div>
            </div>
            <div className="pb-8" />
          </div>
        </DrawerContent>
      </Drawer>

      {/* Modern Pill-Style Bottom Navigation (shadcn-inspired) */}
      <nav
        className={`fixed right-0 bottom-0 left-0 z-30 translate-y-0 transition-transform duration-150 ease-in-out lg:hidden ${className} `}
      >
        {/* Pill container with margins */}
        <div className="px-4 pb-4">
          <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
            <div className="relative flex items-center justify-around gap-1">
              {/* Sliding background indicator */}
              {indicatorVisible && (
                <div
                  className={`absolute top-0 h-full rounded-full bg-gray-900 shadow-md ${
                    isInitialMount.current
                      ? ""
                      : "transition-all duration-150 ease-out"
                  }`}
                  style={{
                    left: `${indicatorStyle.left}px`,
                    width: `${indicatorStyle.width}px`,
                  }}
                />
              )}

              {/* Main navigation items */}
              {displayMainItems.map((item, index) => {
                const isActive = isActiveRoute(item.href, item.activePaths);
                const iconPath =
                  navigationIcons[item.iconKey] ?? navigationIcons.home;

                if (item.comingSoon) {
                  return (
                    <button
                      key={item.label}
                      ref={(el) => {
                        navRefs.current[index] = null;
                        if (el) {
                          el.dataset.navItem = item.label;
                        }
                      }}
                      type="button"
                      disabled
                      className="relative z-10 flex min-h-[44px] cursor-not-allowed items-center justify-center gap-2.5 rounded-full px-3 py-2.5 text-gray-300 transition-colors duration-200"
                      aria-label={`${item.label} bald verfügbar`}
                    >
                      <Icon path={iconPath} className="h-5 w-5 flex-shrink-0" />
                    </button>
                  );
                }

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    ref={(el) => {
                      navRefs.current[index] = el;
                    }}
                    className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                      isActive
                        ? "bg-gray-900 text-white"
                        : "text-gray-400 hover:text-gray-600"
                    } `}
                  >
                    {/* Icon */}
                    <Icon path={iconPath} className="h-5 w-5 flex-shrink-0" />

                    {/* Label - ONLY show when active */}
                    {isActive && (
                      <span className="text-sm font-semibold whitespace-nowrap">
                        {item.label}
                      </span>
                    )}
                  </Link>
                );
              })}

              {/* More button */}
              {showOverflowMenu && (
                <button
                  ref={moreButtonRef}
                  onClick={() => setIsOverflowMenuOpen(true)}
                  className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                    isOverflowMenuOpen || isAnyAdditionalNavActive
                      ? "bg-gray-900 text-white"
                      : "text-gray-400 hover:text-gray-600"
                  } `}
                >
                  {/* Icon */}
                  <Icon
                    path={navigationIcons.more}
                    className="h-5 w-5 flex-shrink-0"
                  />

                  {/* Label - ONLY show when active */}
                  {(isOverflowMenuOpen || isAnyAdditionalNavActive) && (
                    <span className="text-sm font-semibold whitespace-nowrap">
                      Mehr
                    </span>
                  )}
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Safe area padding */}
        <div className="h-safe-area-inset-bottom bg-transparent" />
      </nav>
    </>
  );
}
