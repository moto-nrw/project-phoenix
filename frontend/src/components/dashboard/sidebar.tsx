// components/dashboard/sidebar.tsx
"use client";

import { Suspense, useCallback, useMemo } from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  useAttendanceLogEnabled,
  useStaffMessagingEnabled,
  useNFCEnabled,
  usePresenceMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { useShellAuth } from "~/lib/shell-auth-context";
import { hasPermission, hasRole, isCaregiver } from "~/lib/auth-utils";
import { canOpenRequestsPage } from "~/lib/change-request-access";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { operatorPath } from "~/lib/operator-url";
import { useSidebarAccordion } from "~/lib/hooks/use-sidebar-accordion";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useMessagesUnread } from "~/lib/hooks/use-messages-unread";
import { useStaffMessagesUnread } from "~/lib/hooks/use-staff-messages-unread";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useEnrollmentRequestsPending } from "~/lib/hooks/use-enrollment-requests-pending";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { SidebarAccordionSection } from "~/components/dashboard/sidebar-accordion-section";
import { SidebarSubItem } from "~/components/dashboard/sidebar-sub-item";
import { navigationIcons } from "~/lib/navigation-icons";
import { getSettingValue } from "~/lib/settings-api";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import {
  getActivePlanningSubPageHref,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import {
  ENROLLMENT_SUB_PAGES,
  getActiveEnrollmentSubPageHref,
  getActiveParentSubPageHref,
  getActiveReportsSubPageHref,
  getActiveTeamSubPageHref,
  PARENT_SECTION,
  PARENT_SUB_PAGES,
  PLANNING_SECTION,
  REPORTS_SECTION,
  REPORTS_SUB_PAGES,
  STAFF_FLAT_PAGES,
  TEAM_SECTION,
  TEAM_SUB_PAGES,
} from "~/lib/section-navigation";

// Type für Navigation Items
interface NavItem {
  href: string;
  label: string;
  icon: string;
  concept?: MotoConceptKey;
  requiresAdmin?: boolean;
  // Show when the caller holds this tenant permission (admins always pass). Use
  // instead of requiresAdmin for items open to more than admins, e.g. the
  // Änderungsanfragen queue (users:update, scoped per child in the backend).
  // An array shows the item when ANY listed permission is held (matching
  // backend RequiresAnyPermission routes).
  requiresPermission?: string | readonly string[];
  // All listed permissions are required (matching RequiresAllPermissions).
  requiresAllPermissions?: readonly string[];
  alwaysShow?: boolean;
  hideForAdmin?: boolean;
  comingSoon?: boolean;
  bottomPinned?: boolean;
  newTab?: boolean;
}

/**
 * Die flachen Bereiche der Seitenleiste. Die Bereiche mit Unterseiten
 * (Planung, Eltern, Team, Auswertung) stehen weiter unten als Akkordeon.
 *
 * Keine Farbe je Bereich mehr (BAUARTEN-SPEC, Teil 2): elf Akzentfarben ohne
 * Bedeutung entwerten das Rot, das „krank" heißt. Der aktive Eintrag wird
 * durch Fläche und Schriftschnitt markiert.
 */
const NAV_ITEMS: NavItem[] = [
  {
    ...STAFF_FLAT_PAGES.dashboard,
    icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
    concept: "dashboard",
    requiresAdmin: true,
  },
  {
    ...STAFF_FLAT_PAGES.studentSearch,
    icon: navigationIcons.userSingle,
    concept: "children",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.staff,
    concept: "staff",
    icon: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.rooms,
    icon: navigationIcons.rooms,
    concept: "rooms",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.activities,
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    concept: "activities",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.emergency,
    icon: navigationIcons.emergency,
    concept: "emergency",
    alwaysShow: true,
    bottomPinned: true,
  },
  {
    ...STAFF_FLAT_PAGES.help,
    icon: navigationIcons.book,
    concept: "help",
    alwaysShow: true,
    bottomPinned: true,
    newTab: true,
  },
  {
    ...STAFF_FLAT_PAGES.settings,
    concept: "settings",
    icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM15 12a3 3 0 11-6 0 3 3 0 016 0z",
    requiresAdmin: true,
    bottomPinned: true,
  },
];

interface OperatorNavSection {
  readonly label: string;
  readonly items: readonly NavItem[];
}

// Operator navigation, grouped into static labeled sections. Icons sourced
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
        concept: "organizations",
        alwaysShow: true,
      },
      {
        href: "/operator/schools",
        label: "Schulen",
        icon: navigationIcons.buildingOffice,
        concept: "schools",
        alwaysShow: true,
      },
      {
        href: "/operator/accounts",
        label: "Konten",
        icon: navigationIcons.profile,
        concept: "accounts",
        alwaysShow: true,
      },
      {
        href: "/operator/devices",
        label: "Geräte",
        icon: navigationIcons.device,
        concept: "devices",
        alwaysShow: true,
      },
      {
        href: "/operator/persons",
        label: "Personen",
        icon: navigationIcons.userSingle,
        concept: "people",
        alwaysShow: true,
      },
      {
        href: "/operator/unregistered-tags",
        label: "Unbekannte RFID",
        icon: navigationIcons.security,
        concept: "rfid",
        alwaysShow: true,
      },
    ],
  },
  {
    label: "KOMMUNIKATION",
    items: [
      {
        href: "/operator/announcements",
        label: "Ankündigungen",
        icon: navigationIcons.bell,
        concept: "announcements",
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
        concept: "operators",
        alwaysShow: true,
      },
    ],
  },
];

const NFC_ONLY_HREFS = new Set<string>([
  "/activities",
  "/database/activities",
  "/database/devices",
]);

// Nav items hidden in binary-mode tenants. Rooms and Activities are room/visit
// concepts with no operational meaning when the tenant only tracks
// in-school/out-of-school on active.attendance. The Aktuelle-Aufsicht
// accordion is gated separately below (it's not in NAV_ITEMS).
const BINARY_HIDDEN_HREFS = new Set<string>(["/rooms", "/activities"]);

interface SidebarProps {
  readonly className?: string;
}

function SidebarContent({ className = "" }: SidebarProps) {
  const tParentNav = useTranslations("parentNav");
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  // Prefixes tenant-scoped hrefs with the slug in path-routing mode (no-op in
  // subdomain/operator mode). Used for tenant-scoped navigation links.
  const tenantPath = useTenantAwarePath();
  const { data: session } = useSession();
  const { mode } = useShellAuth();
  // Compare every active state against clean tenant-internal paths. The helper
  // only strips in path-routing mode, avoiding slug/route collisions on tenant
  // subdomains.
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );

  // Offene Abwesenheitsanträge (vacation:approve, #1419). Sie zählen seit
  // #2433 auf das Anfragen-Modul ein, wo sie auch entschieden werden — der
  // Mitarbeiter-Eintrag trägt kein eigenes Badge mehr.
  const { unreadCount: staffAbsencesPendingCount } = useStaffAbsencesPending();
  // Unread parent-OGS messages badge (staff/teacher mode)
  const { unreadCount: messagesUnreadCount } = useMessagesUnread();
  // Ungelesene Team-Chat-Nachrichten (#2598). Eigener Zähler, damit eine
  // Eltern-Nachricht nie den Team-Badge hochzählt und umgekehrt.
  const { unreadCount: teamChatUnreadCount } = useStaffMessagesUnread();
  // Pending parent change-requests badge (Änderungsanfragen; users:update,
  // scoped per child in the backend so the count reflects the caller's own
  // group's requests)
  const { unreadCount: changeRequestsPendingCount } =
    useChangeRequestsPending();
  // Offene Anmeldungsänderungen (config:manage, #2435). Sie zählen seit dem
  // Umzug ins Anfragen-Modul auf dasselbe Badge ein.
  const { unreadCount: enrollmentRequestsPendingCount } =
    useEnrollmentRequestsPending();
  const { unreadCount: careWithdrawalsPendingCount } =
    useCareWithdrawalsPending();
  const requestsPendingCount =
    changeRequestsPendingCount +
    staffAbsencesPendingCount +
    enrollmentRequestsPendingCount +
    careWithdrawalsPendingCount;

  // Accordion state passes `from` param so child pages (e.g. student detail)
  // keep the originating accordion section open
  const fromParam = searchParams.get("from");
  const { expanded, toggle } = useSidebarAccordion(pathname, fromParam);

  const userIsAdmin = hasRole(session, "admin");
  const userIsCaregiver = isCaregiver(session);
  // Elternmitteilungen (#1669) authoring is ADMIN-ONLY in v1: every
  // /api/parent-announcements route is guarded by the admin:* wildcard
  // (backend api/announcement/api.go), because the service does no per-caller
  // audience scoping. Mirror that exact permission so a non-admin never sees a
  // nav entry whose every list/create/publish call would 403. A future
  // delegated-announcer role must relax the backend route first.
  const canAnnounce = hasPermission(session, "admin:*");
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";
  const nfcEnabled = useNFCEnabled();
  const attendanceLogEnabled = useAttendanceLogEnabled();
  const staffMessagingEnabled = useStaffMessagingEnabled();
  // Fetch the settings schema for anyone the backend lets read config, not just
  // admins. The meal-plan GET route is guarded by config:read, so a non-admin
  // config reader (custom config-manager) must also see the feature flags;
  // gating the fetch on userIsAdmin alone hid the Essensplan entry from them.
  // Admins stay in (their other flag, timetable.enabled, depends on this too).
  // Announcers are admin:* holders (see canAnnounce), who satisfy config:read
  // via the wildcard, so they are already covered here.
  const canReadConfig = userIsAdmin || hasPermission(session, "config:read");
  const { data: settingsSchema } = useSettingsSchema(canReadConfig, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });

  const parentNewsEnabled =
    getSettingValue(settingsSchema, "operations.parent_news_enabled") === true;

  const mealPlanEnabled =
    getSettingValue(settingsSchema, "operations.meal_plan_enabled") === true;

  // Planung-Akkordeon (#1946): nur verstecken, wenn timetable.enabled explizit
  // false liefert — `!== false` statt `=== true`, damit der Bereich beim
  // Schema-Laden nicht kurz verschwindet (gleiches Muster wie das Route-Gate
  // in betreuungsplan-view.tsx).
  const timetableEnabled =
    getSettingValue(settingsSchema, "timetable.enabled") !== false;

  // Kalenderzeiträume und Abrechnung bleiben auch bei abgeschaltetem
  // Planungsbereich erreichbar: Anmeldephasen verknüpfen sich mit
  // Kalenderzeiträumen; Abrechnung ist unabhängig davon über config:manage
  // geschützt.
  const planningSubPages = PLANNING_SUB_PAGES.filter((page) => {
    // Nicht-Admins sehen nur Seiten mit gehaltener nonAdminPermission
    // (#2283); das timetable.enabled-Gate darunter gilt für alle.
    if (
      !userIsAdmin &&
      (page.nonAdminPermission === undefined ||
        !hasPermission(session, page.nonAdminPermission))
    ) {
      return false;
    }
    return timetableEnabled || page.independentOfTimetable === true;
  });

  // Visible "Eltern" accordion sub-pages. Same per-item gating the flat
  // NAV_ITEMS carried before consolidation: overview + Nachrichten for all
  // staff, the rest per permission / feature flag.
  const parentSubPages = useMemo(
    () =>
      PARENT_SUB_PAGES.filter((page) => {
        switch (page.feature) {
          case "overview":
          case "messages":
            return true;
          case "requests":
            return canOpenRequestsPage(session);
          case "approvals":
            return userIsAdmin;
          case "announcements":
            return canAnnounce && parentNewsEnabled;
          case "mealPlan":
            return (
              mealPlanEnabled === true &&
              (userIsAdmin || hasPermission(session, "config:read"))
            );
        }
      }),
    [userIsAdmin, session, canAnnounce, parentNewsEnabled, mealPlanEnabled],
  );

  // Eltern badge: ungelesene Elternnachrichten plus offene Anfragen. Der
  // Anfragen-Eintrag steht seit dem Navigationsumbau in diesem Bereich, also
  // zählt sein Badge auch hier mit.
  const parentSectionBadgeCount = messagesUnreadCount + requestsPendingCount;

  // Team (#2598/#2596): Team-Chat ist Opt-in
  // (operations.staff_messaging_enabled, Default aus), die Dateiablage sieht
  // jeder mit Tenant-Zugang — welche Ordner, entscheidet das Backend.
  const teamSubPages = useMemo(
    () =>
      TEAM_SUB_PAGES.filter(
        (page) =>
          page.href !== STAFF_FLAT_PAGES.teamChat.href || staffMessagingEnabled,
      ),
    [staffMessagingEnabled],
  );

  // Auswertung: dieselben Rechte-Regeln, die die Einträge vorher flach
  // getragen haben.
  const reportsSubPages = useMemo(
    () =>
      REPORTS_SUB_PAGES.filter((page) => {
        switch (page.href) {
          case STAFF_FLAT_PAGES.statistics.href:
            // Das Backend verlangt config:read UND users:read.
            return (
              userIsAdmin ||
              (hasPermission(session, "config:read") &&
                hasPermission(session, "users:read"))
            );
          case STAFF_FLAT_PAGES.payroll.href:
            return userIsAdmin || hasPermission(session, "config:manage");
          case STAFF_FLAT_PAGES.dayLog.href:
            // Tagesbericht (#1456) hängt am Anwesenheitsprotokoll-Gate
            // (gdpr.attendance_log_enabled, Opt-in, Default aus).
            return (
              attendanceLogEnabled &&
              (userIsAdmin || hasPermission(session, "users:read"))
            );
          default:
            return true;
        }
      }),
    [userIsAdmin, session, attendanceLogEnabled],
  );

  // Filter flat navigation items based on permissions
  const filteredNavItems = NAV_ITEMS.filter((item) => {
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) return false;
    if (!nfcEnabled && NFC_ONLY_HREFS.has(item.href)) return false;
    if (isBinaryMode && BINARY_HIDDEN_HREFS.has(item.href)) return false;
    if (item.alwaysShow) return true;
    // Permission-gated items (e.g. Änderungsanfragen on users:update): show for
    // admins or anyone holding the permission (any of them, for arrays),
    // matching the backend route gate.
    if (item.requiresPermission && !userIsAdmin) {
      const required =
        typeof item.requiresPermission === "string"
          ? [item.requiresPermission]
          : item.requiresPermission;
      if (!required.some((p) => hasPermission(session, p))) return false;
    }
    if (
      item.requiresAllPermissions &&
      !userIsAdmin &&
      !item.requiresAllPermissions.every((p) => hasPermission(session, p))
    )
      return false;
    if (item.requiresAdmin && !userIsAdmin) return false;
    return true;
  });

  // Helper to determine active href for student detail pages based on referrer
  const getStudentDetailActiveHref = (from: string | null): string => {
    if (!from) return "/students/search";
    if (from.startsWith("/ogs-groups")) return "/ogs-groups";
    if (from.startsWith("/active-supervisions")) return "/active-supervisions";
    if (from.startsWith("/day-log")) return "/day-log";
    // Drill-in from a room ("Kinder im Raum"), both the legacy subpage
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
  // - Anywhere under /organizations or /organizations/{slug}: Träger
  // - Anywhere under /organizations/{slug}/schools/{schoolSlug}: Schulen
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
    if (href === "/parents") return pathname === "/parents" || pathname === "/";
    // Schul-Portal (#2207): auf dem Schul-Host ist die Klassenansicht die
    // /staff/dienstplan has its own sidebar entry — don't also light up "Mitarbeiter"
    if (href === "/staff") {
      return (
        pathname.startsWith("/staff") &&
        !pathname.startsWith("/staff/dienstplan")
      );
    }
    // /calendar-periods gehört zum Betreuungsplan, nicht zum Kalender —
    // ohne Exakt-Match würde der /calendar-Eintrag per Präfix mitleuchten.
    if (href === "/calendar") {
      return pathname === "/calendar" || pathname.startsWith("/calendar/");
    }
    if (href.startsWith("/parents/")) {
      // On the parents host the proxy rewrites /parents/* internally while the
      // browser (and usePathname) shows the external path without the prefix —
      // /parents/news is visited as /news. Match both spellings.
      return (
        pathname.startsWith(href) ||
        pathname.startsWith(href.slice("/parents".length))
      );
    }
    return pathname.startsWith(href);
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

  // Ein Symbolformat für alle Einträge. Farbe je Bereich gibt es nicht mehr;
  // das Symbol nimmt die Textfarbe des Eintrags an.
  const iconClasses =
    "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5 transition-colors";

  const renderNavIcon = (item: NavItem) => {
    const concept = item.concept ? MOTO_CONCEPTS[item.concept] : null;
    const isActive = !item.comingSoon && isActiveLink(item.href);
    if (concept) {
      const ConceptIcon = concept.icon;
      return (
        <ConceptIcon
          size={22}
          weight={isActive ? "fill" : "regular"}
          className={iconClasses}
          aria-hidden="true"
        />
      );
    }

    return (
      <svg
        className={iconClasses}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        aria-hidden="true"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d={item.icon}
        />
      </svg>
    );
  };

  const renderNavItem = (item: NavItem) => (
    <div key={item.comingSoon ? item.label : item.href}>
      {item.comingSoon ? (
        <div
          className={`group ${getLinkClasses(item.href, true)}`}
          title={tParentNav("comingSoonTooltip")}
        >
          {renderNavIcon(item)}
          <span>{item.label}</span>
          <span className="ml-2 rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 opacity-0 transition-opacity group-hover:opacity-100">
            Bald
          </span>
        </div>
      ) : (
        <Link
          href={tenantPath(item.href)}
          className={getLinkClasses(item.href)}
          {...(item.newTab
            ? { target: "_blank", rel: "noopener noreferrer" }
            : {})}
        >
          {renderNavIcon(item)}
          <span className="flex flex-1 items-center justify-between">
            {item.label}
          </span>
        </Link>
      )}
    </div>
  );

  // Coming soon items
  const comingSoonItems = mainNavItems.filter((item) => item.comingSoon);

  // Die Sammlung „Meine Gruppen" und die Aufsicht hingen bis zum
  // Navigationsumbau als dynamische Unterpunkte in der Seitenleiste — eine
  // Navigation, die mit den Daten wächst. Sie sind jetzt Reiter an ihrer
  // Sammlung (Kinder bzw. Räume); die Seitenleiste kennt nur noch die
  // Bereiche. Die Merker für die zuletzt gewählte Gruppe bzw. Aufsicht
  // schreiben weiterhin die Seiten selbst.

  const activeEnrollmentSubPageHref = getActiveEnrollmentSubPageHref(pathname);
  const isOnEnrollmentsPage = activeEnrollmentSubPageHref !== null;

  const activePlanningSubPageHref = getActivePlanningSubPageHref(pathname);
  const isOnPlanningPage = activePlanningSubPageHref !== null;

  // Hub = die erste sichtbare Unterseite: der Betreuungsplan, bzw. bei
  // abgeschaltetem timetable.enabled die Kalenderzeiträume — sonst führte der
  // Header-Klick auf die "deaktiviert"-Hinweisseite.
  const planningHubHref = planningSubPages[0]?.href ?? "/betreuungsplan";

  const handlePlanningToggle = useCallback(() => {
    // Navigate-on-expand wie bei den anderen Akkordeons, damit der Klick auf
    // den Bereichs-Header immer auf einer nützlichen Seite landet.
    const onSection = getActivePlanningSubPageHref(pathname) !== null;
    if (!onSection) {
      toggle("planning");
      router.push(planningHubHref);
    } else if (pathname === planningHubHref) {
      toggle("planning");
    } else {
      router.push(planningHubHref);
    }
  }, [toggle, pathname, router, planningHubHref]);

  const activeParentSubPageHref = getActiveParentSubPageHref(pathname);
  const isOnParentPage = activeParentSubPageHref !== null;

  const activeTeamSubPageHref = getActiveTeamSubPageHref(pathname);
  const isOnTeamPage = activeTeamSubPageHref !== null;
  const teamHubHref = teamSubPages[0]?.href ?? STAFF_FLAT_PAGES.dateien.href;

  const handleTeamToggle = useCallback(() => {
    if (getActiveTeamSubPageHref(pathname) === null) {
      toggle("team");
      router.push(teamHubHref);
    } else if (pathname === teamHubHref) {
      toggle("team");
    } else {
      router.push(teamHubHref);
    }
  }, [toggle, pathname, router, teamHubHref]);

  const activeReportsSubPageHref = getActiveReportsSubPageHref(pathname);
  const isOnReportsPage = activeReportsSubPageHref !== null;
  const reportsHubHref =
    reportsSubPages[0]?.href ?? STAFF_FLAT_PAGES.timeTracking.href;

  const handleReportsToggle = useCallback(() => {
    if (getActiveReportsSubPageHref(pathname) === null) {
      toggle("reports");
      router.push(reportsHubHref);
    } else if (pathname === reportsHubHref) {
      toggle("reports");
    } else {
      router.push(reportsHubHref);
    }
  }, [toggle, pathname, router, reportsHubHref]);

  const handleParentToggle = useCallback(() => {
    // Hub = the /eltern overview. Mirrors the other accordions'
    // navigate-on-expand behavior so the section label lands on a real page.
    const onSection = getActiveParentSubPageHref(pathname) !== null;
    if (!onSection) {
      toggle("eltern");
      router.push("/eltern");
    } else if (pathname === "/eltern") {
      toggle("eltern");
    } else {
      router.push("/eltern");
    }
  }, [toggle, pathname, router]);

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
  // Operator mode: sectioned navigation (static labels, no accordions)
  if (mode === "operator") {
    const renderOperatorItem = (item: NavItem) => (
      <Link
        key={item.href}
        href={item.href}
        className={getLinkClasses(item.href)}
      >
        {renderNavIcon(item)}
        <span className="flex flex-1 items-center justify-between">
          {item.label}
        </span>
      </Link>
    );

    return (
      <aside
        className={`min-h-screen w-64 border-r border-gray-200/70 bg-white/95 ${className}`}
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
        </div>
      </aside>
    );
  }

  return (
    <aside
      className={`min-h-screen w-64 border-r border-gray-200/70 bg-white/95 ${className}`}
    >
      <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
        {/* Main navigation, scrollable */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-3 lg:p-4 xl:p-3">
          {/* Die fünf flachen Bereiche: Start, Kinder, Mitarbeitende, Räume,
              Aktivitäten. Reihenfolge kommt aus NAV_ITEMS. */}
          {mainNavItems.filter((item) => !item.comingSoon).map(renderNavItem)}

          {/* Planung (#1946) — Betreuungsplan, Dienstplan, Vertretung,
              Tageslisten, Zeiträume, Kalender. Bei explizit ausgeschaltetem
              timetable.enabled bleiben die Seiten übrig, die nicht daran
              hängen. Nicht-Admins sehen nur Seiten mit gehaltener
              nonAdminPermission (heute: Kalender). */}
          {planningSubPages.length > 0 && (
            <SidebarAccordionSection
              icon={navigationIcons.betreuungsplan}
              concept="carePlan"
              label={PLANNING_SECTION.label}
              isExpanded={expanded === "planning"}
              onToggle={handlePlanningToggle}
              isActive={isOnPlanningPage}
              isIconActive={isOnPlanningPage}
              hasChildren={planningSubPages.length > 0}
            >
              {planningSubPages.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  // Tenant-scoped [tenant]/… Routen: im Path-Routing-Modus
                  // via tenantPath prefixen (No-op im Subdomain-Modus).
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={activePlanningSubPageHref === page.href}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Eltern — alle Flächen, die mit Eltern zu tun haben: Nachrichten,
              Anfragen, Neue Elternkonten, Mitteilungen, Essensplan und die
              Anmeldungen. Der Anmeldungsblock stand bis zum
              Navigationsumbau als eigener Bereich daneben. */}
          <SidebarAccordionSection
            icon={navigationIcons.parents}
            concept="parents"
            label={PARENT_SECTION.label}
            isExpanded={expanded === "eltern"}
            onToggle={handleParentToggle}
            isActive={isOnParentPage || isOnEnrollmentsPage}
            isIconActive={isOnParentPage || isOnEnrollmentsPage}
            hasChildren={parentSubPages.length > 0}
            badgeCount={parentSectionBadgeCount}
          >
            {parentSubPages.map((page) => (
              <SidebarSubItem
                key={page.href}
                // Every Eltern sub-page is a tenant-scoped [tenant]/… route
                // (/eltern, /messages, /admin/guardian-approvals, /meal-plan,
                // …). In path-routing mode a bare href is either captured as
                // the tenant slug ("/eltern") or leaves the current tenant path
                // entirely, so prefix all of them via tenantPath. No-op in
                // subdomain mode.
                href={tenantPath(page.href)}
                label={page.label}
                isActive={activeParentSubPageHref === page.href}
                badgeCount={
                  page.feature === "messages"
                    ? messagesUnreadCount
                    : page.feature === "requests"
                      ? requestsPendingCount
                      : 0
                }
              />
            ))}
            {userIsAdmin &&
              ENROLLMENT_SUB_PAGES.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={activeEnrollmentSubPageHref === page.href}
                />
              ))}
          </SidebarAccordionSection>

          {/* Team — die internen Flächen der OGS. */}
          {teamSubPages.length > 0 && (
            <SidebarAccordionSection
              icon={navigationIcons.chat}
              concept="messages"
              label={TEAM_SECTION.label}
              isExpanded={expanded === "team"}
              onToggle={handleTeamToggle}
              isActive={isOnTeamPage}
              isIconActive={isOnTeamPage}
              hasChildren={teamSubPages.length > 0}
              badgeCount={teamChatUnreadCount}
            >
              {teamSubPages.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={activeTeamSubPageHref === page.href}
                  badgeCount={
                    page.href === STAFF_FLAT_PAGES.teamChat.href
                      ? teamChatUnreadCount
                      : 0
                  }
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Auswertung — Zahlen über einen Zeitraum. */}
          {reportsSubPages.length > 0 && (
            <SidebarAccordionSection
              icon={navigationIcons.chart}
              concept="reports"
              label={REPORTS_SECTION.label}
              isExpanded={expanded === "reports"}
              onToggle={handleReportsToggle}
              isActive={isOnReportsPage}
              isIconActive={isOnReportsPage}
              hasChildren={reportsSubPages.length > 0}
            >
              {reportsSubPages.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={activeReportsSubPageHref === page.href}
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
          className={`min-h-screen w-64 border-r border-gray-200/70 bg-white/95 ${className}`}
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
