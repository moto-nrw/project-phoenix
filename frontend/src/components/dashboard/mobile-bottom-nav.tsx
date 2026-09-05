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
import { NavLink } from "~/components/ui/nav-link";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  hasEffectiveAdminScope,
  hasPermission,
  hasRole,
  isCaregiver,
} from "~/lib/auth-utils";
import { useChangeRequestAccess } from "~/lib/hooks/use-change-request-access";
import { navigationIcons } from "~/lib/navigation-icons";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { operatorPath } from "~/lib/operator-url";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import {
  useAttendanceLogEnabled,
  useDisplayEnabled,
  useNFCEnabled,
  useOpenCareGroupMode,
  useStaffMessagingEnabled,
  usePresenceMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
  useTimetableEnabled,
} from "~/lib/tenant-context";
import { getSettingValue } from "~/lib/settings-api";
import {
  getPlanningMobileActivePaths,
  isPlanningPageHref,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  COMMUNICATION_SUB_PAGES,
  DATABASE_SECTION,
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  PARENT_SUB_PAGES,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";
import {
  STAFF_NAV_BOTTOM,
  STAFF_NAV_CONCEPTS,
  STAFF_NAV_GROUPS,
  STAFF_NAV_TOP,
  type StaffNavEntry,
  type StaffNavSectionKey,
} from "~/lib/staff-navigation";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { Button, ButtonLink } from "~/components/ui/button";
import { LogoutModal } from "~/components/ui/logout-modal";
import { StaffPreviewModal } from "~/components/staff-preview/staff-preview-modal";
import { RefreshButton } from "./header/refresh-button";
import { Bell, Eye } from "lucide-react";

// Icon component for consistent SVG rendering
const Icon = ({ path, className }: { path: string; className?: string }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
    aria-hidden="true"
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

function MobileNavIcon({
  item,
  active,
  className,
}: {
  readonly item: Pick<NavItem, "concept" | "iconKey">;
  readonly active: boolean;
  readonly className?: string;
}) {
  const concept = item.concept ? MOTO_CONCEPTS[item.concept] : null;
  if (!concept) {
    return (
      <Icon
        path={navigationIcons[item.iconKey] ?? navigationIcons.home}
        className={className}
      />
    );
  }

  if (active) {
    return (
      <MotoDuotoneIcon
        icon={concept.icon}
        tone={concept.tone}
        size={20}
        className={className}
      />
    );
  }

  const ConceptIcon = concept.icon;
  return (
    <ConceptIcon
      size={20}
      weight="regular"
      className={className}
      aria-hidden="true"
    />
  );
}

// Animation timing constant for initial mount transition delay
// This ensures the sliding indicator position is set before enabling smooth transitions
const INITIAL_MOUNT_DELAY_MS = 100;

/**
 * Indikator-Position nur setzen, wenn sie sich ändert. Die Effekte unten
 * hängen an `displayMainItems`, das jeder Render neu filtert; ein immer neues
 * State-Objekt hielt darüber eine Render-Schleife am Laufen (#2938: rund
 * 2.000 Renders pro Sekunde im Leerlauf auf jeder Seite).
 */
function keepIfUnchanged(left: number, width: number) {
  return (previous: { left: number; width: number }) =>
    previous.left === left && previous.width === width
      ? previous
      : { left, width };
}

interface NavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  concept?: MotoConceptKey;
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
  {
    href: "/dashboard",
    label: "Home",
    iconKey: "home",
    concept: "dashboard",
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
    concept: "activities",
    alwaysShow: true,
  },
  {
    href: "/rooms",
    label: "Räume",
    iconKey: "rooms",
    concept: "rooms",
    alwaysShow: true,
  },
];

const STAFF_MAIN_ITEMS: NavItem[] = [
  {
    // Tagesplan (#2383): die Standardseite der Betreuungskräfte, deshalb der
    // erste Tab. Gating (binary-Modus, timetable.enabled) unten in
    // filteredMainItemsByMode.
    href: "/tagesplan",
    label: "Tagesplan",
    iconKey: "betreuungsplan",
    concept: "carePlan",
    alwaysShow: true,
  },
  {
    href: "/ogs-groups",
    label: "Gruppe",
    iconKey: "group",
    concept: "groups",
    alwaysShow: true,
  },
  {
    href: "/active-supervisions",
    label: "Aufsicht",
    iconKey: "supervision",
    concept: "supervision",
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
    concept: "activities",
    alwaysShow: true,
  },
];

// Order mirrors the desktop sidebar sections: VERWALTUNG → KOMMUNIKATION → TEAM.
const OPERATOR_MAIN_ITEMS: NavItem[] = [
  {
    href: "/operator/organizations",
    label: "Verwaltung",
    iconKey: "buildingOffice",
    concept: "organizations",
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
    href: "/operator/announcements",
    label: "Ankündigungen",
    iconKey: "bell",
    concept: "announcements",
    alwaysShow: true,
  },
  {
    href: "/operator/operators",
    label: "Operatoren",
    iconKey: "group",
    concept: "operators",
    alwaysShow: true,
  },
];

// Additional navigation items that appear in the overflow menu
interface AdditionalNavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  concept?: MotoConceptKey;
  requiresAdmin?: boolean;
  // Show for admins or anyone holding this tenant permission (matches the
  // backend route gate). Use instead of alwaysShow for permission-gated pages.
  // An array shows the item when ANY listed permission is held.
  requiresPermission?: string | readonly string[];
  // All listed permissions are required (matching RequiresAllPermissions).
  requiresAllPermissions?: readonly string[];
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
    concept: "schools",
    alwaysShow: true,
  },
  {
    href: "/operator/accounts",
    label: "Konten",
    iconKey: "profile",
    concept: "accounts",
    alwaysShow: true,
  },
  {
    href: "/operator/devices",
    label: "Geräte",
    iconKey: "device",
    concept: "devices",
    alwaysShow: true,
  },
  {
    href: "/operator/persons",
    label: "Personen",
    iconKey: "userSingle",
    concept: "people",
    alwaysShow: true,
  },
  {
    href: "/operator/unregistered-tags",
    label: "Unbekannte RFID",
    iconKey: "security",
    concept: "rfid",
    alwaysShow: true,
  },
];

/**
 * Das Mehr-Menü ist dieselbe Liste wie die Seitenleiste (#2826): der Baum
 * in ~/lib/staff-navigation bestimmt Gruppe und Reihenfolge, hier stehen nur
 * Symbol und Sichtbarkeitsregel jeder Seite. Was die Reiter unten schon
 * zeigen, wird aus dem Menü entfernt; leere Gruppen fallen weg.
 */
const PAGE_ITEMS: readonly AdditionalNavItem[] = [
  {
    ...STAFF_FLAT_PAGES.dashboard,
    iconKey: "home",
    concept: "dashboard",
    requiresAdmin: true,
  },
  {
    // Tagesplan (#2383): Laufzeit-Gating (binary, timetable.enabled,
    // schedules:read) unten in isHrefEnabled, wie beim Reiter.
    ...STAFF_FLAT_PAGES.tagesplan,
    iconKey: "betreuungsplan",
    concept: "carePlan",
    hideForAdmin: true,
  },
  {
    ...STAFF_FLAT_PAGES.studentSearch,
    iconKey: "search",
    concept: "children",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.rooms,
    iconKey: "rooms",
    concept: "rooms",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.activities,
    iconKey: "activities",
    concept: "activities",
    alwaysShow: true,
  },
  {
    // Gemeinsame Übersicht für alle drei Vertretungsvorgänge.
    ...STAFF_FLAT_PAGES.substitutions,
    iconKey: "substitutions",
    concept: "groupAccess",
  },
  {
    // Anfragen-Modul (#2429). Gating unten in isHrefEnabled über
    // canOpenRequestsPage: requiresPermission kann das
    // users:absence+users:read-Paar nicht ausdrücken.
    ...STAFF_FLAT_PAGES.anfragen,
    iconKey: "tray",
    concept: "requests",
  },
  // Eltern: Nachrichten für alle, der Rest je Recht und Schulschalter
  // (isHrefEnabled), dieselben Regeln wie in der Seitenleiste.
  ...PARENT_SUB_PAGES.map((page) => ({
    href: page.href,
    label: page.label,
    iconKey: "parents" as const,
    concept: STAFF_NAV_CONCEPTS[page.href],
  })),
  {
    ...STAFF_FLAT_PAGES.timeTracking,
    iconKey: "clock",
    concept: "timeTracking",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.calendar,
    iconKey: "calendar",
    concept: "calendar",
    // Match the backend calendar:own gate on GET /api/calendar/my.
    requiresPermission: "calendar:own",
  },
  {
    ...STAFF_FLAT_PAGES.staff,
    iconKey: "staff",
    concept: "staff",
    alwaysShow: true,
  },
  // Team-Chat (#2598) ist Opt-in (isHrefEnabled); die Tagesinformationen
  // (#2180) sind wie die Route an users:read gebunden. Auf kleinen
  // Bildschirmen ist das Menü der einzige Zugang zu beiden.
  ...COMMUNICATION_SUB_PAGES.map((page) => ({
    href: page.href,
    label: page.label,
    iconKey: "chat" as const,
    concept: STAFF_NAV_CONCEPTS[page.href],
    ...(page.feature === "staffNotices"
      ? { requiresPermission: "users:read" }
      : {}),
  })),
  // Planung: der Katalog liefert alle Alt-Pfade als activePaths.
  ...PLANNING_SUB_PAGES.filter((page) => page.showInMobileNav).map((page) => ({
    href: page.href,
    label: page.label,
    iconKey: "betreuungsplan" as const,
    concept: STAFF_NAV_CONCEPTS[page.href],
    requiresAdmin: page.nonAdminPermission === undefined,
    requiresPermission: page.nonAdminPermission,
    activePaths: getPlanningMobileActivePaths(page.href),
  })),
  {
    // Tagesauswertung (#1456): Opt-in über gdpr.attendance_log_enabled
    // (isHrefEnabled), Recht wie die Route.
    ...STAFF_FLAT_PAGES.dayLog,
    iconKey: "calendar",
    concept: "dayReport",
    requiresPermission: "users:read",
  },
  {
    ...STAFF_FLAT_PAGES.statistics,
    iconKey: "chart",
    concept: "reports",
    requiresAllPermissions: ["config:read", "users:read"],
  },
  {
    // Dateiablage (#2596): jeder mit Tenant-Zugang; welche Ordner sichtbar
    // sind, entscheidet das Backend pro Ordner.
    ...STAFF_FLAT_PAGES.dateien,
    iconKey: "book",
    concept: "files",
    alwaysShow: true,
  },
  {
    // Info-Displays: Opt-in über display.enabled (isHrefEnabled).
    ...STAFF_FLAT_PAGES.infoDisplays,
    iconKey: "device",
    concept: "infoDisplays",
    requiresPermission: ["display:read", "display:manage"],
  },
  {
    ...STAFF_FLAT_PAGES.emergency,
    iconKey: "emergency",
    concept: "emergency",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.help,
    iconKey: "book",
    concept: "help",
    alwaysShow: true,
    newTab: true,
  },
  {
    ...STAFF_FLAT_PAGES.settings,
    iconKey: "settings",
    concept: "settings",
    requiresAdmin: true,
  },
];

const PAGE_ITEMS_BY_HREF = new Map(PAGE_ITEMS.map((item) => [item.href, item]));

/**
 * Die Akkordeon-Bereiche der Seitenleiste als je eine Zeile: Meine Gruppen
 * und Aktuelle Aufsicht führen auf ihre Übersicht, Datenverwaltung und
 * Anmeldungen auf ihre Hub-Seite; die Unterseiten erreicht man dort über
 * die Kacheln.
 */
const SECTION_ITEMS: Readonly<Record<StaffNavSectionKey, AdditionalNavItem>> = {
  groups: {
    href: "/ogs-groups",
    label: "Meine Gruppen",
    iconKey: "group",
    concept: "groups",
    alwaysShow: true,
  },
  supervisions: {
    href: "/active-supervisions",
    label: "Aktuelle Aufsicht",
    iconKey: "supervision",
    concept: "supervision",
    alwaysShow: true,
  },
  database: {
    href: DATABASE_SECTION.href,
    label: DATABASE_SECTION.label,
    iconKey: "database",
    concept: "database",
    requiresAdmin: true,
  },
  enrollments: {
    href: ENROLLMENT_SECTION.href,
    label: ENROLLMENT_SECTION.label,
    iconKey: "enrollments",
    concept: "enrollments",
    requiresAdmin: true,
    activePaths: ENROLLMENT_SUB_PAGES.map((page) => page.href),
  },
};

function itemForEntry(entry: StaffNavEntry): AdditionalNavItem | undefined {
  return entry.kind === "page"
    ? PAGE_ITEMS_BY_HREF.get(entry.href)
    : SECTION_ITEMS[entry.section];
}

interface DrawerGroup {
  readonly key: string;
  readonly label: string | null;
  readonly items: readonly AdditionalNavItem[];
}

// Tenant-scoped [tenant]/… routes that need the slug prefix in path-routing
// mode. PAGE_ITEMS and SECTION_ITEMS are the catalogs for every staff drawer
// entry; only /help is host-agnostic and must not carry the slug.
const TENANT_SCOPED_HREFS = new Set<string>([
  ...PAGE_ITEMS.filter((item) => item.href !== STAFF_FLAT_PAGES.help.href).map(
    (item) => item.href,
  ),
  ...Object.values(SECTION_ITEMS).map((item) => item.href),
]);

const NFC_ONLY_HREFS = new Set<string>(["/activities"]);

// Nav-Einträge, die im binären Anwesenheitsmodus verborgen bleiben (#2915).
// Gleiche fachliche Regel wie die Desktop-Sidebar (dortiges
// BINARY_HIDDEN_HREFS plus das separat gegatete Aufsicht-Accordion): Räume,
// Aktivitäten und Aufsicht sind Raum-/Besuchs-Konzepte ohne Bedeutung, wenn
// eine Schule nur in der Schule / nicht in der Schule erfasst. Die Seiten
// sperrt der BinaryModeGuard — ein Nav-Eintrag dorthin endet auf einer
// 404-Seite.
const BINARY_HIDDEN_HREFS = new Set<string>([
  "/rooms",
  "/activities",
  "/active-supervisions",
]);

interface MobileBottomNavProps {
  readonly className?: string;
}

export function MobileBottomNav({ className = "" }: MobileBottomNavProps) {
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
  // subdomain/operator/parent mode). Used for tenant-scoped navigation links.
  const tenantPath = useTenantAwarePath();
  const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);
  // Unter lg gibt es keine Shell-Kopfzeile mehr (Eltern-App-Muster), also
  // auch keinen Avatar mit Profilmenü: Profil und Abmelden wohnen hier im
  // „Mehr"-Menü, wie in der Eltern-App.
  const [logoutModalOpen, setLogoutModalOpen] = useState(false);
  const [staffPreviewModalOpen, setStaffPreviewModalOpen] = useState(false);

  // Refs for sliding indicator
  const navRefs = useRef<(HTMLAnchorElement | null)[]>([]);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });
  const [indicatorVisible, setIndicatorVisible] = useState(false);
  const isInitialMount = useRef(true);

  // Get session for role checking
  const { data: session } = useSession();
  const changeRequestAccess = useChangeRequestAccess();

  // Get supervision state
  const {
    hasGroups,
    isSupervising,
    isLoadingGroups,
    isLoadingSupervision,
    overviewEnabled,
  } = useOptionalSupervision();

  // Get shell auth mode
  const { mode, isSessionExpired, canStartStaffPreview, profileUrl } =
    useShellAuth();

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
  const baseMain =
    mode === "operator"
      ? resolvedOperatorMainItems
      : isCaregiver(session)
        ? STAFF_MAIN_ITEMS
        : hasEffectiveAdminScope(session)
          ? ADMIN_MAIN_ITEMS
          : STAFF_MAIN_ITEMS;
  // Callers covered by the school-wide overview (#2380): inject the
  // "Aufsicht" tab dynamically. This includes effective admins and verified
  // staff under all_staff. Gate on overviewEnabled (confirmed via
  // /supervisors/all 200) rather than just isSupervising so a synthetic
  // Schulhof entry does not surface the tab when the school keeps everyone
  // on their own supervisions.
  // STAFF_MAIN_ITEMS already contains /active-supervisions, so only inject
  // when it is missing (i.e. for admin-only users whose baseline is
  // ADMIN_MAIN_ITEMS) to avoid duplicate React keys.
  const alreadyHasSupervisionTab = baseMain.some(
    (item) => item.href === "/active-supervisions",
  );
  const filteredMainItems =
    !isLoadingSupervision && overviewEnabled && !alreadyHasSupervisionTab
      ? [
          ...baseMain.slice(0, 1),
          {
            href: "/active-supervisions",
            label: "Aufsicht",
            iconKey: "supervision" as const,
            concept: "supervision" as const,
            alwaysShow: true,
          },
          ...baseMain.slice(1),
        ]
      : baseMain;

  // Pre-compute permission flags to reduce complexity in filter
  const userIsAdmin = hasRole(session, "admin");
  const userHasEffectiveAdminScope = hasEffectiveAdminScope(session);
  const userIsCaregiver = isCaregiver(session);
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const showActivityNav = nfcEnabled && presenceMode !== "binary";
  const isBinaryMode = presenceMode === "binary";
  const hasGroupSupervision = !isLoadingGroups && hasGroups;
  const hasRoomSupervision = !isLoadingSupervision && isSupervising;

  // Gruppenübergaben (#1940) sind nur bei festen Gruppen sinnvoll.
  const openCareGroupMode = useOpenCareGroupMode();
  const staffMessagingEnabled = useStaffMessagingEnabled();
  // Betreuungsplan-Flag für den Tagesplan-Eintrag (#2383): vom Tenant-Resolve,
  // damit es auch ohne config:read aufgelöst ist.
  const tagesplanEnabled = useTimetableEnabled();
  const displayEnabled = useDisplayEnabled();
  const attendanceLogEnabled = useAttendanceLogEnabled();
  // Schulschalter aus dem Settings-Schema, mit demselben Lesemuster wie die
  // Desktop-Sidebar: für Admins und config:read-Halter (der Essensplan-
  // Eintrag hängt für sie an operations.meal_plan_enabled). `!== false` für
  // timetable.enabled, damit die Planungs-Einträge während des Schema-Ladens
  // nicht kurz verschwinden.
  const canReadConfig =
    mode === "teacher" &&
    (userIsAdmin || hasPermission(session, "config:read"));
  const { data: settingsSchema } = useSettingsSchema(canReadConfig, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });
  const timetableEnabled =
    getSettingValue(settingsSchema, "timetable.enabled") !== false;
  const parentNewsEnabled =
    getSettingValue(settingsSchema, "operations.parent_news_enabled") === true;
  const mealPlanEnabled =
    getSettingValue(settingsSchema, "operations.meal_plan_enabled") === true;
  // Elternmitteilungen (#1669) authoring is admin-only (admin:* wildcard on
  // every /api/parent-announcements route); same rule as the sidebar entry.
  const canAnnounce = hasPermission(session, "admin:*");

  // Filter additional navigation items based on permissions
  const filteredMainItemsByMode = filteredMainItems.filter(
    (item) =>
      (showActivityNav || !NFC_ONLY_HREFS.has(item.href)) &&
      // Binärer Anwesenheitsmodus (#2915): dieselbe Sichtbarkeitsregel wie in
      // der Desktop-Sidebar.
      !(isBinaryMode && BINARY_HIDDEN_HREFS.has(item.href)) &&
      (item.href !== "/ogs-groups" ||
        userIsCaregiver ||
        userHasEffectiveAdminScope) &&
      // Bei offener Betreuung gibt es keine "meine Gruppe" — der
      // gruppenbasierte Einstieg entfällt (#1544).
      !(openCareGroupMode && item.href === "/ogs-groups") &&
      // Tagesplan (#2383): nur im detaillierten Modus, nur an Schulen mit
      // aktiviertem Betreuungsplan (Flag vom Tenant-Resolve, ohne
      // config:read) und nur mit schedules:read — das Gate der Route
      // (/timetable/operations/planned-now), sonst wäre der Tab ein 403.
      !(
        item.href === "/tagesplan" &&
        (presenceMode === "binary" ||
          !tagesplanEnabled ||
          !hasPermission(session, "schedules:read"))
      ),
  );

  // Laufzeit-Regeln je Seite (Schulschalter, Anwesenheitsmodus, Rolle) —
  // dieselben wie in der Desktop-Sidebar, damit beide dieselben Seiten
  // zeigen.
  const isHrefEnabled = (href: string): boolean => {
    // Anfragen (#2429/#2911): dieselbe effektive Regel wie in Sidebar,
    // Seiten-Guard und Badge.
    if (href === "/anfragen") return changeRequestAccess.canOpenRequestsPage;
    if (href === "/ogs-groups") {
      // Bei offener Betreuung gibt es keine "meine Gruppe" (#1544).
      return (
        (userIsCaregiver || userHasEffectiveAdminScope) && !openCareGroupMode
      );
    }
    // Aufsicht wie in der Seitenleiste: eigene Aufsicht der Betreuungskräfte
    // oder die schulweite Übersicht (#2380); im binären Modus ohne Bedeutung.
    if (href === "/active-supervisions") {
      return (userIsCaregiver || overviewEnabled) && !isBinaryMode;
    }
    if (
      href === "/tagesplan" &&
      (isBinaryMode ||
        !tagesplanEnabled ||
        !hasPermission(session, "schedules:read"))
    ) {
      return false;
    }
    if (!showActivityNav && NFC_ONLY_HREFS.has(href)) return false;
    // Binärer Anwesenheitsmodus (#2915): auch im Mehr-Menü kein Link auf eine
    // Seite, die der BinaryModeGuard sperrt.
    if (isBinaryMode && BINARY_HIDDEN_HREFS.has(href)) return false;
    if (
      isPlanningPageHref(href) &&
      href !== "/calendar-periods" &&
      href !== "/payroll" &&
      !timetableEnabled
    ) {
      // Gilt auch für Nicht-Admins mit nonAdminPermission (#2283): die
      // Betreuungsplan-Leseansicht verschwindet mit timetable.enabled.
      return false;
    }
    // Team-Chat (#2598) ist Opt-in und faellt fail-closed: ohne eingeschalteten
    // Schalter taucht der Eintrag gar nicht erst auf, genau wie in der
    // Seitenleiste.
    if (href === "/team-chat" && !staffMessagingEnabled) return false;
    if (href === "/info-displays" && !displayEnabled) return false;
    if (href === "/day-log" && !attendanceLogEnabled) return false;
    // Eltern-Seiten: dieselben Regeln wie die Sidebar-Gruppe.
    if (href === "/admin/guardian-approvals") return userIsAdmin;
    if (href === "/parent-announcements") {
      return canAnnounce && parentNewsEnabled;
    }
    if (href === "/eltern/bankverbindungen") {
      return hasPermission(session, "guardians:financial");
    }
    if (href === "/meal-plan") {
      return (
        mealPlanEnabled &&
        (userIsAdmin || hasPermission(session, "config:read"))
      );
    }
    return true;
  };

  const isAdditionalItemVisible = (item: AdditionalNavItem): boolean => {
    if (!isHrefEnabled(item.href)) return false;
    // Hide items marked as hideForAdmin for admin users
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) {
      return false;
    }
    if (item.alwaysShow) return true;
    if (item.requiresAdmin) return userIsAdmin;
    if (item.requiresAllPermissions) {
      return (
        userIsAdmin ||
        item.requiresAllPermissions.every((permission) =>
          hasPermission(session, permission),
        )
      );
    }
    if (item.requiresPermission) {
      const required =
        typeof item.requiresPermission === "string"
          ? [item.requiresPermission]
          : item.requiresPermission;
      return (
        userIsAdmin ||
        required.some((permission) => hasPermission(session, permission))
      );
    }
    if (item.requiresSupervision && !userIsAdmin) {
      return hasGroupSupervision || hasRoomSupervision;
    }
    if (item.requiresActiveSupervision && !userIsAdmin) {
      return hasRoomSupervision;
    }
    return true;
  };

  // Static navigation - 4 main items + overflow drawer. Operator mode uses a
  // dedicated item list (the 4 sibling Verwaltung pages + Einstellungen) since
  // the bottom nav has only one Verwaltung slot.
  const displayMainItems: NavItem[] = filteredMainItemsByMode;
  const showOverflowMenu = true;
  // Avoid duplicates between main and additional
  const mainHrefs = new Set(
    displayMainItems.filter((i) => i.href !== "#").map((i) => i.href),
  );

  // Das Mehr-Menü aus dem Baum der Seitenleiste (#2826): die Startseite der
  // Rolle oben, dann die fünf Gruppen mit Überschrift, unten Notfall, Hilfe
  // und Einstellungen. Was die Reiter schon zeigen, fehlt hier; leere
  // Gruppen fallen weg.
  const entriesToItems = (entries: readonly StaffNavEntry[]) =>
    entries
      .map(itemForEntry)
      .filter((item): item is AdditionalNavItem => item !== undefined)
      .filter(
        (item) => isAdditionalItemVisible(item) && !mainHrefs.has(item.href),
      );

  const drawerGroups: readonly DrawerGroup[] =
    mode === "operator"
      ? [
          {
            key: "operator",
            label: null,
            items: resolvedOperatorAdditionalItems.filter(
              (i) => !mainHrefs.has(i.href),
            ),
          },
        ]
      : [
          { key: "top", label: null, items: entriesToItems(STAFF_NAV_TOP) },
          ...STAFF_NAV_GROUPS.map((group) => ({
            key: group.key,
            label: group.label,
            items: entriesToItems(group.entries),
          })),
          {
            key: "bottom",
            label: null,
            items: entriesToItems(STAFF_NAV_BOTTOM),
          },
        ];
  const visibleDrawerGroups = drawerGroups.filter(
    (group) => group.items.length > 0,
  );
  const displayAdditionalItems = visibleDrawerGroups.flatMap(
    (group) => group.items,
  );

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
          setIndicatorStyle(keepIfUnchanged(offsetLeft, offsetWidth));
          setIndicatorVisible(true);
        }
      } else if (isAnyAdditionalNavActive && moreButtonRef.current) {
        // "Mehr" button is active
        const { offsetLeft, offsetWidth } = moreButtonRef.current;
        setIndicatorStyle(keepIfUnchanged(offsetLeft, offsetWidth));
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
          setIndicatorStyle(keepIfUnchanged(offsetLeft, offsetWidth));
          setIndicatorVisible(true);
        }
      }
    }, 50);

    return () => clearTimeout(timer);
  }, [displayMainItems, isActiveRoute]);

  const moreLabel = "Mehr";

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
              <div className="space-y-5">
                {visibleDrawerGroups.map((group) => (
                  <div key={group.key} className="space-y-2">
                    {/* Gruppenüberschrift wie in der Seitenleiste; die
                        Start- und Fußzeilen stehen ohne. */}
                    {group.label && (
                      <p className="px-1 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                        {group.label}
                      </p>
                    )}
                    {group.items.map((item) => {
                      const isActive = isActiveRoute(
                        item.href,
                        item.activePaths,
                      );
                      const href = TENANT_SCOPED_HREFS.has(item.href)
                        ? tenantPath(item.href)
                        : item.href;

                      // Coming soon items are not clickable
                      if (item.comingSoon) {
                        return (
                          <div
                            key={item.label}
                            className="flex items-center gap-3 rounded-xl bg-gray-50 px-4 py-3 opacity-50"
                          >
                            <MobileNavIcon
                              item={item}
                              active={false}
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
                        <NavLink
                          key={item.href}
                          href={href}
                          onClick={closeOverflowMenu}
                          {...(item.newTab
                            ? { target: "_blank", rel: "noopener noreferrer" }
                            : {})}
                          className={`flex items-center gap-3 rounded-xl px-4 py-3 transition-all ${
                            isActive
                              ? "bg-gray-100 font-semibold text-gray-900"
                              : "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200"
                          } `}
                        >
                          <MobileNavIcon
                            item={item}
                            active={isActive}
                            className={`h-5 w-5 ${isActive ? "" : "text-gray-600"}`}
                          />
                          <span className="text-base font-medium">
                            {item.label}
                          </span>
                        </NavLink>
                      );
                    })}
                  </div>
                ))}
              </div>
              {(mode === "teacher" ||
                mode === "operator" ||
                isSessionExpired ||
                canStartStaffPreview) && (
                <div className="mt-4 space-y-2 border-t border-gray-100 pt-4">
                  {isSessionExpired ? (
                    <p className="bg-moto-red-soft text-moto-red-strong rounded-xl px-4 py-3 text-sm font-medium">
                      Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut
                      an.
                    </p>
                  ) : null}
                  {mode !== "parent" ? <RefreshButton drawer /> : null}
                  {mode === "teacher" ? (
                    <ButtonLink
                      href={tenantPath("/reminders")}
                      onClick={closeOverflowMenu}
                      variant="ghost"
                      size="touch"
                      className="w-full justify-start gap-3 px-4"
                    >
                      <Bell
                        className="h-5 w-5 text-gray-600"
                        aria-hidden="true"
                      />
                      <span className="text-base font-medium">
                        Erinnerungen
                      </span>
                    </ButtonLink>
                  ) : null}
                  {mode === "teacher" && canStartStaffPreview ? (
                    <Button
                      type="button"
                      onClick={() => {
                        closeOverflowMenu();
                        setStaffPreviewModalOpen(true);
                      }}
                      variant="ghost"
                      size="touch"
                      className="w-full justify-start gap-3 px-4"
                    >
                      <Eye
                        className="h-5 w-5 text-gray-600"
                        aria-hidden="true"
                      />
                      <span className="text-base font-medium">
                        Ansicht eines Mitarbeitenden
                      </span>
                    </Button>
                  ) : null}
                </div>
              )}
              {/* Konto-Zeilen wie im „Mehr"-Menü der Eltern-App: ohne
                  Shell-Kopfzeile gibt es mobil keinen Avatar mehr, Profil
                  und Abmelden brauchen deshalb hier einen Platz. */}
              <div className="mt-4 space-y-2 border-t border-gray-100 pt-4">
                {profileUrl ? (
                  <ButtonLink
                    href={
                      mode === "teacher" ? tenantPath(profileUrl) : profileUrl
                    }
                    onClick={closeOverflowMenu}
                    variant={isActiveRoute(profileUrl) ? "surface" : "ghost"}
                    size="touch"
                    className="w-full justify-start gap-3 px-4"
                  >
                    <MobileNavIcon
                      item={{ iconKey: "profile" }}
                      active={isActiveRoute(profileUrl)}
                      className="h-5 w-5 text-gray-600"
                    />
                    <span className="text-base font-medium">Profil</span>
                  </ButtonLink>
                ) : null}
                <Button
                  type="button"
                  onClick={() => {
                    closeOverflowMenu();
                    setLogoutModalOpen(true);
                  }}
                  variant="ghost"
                  size="touch"
                  className="w-full justify-start gap-3 px-4"
                >
                  <MobileNavIcon
                    item={{ iconKey: "profile", concept: "logout" }}
                    active={false}
                    className="h-5 w-5 text-gray-600"
                  />
                  <span className="text-base font-medium text-gray-900">
                    Abmelden
                  </span>
                </Button>
              </div>
            </div>
            <div className="pb-8" />
          </div>
        </DrawerContent>
      </Drawer>

      <LogoutModal
        isOpen={logoutModalOpen}
        onClose={() => setLogoutModalOpen(false)}
      />
      {mode === "teacher" && canStartStaffPreview ? (
        <StaffPreviewModal
          isOpen={staffPreviewModalOpen}
          onClose={() => setStaffPreviewModalOpen(false)}
        />
      ) : null}

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
                  className={`absolute top-0 h-full rounded-full bg-gray-100 ${
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
                      <MobileNavIcon
                        item={item}
                        active={false}
                        className="h-5 w-5 flex-shrink-0"
                      />
                    </button>
                  );
                }

                return (
                  <NavLink
                    key={item.href}
                    href={item.href}
                    ref={(el: HTMLAnchorElement | null) => {
                      navRefs.current[index] = el;
                    }}
                    aria-label={item.label}
                    className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                      isActive
                        ? "bg-gray-100 text-gray-900"
                        : "text-gray-400 hover:text-gray-600"
                    } `}
                  >
                    {/* Icon */}
                    <MobileNavIcon
                      item={item}
                      active={isActive}
                      className="h-5 w-5 flex-shrink-0"
                    />

                    {isActive && (
                      <span className="text-sm font-semibold whitespace-nowrap">
                        {item.label}
                      </span>
                    )}
                  </NavLink>
                );
              })}

              {/* More button */}
              {showOverflowMenu && (
                <button
                  ref={moreButtonRef}
                  type="button"
                  onClick={() => setIsOverflowMenuOpen(true)}
                  aria-label={moreLabel}
                  className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                    isOverflowMenuOpen || isAnyAdditionalNavActive
                      ? "bg-gray-100 text-gray-900"
                      : "text-gray-400 hover:text-gray-600"
                  } `}
                >
                  {/* Icon */}
                  <Icon
                    path={navigationIcons.more}
                    className="h-5 w-5 flex-shrink-0"
                  />

                  {(isOverflowMenuOpen || isAnyAdditionalNavActive) && (
                    <span className="text-sm font-semibold whitespace-nowrap">
                      {moreLabel}
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
