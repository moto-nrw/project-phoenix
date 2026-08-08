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
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  hasPermission,
  hasRole,
  isCaregiver,
  isLehrkraftOnly,
} from "~/lib/auth-utils";
import { navigationIcons } from "~/lib/navigation-icons";
import { operatorPath } from "~/lib/operator-url";
import { useParentMealPlanEnabled } from "~/lib/hooks/use-parent-meal-plan-enabled";
import { useParentNewsEnabled } from "~/lib/hooks/use-parent-news-enabled";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { getSettingValue } from "~/lib/settings-api";
import {
  getPlanningMobileActivePaths,
  isPlanningPageHref,
  PLANNING_SUB_PAGES,
  type PlanningPageHref,
} from "~/lib/planning-navigation";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  DATABASE_SECTION,
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  PARENT_SECTION,
  PARENT_SUB_PAGES,
} from "~/lib/section-navigation";
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

// Reines Lehrkraft-Konto (#1772): nur die Klassenansicht ist erreichbar,
// alles andere würde 403 antworten. Hilfe kommt über das Overflow-Menü.
const LEHRKRAFT_MAIN_ITEMS: NavItem[] = [
  {
    href: "/klassen",
    label: "Klassen",
    iconKey: "academicCap",
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
  // Show for admins or anyone holding this tenant permission (matches the
  // backend route gate). Use instead of alwaysShow for permission-gated pages.
  requiresPermission?: string;
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
    href: "/parents/calendar",
    label: "Kalender",
    tKey: "calendar",
    iconKey: "calendar",
    alwaysShow: true,
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
  // Neuigkeiten — only shown once a linked school broadcasts announcements
  // (gated via useParentNewsEnabled in the parent display filter below).
  {
    href: "/parents/news",
    label: "Neuigkeiten",
    tKey: "news",
    iconKey: "newspaper",
  },
  // Essensplan — only shown once a linked school runs a meal plan (gated via
  // useParentMealPlanEnabled in the parent display filter below).
  {
    href: "/parents/meal-plan",
    label: "Essensplan",
    tKey: "mealPlan",
    iconKey: "utensils",
  },
  // Feedback ans Produktteam (#1678) — last of the real entries, mirroring the
  // desktop sidebar where it sits pinned below the daily-use areas. It is a
  // channel to moto, not to the school, so no per-school gate applies.
  {
    href: "/parents/feedback",
    label: "Feedback",
    tKey: "feedback",
    iconKey: "feedback",
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

const PLANNING_ICON_KEYS: Record<
  PlanningPageHref,
  keyof typeof navigationIcons
> = {
  "/betreuungsplan": "betreuungsplan",
  "/dienstplan": "dienstplan",
  "/vertretung": "vertretung",
  "/lists": "calendar",
  "/calendar-periods": "calendar",
  "/payroll": "chart",
};

const PLANNING_ADDITIONAL_ITEMS: AdditionalNavItem[] =
  PLANNING_SUB_PAGES.filter((page) => page.showInMobileNav).map((page) => ({
    href: page.href,
    label: page.label,
    iconKey: PLANNING_ICON_KEYS[page.href],
    requiresAdmin: page.nonAdminPermission === undefined,
    requiresPermission: page.nonAdminPermission,
    activePaths: getPlanningMobileActivePaths(page.href),
  }));

const additionalNavItems: AdditionalNavItem[] = [
  {
    // Klassenansicht (#1772) für Dual-Role-Konten (lehrkraft + user/admin):
    // deren Haupt-Nav ist die Staff-/Admin-Leiste, der Einstieg in die
    // Klassenansicht kommt über das Overflow-Menü. Rollen-Gating unten in
    // filteredAdditionalItems (permission-basiert ginge nicht: admin:*
    // matcht class_day:read, Admins ohne lehrkraft-Rolle haben aber keine
    // Klassen und landen auf einer leeren Seite).
    href: "/klassen",
    label: "Klassenansicht",
    iconKey: "academicCap",
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
  { href: "/staff", label: "Mitarbeiter", iconKey: "staff", alwaysShow: true },
  {
    href: "/calendar",
    label: "Mein Kalender",
    iconKey: "calendar",
    // Match the backend calendar:own gate on GET /api/calendar/my.
    requiresPermission: "calendar:own",
  },
  { href: "/rooms", label: "Räume", iconKey: "rooms", alwaysShow: true },
  {
    // Alt-Bereich für temporären Gruppen-Datenzugriff (#1940) — nur bei
    // festen Gruppen sichtbar (Filter unten).
    href: "/substitutions",
    label: "Gruppenzugriff",
    iconKey: "substitutions",
    requiresAdmin: true,
  },
  // Planning is flattened in the mobile drawer. The shared catalog omits the
  // desktop-only calendar-period editor and supplies all legacy active paths.
  ...PLANNING_ADDITIONAL_ITEMS,
  {
    href: DATABASE_SECTION.href,
    label: DATABASE_SECTION.label,
    iconKey: "database",
    requiresAdmin: true,
  },
  {
    href: ENROLLMENT_SECTION.href,
    label: ENROLLMENT_SECTION.label,
    iconKey: "enrollments",
    requiresAdmin: true,
    activePaths: ENROLLMENT_SUB_PAGES.map((page) => page.href),
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
  // Eltern hub — mirrors the desktop "Eltern" accordion. Mobile has no
  // accordions, so a single overflow entry points at the /eltern overview and
  // the sub-pages are reached from its cards (same treatment as
  // Datenverwaltung / Anmeldungen). Shown to all staff; the overview itself
  // renders only the cards the caller may access.
  {
    href: PARENT_SECTION.href,
    label: PARENT_SECTION.label,
    iconKey: "parents",
    alwaysShow: true,
    activePaths: PARENT_SUB_PAGES.map((page) => page.href),
  },
  // Reminders live in the header bell (always visible on desktop + mobile),
  // so the bottom nav no longer carries a coming-soon "Erinnerungen" entry.
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
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const searchParams = useSearchParams();
  // Strip the tenant prefix so all active-state checks compare against
  // unprefixed paths (e.g. "/eltern"). Only path-routing mode carries the slug
  // in usePathname() as "/{slug}/eltern"; mirror the desktop sidebar's
  // normalization so the "Mehr" button and drawer rows highlight correctly.
  // Gate on routingMode: useTenantSlugSafe() still returns the slug in
  // subdomain mode, so without this guard a tenant whose slug is a real route
  // (e.g. "messages") visiting messages.<domain>/messages would be stripped to
  // "/" and mis-highlight Home. No-op in subdomain/operator/parent mode.
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );
  // Prefixes tenant-scoped hrefs with the slug in path-routing mode (no-op in
  // subdomain/operator/parent mode). Used for the Eltern hub link below.
  const tenantPath = useTenantAwarePath();
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
      // Check if we came from this page via the 'from' query parameter. Grouped
      // items (e.g. Eltern) own several routes via activePaths, so a child page
      // reached with ?from=/messages must still highlight the Eltern entry —
      // compare `from` against the item's href AND its activePaths.
      if (pathname.startsWith("/students/")) {
        const from = searchParams.get("from");
        if (
          from &&
          (from.startsWith(href) ||
            activePaths?.some((p) => from.startsWith(p)))
        ) {
          return true;
        }
      }
      if (activePaths?.some((p) => pathname.startsWith(p))) {
        return true;
      }
      if (href === "/staff") {
        return (
          pathname.startsWith("/staff") &&
          !pathname.startsWith("/staff/dienstplan")
        );
      }
      // /calendar-periods gehört zum Betreuungsplan (activePaths), nicht zum
      // Kalender — ohne Exakt-Match leuchtet /calendar per Präfix mit.
      if (href === "/calendar") {
        return pathname === "/calendar" || pathname.startsWith("/calendar/");
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
  // Reines Lehrkraft-Konto (#1772): geteiltes Prädikat aus auth-utils.
  const lehrkraftOnly = isLehrkraftOnly(session);
  const baseMain =
    mode === "parent"
      ? parentMainItems
      : mode === "operator"
        ? resolvedOperatorMainItems
        : lehrkraftOnly
          ? LEHRKRAFT_MAIN_ITEMS
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
  // Only advertise Essensplan in the parents portal once a linked school runs
  // a meal plan; otherwise the overflow link leads to an empty page.
  const parentMealPlanEnabled = useParentMealPlanEnabled(mode === "parent");
  // Same gate for Neuigkeiten: hidden until a linked school broadcasts
  // announcements, otherwise the overflow link dead-ends on an empty feed.
  const parentNewsEnabled = useParentNewsEnabled(mode === "parent");
  const hasGroupSupervision = !isLoadingGroups && hasGroups;
  const hasRoomSupervision = !isLoadingSupervision && isSupervising;

  // Gruppenzugriff (#1940) ist nur bei festen Gruppen sinnvoll.
  const openCareGroupMode = useOpenCareGroupMode();
  // Planung-Einträge (#1946) hängen an timetable.enabled. Gleiches
  // settingsSchema-Lesemuster wie die Desktop-Sidebar; `!== false`, damit die
  // Einträge während des Schema-Ladens nicht kurz verschwinden. Das Ergebnis
  // gated nur die admin-only Planungs-Einträge, darum feuert der Request auch
  // nur für Admins.
  const { data: settingsSchema } = useSettingsSchema(
    mode === "teacher" && userIsAdmin,
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      shouldRetryOnError: false,
    },
  );
  const timetableEnabled =
    getSettingValue(settingsSchema, "timetable.enabled") !== false;

  // Filter additional navigation items based on permissions
  const filteredMainItemsByMode = filteredMainItems.filter(
    (item) =>
      (showActivityNav || !NFC_ONLY_HREFS.has(item.href)) &&
      // Bei offener Betreuung gibt es keine "meine Gruppe" — der
      // gruppenbasierte Einstieg entfällt (#1544).
      !(openCareGroupMode && item.href === "/ogs-groups"),
  );

  const filteredAdditionalItems = additionalNavItems.filter((item) => {
    // Reines Lehrkraft-Konto (#1772): im Overflow bleibt nur die Hilfe —
    // jede andere Seite würde 403 antworten. (/klassen sitzt dort schon
    // als Haupt-Tab.)
    if (lehrkraftOnly) return item.href === "/help";
    // Dual-Role-Lehrkraft (#1772): Klassenansicht über das Overflow-Menü,
    // die Haupt-Nav bleibt die Staff-/Admin-Leiste.
    if (item.href === "/klassen") return hasRole(session, "lehrkraft");
    // Hide items marked as hideForAdmin for admin users
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) {
      return false;
    }
    if (!showActivityNav && NFC_ONLY_HREFS.has(item.href)) return false;
    if (
      isPlanningPageHref(item.href) &&
      item.href !== "/calendar-periods" &&
      item.href !== "/payroll" &&
      !timetableEnabled &&
      (userIsAdmin || item.requiresPermission === undefined)
    ) {
      return false;
    }
    if (item.href === "/substitutions" && openCareGroupMode) return false;
    if (item.alwaysShow) return true;
    if (item.requiresAdmin) return userIsAdmin;
    if (item.requiresPermission) {
      return userIsAdmin || hasPermission(session, item.requiresPermission);
    }
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
      ? parentAdditionalItems.filter(
          (i) =>
            !mainHrefs.has(i.href) &&
            (i.href !== "/parents/meal-plan" || parentMealPlanEnabled) &&
            (i.href !== "/parents/news" || parentNewsEnabled),
        )
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
          <div className="min-h-0 w-full flex-1 overflow-y-auto">
            {/* Hidden header for accessibility only */}
            <DrawerHeader className="sr-only">
              <DrawerTitle>Navigation</DrawerTitle>
              <DrawerDescription>Wähle eine Seite</DrawerDescription>
            </DrawerHeader>
            <div className="px-4 pt-6 pb-4">
              <div className="space-y-2">
                {displayAdditionalItems.map((item) => {
                  const isActive = isActiveRoute(item.href, item.activePaths);
                  // The Eltern hub is a tenant-scoped [tenant]/eltern route. In
                  // path-routing mode a bare "/eltern" href is captured as the
                  // tenant slug, so prefix it the same way the /eltern page
                  // prefixes its card links. Other entries stay bare — /help is
                  // host-agnostic and must not carry the slug.
                  const href =
                    item.href === "/eltern" || isPlanningPageHref(item.href)
                      ? tenantPath(item.href)
                      : item.href;

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
                      href={href}
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
                    aria-label={item.label}
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
                  type="button"
                  onClick={() => setIsOverflowMenuOpen(true)}
                  aria-label="Mehr"
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
