// components/dashboard/sidebar.tsx
"use client";

import { Suspense, useCallback, useEffect, useMemo } from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  useAttendanceLogEnabled,
  useStaffMessagingEnabled,
  useDisplayEnabled,
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import { hasPermission, hasRole, isCaregiver } from "~/lib/auth-utils";
import { canOpenRequestsPage } from "~/lib/change-request-access";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { operatorPath } from "~/lib/operator-url";
import { useSidebarAccordion } from "~/lib/hooks/use-sidebar-accordion";
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useMessagesUnread } from "~/lib/hooks/use-messages-unread";
import { useStaffMessagesUnread } from "~/lib/hooks/use-staff-messages-unread";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useEnrollmentRequestsPending } from "~/lib/hooks/use-enrollment-requests-pending";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useGroupAttendanceCounts } from "~/lib/group-attendance-count-context";
import { SidebarAccordionSection } from "~/components/dashboard/sidebar-accordion-section";
import { SidebarSubItem } from "~/components/dashboard/sidebar-sub-item";
import { navigationIcons } from "~/lib/navigation-icons";
import { getSettingValue } from "~/lib/settings-api";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { NotificationBadge } from "~/components/ui/notification-badge";
import {
  getActivePlanningSubPageHref,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import {
  DATABASE_SECTION,
  DATABASE_SUB_PAGES,
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  getActiveEnrollmentSubPageHref,
  getActiveParentSubPageHref,
  PARENT_SECTION,
  PARENT_SUB_PAGES,
  PLANNING_SECTION,
  STAFF_FLAT_PAGES,
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
  activeColor?: string;
  newTab?: boolean;
}

// Flat navigation items (excludes accordion sections: ogs-groups, active-supervisions, database)
const NAV_ITEMS: NavItem[] = [
  {
    ...STAFF_FLAT_PAGES.dashboard,
    icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
    concept: "dashboard",
    activeColor: "text-moto-blue",
    requiresAdmin: true,
  },
  {
    ...STAFF_FLAT_PAGES.studentSearch,
    icon: navigationIcons.userSingle,
    concept: "children",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.activities,
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    concept: "activities",
    activeColor: "text-moto-coral",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.rooms,
    icon: navigationIcons.rooms,
    concept: "rooms",
    activeColor: "text-moto-navy",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.staff,
    concept: "staff",
    icon: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2",
    activeColor: "text-moto-orange",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.teamChat,
    // Kein eigenes moto-Konzept-Icon: der Team-Chat ist eine neue Fläche und
    // teilt sich die Sprechblasen-Form mit den Eltern-Nachrichten, nur in der
    // Mitarbeitenden-Farbe statt in Blau.
    icon: "M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z",
    activeColor: "text-moto-orange",
    alwaysShow: true,
  },
  {
    ...STAFF_FLAT_PAGES.calendar,
    icon: navigationIcons.calendar,
    concept: "calendar",
    activeColor: "text-moto-indigo",
    // Backend gates GET /api/calendar/my on calendar:own; match it so
    // restricted/custom roles without the permission don't land on a 403 page.
    requiresPermission: "calendar:own",
  },
  {
    // Alt-Seite mit eigenem Datenmodell (education.group_substitution):
    // vergibt temporären Gruppen-Datenzugriff, keine Personalplanung — daher
    // "Gruppenzugriff" zur Abgrenzung vom Planungsbereich "Vertretung"
    // (#1940). Nur relevant bei festen Gruppen (operations.group_mode);
    // Gating siehe substitutionsItem-Rendering unten.
    ...STAFF_FLAT_PAGES.substitutions,
    icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15",
    concept: "groupAccess",
    activeColor: "text-moto-purple",
    requiresAdmin: true,
  },
  {
    ...STAFF_FLAT_PAGES.infoDisplays,
    concept: "infoDisplays",
    icon: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z",
    activeColor: "text-moto-blue",
    requiresPermission: ["display:read", "display:manage"],
  },
  {
    ...STAFF_FLAT_PAGES.timeTracking,
    icon: "M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0Z",
    concept: "timeTracking",
    alwaysShow: true,
  },
  {
    // Tagesauswertung (#1456): rückwirkender Tagesstatus pro Gruppe
    // (Anwesend/Krank/Entschuldigt/Abwesend). Als Auswertung unten bei den
    // Berichts-Einträgen einsortiert. Opt-in über
    // gdpr.attendance_log_enabled — Gating siehe filteredNavItems.
    ...STAFF_FLAT_PAGES.dayLog,
    concept: "dayReport",
    icon: "M9 12h6m-6 4h6M9 8h6M5 21h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v14a2 2 0 002 2zM9 3v2m6-2v2",
    activeColor: "text-moto-green",
    requiresPermission: "users:read",
  },
  {
    // Anfragen-Modul (#2429): eingereichte Wünsche von Eltern und
    // Mitarbeitenden an einem Ort. Sichtbarkeit hängt an zwei getrennten
    // Rechte-Regeln (Eltern-Reiter bzw. Mitarbeitende-Reiter) — Gating unten
    // in filteredNavItems über canOpenRequestsPage, weil requiresPermission
    // das users:absence+users:read-Paar nicht ausdrücken kann.
    ...STAFF_FLAT_PAGES.anfragen,
    icon: navigationIcons.tray,
    concept: "requests",
    activeColor: "text-moto-blue",
  },
  {
    // Dateiablage (#2596): jeder mit Tenant-Zugang sieht den Eintrag; welche
    // Ordner sichtbar sind, entscheidet das Backend pro Ordner.
    ...STAFF_FLAT_PAGES.dateien,
    icon: "M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z",
    concept: "files",
    activeColor: "text-moto-indigo",
    alwaysShow: true,
  },
  {
    // Statistik (#2606): Anwesenheitsquoten je Kind, Gruppe und Zeitraum
    // plus Raumauslastung. Das Backend verlangt config:read UND users:read.
    ...STAFF_FLAT_PAGES.statistics,
    icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
    requiresAllPermissions: ["config:read", "users:read"],
    concept: "reports",
    activeColor: "text-moto-blue",
  },
  {
    ...STAFF_FLAT_PAGES.emergency,
    icon: navigationIcons.emergency,
    concept: "emergency",
    activeColor: "text-moto-wine",
    alwaysShow: true,
    bottomPinned: true,
  },
  {
    ...STAFF_FLAT_PAGES.help,
    icon: navigationIcons.book,
    concept: "help",
    activeColor: "text-moto-green",
    alwaysShow: true,
    bottomPinned: true,
    newTab: true,
  },
  {
    ...STAFF_FLAT_PAGES.settings,
    concept: "settings",
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
const OPERATOR_VERWALTUNG_ACTIVE_COLOR = "text-moto-blue";

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
        activeColor: "text-moto-petrol",
        alwaysShow: true,
      },
      {
        href: "/operator/schools",
        label: "Schulen",
        icon: navigationIcons.buildingOffice,
        concept: "schools",
        activeColor: "text-moto-gold",
        alwaysShow: true,
      },
      {
        href: "/operator/accounts",
        label: "Konten",
        icon: navigationIcons.profile,
        concept: "accounts",
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/devices",
        label: "Geräte",
        icon: navigationIcons.device,
        concept: "devices",
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/persons",
        label: "Personen",
        icon: navigationIcons.userSingle,
        concept: "people",
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
        alwaysShow: true,
      },
      {
        href: "/operator/unregistered-tags",
        label: "Unbekannte RFID",
        icon: navigationIcons.security,
        concept: "rfid",
        activeColor: OPERATOR_VERWALTUNG_ACTIVE_COLOR,
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
        activeColor: "text-moto-amber",
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
        activeColor: "text-violet-500",
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

/**
 * Determine if a supervision sub-item should be highlighted as active.
 * Sessions are keyed by active-group ID (`?session=`, #2265); the legacy
 * room key still resolves for old links and stored state.
 */
function isRoomSubItemActive(
  childSessionId: string | null,
  childRoomId: string | null,
  sessionId: string,
  roomId: string,
  pathname: string,
  currentSessionParam: string | null,
  currentRoomParam: string | null,
  index: number,
): boolean {
  if (childSessionId) return childSessionId === sessionId;
  if (childRoomId) return childRoomId === roomId;
  if (!pathname.startsWith("/active-supervisions")) return false;
  if (currentSessionParam) return currentSessionParam === sessionId;
  if (currentRoomParam) return currentRoomParam === roomId;
  return index === 0;
}

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

  // Get supervision state
  const {
    isLoadingGroups,
    isLoadingSupervision,
    groups,
    supervisedRooms,
    overviewEnabled,
  } = useOptionalSupervision();

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
  const displayEnabled = useDisplayEnabled();
  const attendanceLogEnabled = useAttendanceLogEnabled();
  const staffMessagingEnabled = useStaffMessagingEnabled();
  const { counts: groupAttendanceCounts } = useGroupAttendanceCounts();
  const canShowGroupAttendanceCounts = pathname.startsWith("/ogs-groups");
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
    return (
      timetableEnabled ||
      page.href === "/calendar-periods" ||
      page.href === "/payroll"
    );
  });

  // Gruppenzugriff (#1940): temporäre Gruppen-Datenzugriffe sind nur bei
  // festen Gruppen sinnvoll; bei offener Betreuung arbeiten ohnehin alle
  // Berechtigten mit allen Kindern.
  const openCareGroupMode = useOpenCareGroupMode();

  const formatGroupAttendanceCount = (groupId: string | number) => {
    if (!canShowGroupAttendanceCounts) return undefined;
    const count = groupAttendanceCounts[groupId.toString()];
    return count ? `${count.present}/${count.total}` : undefined;
  };

  const databaseSubPages = useMemo(
    () =>
      DATABASE_SUB_PAGES.filter((page) => {
        if (!nfcEnabled && NFC_ONLY_HREFS.has(page.href)) return false;
        // Jahrgangswechsel is gated on grade_transitions:read so a user without
        // it isn't sent to a page that only 403s.
        if (page.href === "/database/grade-transitions") {
          return (
            userIsAdmin || hasPermission(session, "grade_transitions:read")
          );
        }
        return true;
      }),
    [nfcEnabled, userIsAdmin, session],
  );

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
          case "approvals":
            return userIsAdmin;
          case "announcements":
            return canAnnounce && parentNewsEnabled;
          case "bankDetails":
            return hasPermission(session, "guardians:financial");
          case "mealPlan":
            return (
              mealPlanEnabled === true &&
              (userIsAdmin || hasPermission(session, "config:read"))
            );
        }
      }),
    [userIsAdmin, session, canAnnounce, parentNewsEnabled, mealPlanEnabled],
  );

  // Eltern badge: unread messages. Die Elternanfragen zählen seit #2429 am
  // Top-Level-Eintrag "Anfragen", nicht mehr hier.
  const parentSectionBadgeCount = messagesUnreadCount;

  // Filter flat navigation items based on permissions
  const filteredNavItems = NAV_ITEMS.filter((item) => {
    // Anfragen (#2429): zwei Reiter mit getrennten Rechten. Die geteilte Regel
    // deckt users:update, das Paar users:absence+users:read und
    // vacation:approve ab — als requiresPermission nicht ausdrückbar.
    if (item.href === "/anfragen") return canOpenRequestsPage(session);
    if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) return false;
    if (!nfcEnabled && NFC_ONLY_HREFS.has(item.href)) return false;
    if (isBinaryMode && BINARY_HIDDEN_HREFS.has(item.href)) return false;
    // Info-Point Dashboard is opt-in (display.enabled, default off) — hide
    // the admin entry until a school explicitly turns the feature on.
    if (!displayEnabled && item.href === "/info-displays") return false;
    // Tagesauswertung (#1456) hängt am Anwesenheitsprotokoll-Gate
    // (gdpr.attendance_log_enabled, Opt-in, Default aus).
    if (!attendanceLogEnabled && item.href === "/day-log") return false;
    // Team-Chat (#2598) ist Opt-in (operations.staff_messaging_enabled,
    // Default aus): ohne Einschalten taucht der Eintrag gar nicht erst auf.
    if (!staffMessagingEnabled && item.href === "/team-chat") return false;
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

  const renderNavIcon = (item: NavItem) => {
    const concept = item.concept ? MOTO_CONCEPTS[item.concept] : null;
    const isActive = !item.comingSoon && isActiveLink(item.href);
    if (concept) {
      if (isActive) {
        return (
          <MotoDuotoneIcon
            icon={concept.icon}
            tone={concept.tone}
            size={22}
            className="mr-3 h-5 w-5 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5"
          />
        );
      }

      const ConceptIcon = concept.icon;
      return (
        <ConceptIcon
          size={22}
          weight="regular"
          className={getIconClasses(item)}
          aria-hidden="true"
        />
      );
    }

    return (
      <svg
        className={getIconClasses(item)}
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
          href={
            item.href === "/anfragen" || item.href === "/team-chat"
              ? tenantPath(item.href)
              : item.href
          }
          className={getLinkClasses(item.href)}
          {...(item.newTab
            ? { target: "_blank", rel: "noopener noreferrer" }
            : {})}
        >
          {renderNavIcon(item)}
          <span className="flex flex-1 items-center justify-between">
            {item.label}
            {item.href === "/anfragen" && (
              <NotificationBadge
                count={requestsPendingCount}
                tone="staff"
                ariaLabel={`${requestsPendingCount} ${requestsPendingCount === 1 ? "offene Anfrage" : "offene Anfragen"}`}
                className="ml-2"
              />
            )}
            {item.href === "/team-chat" && (
              <NotificationBadge
                count={teamChatUnreadCount}
                tone="staff"
                ariaLabel={`${teamChatUnreadCount} ungelesene Nachrichten im Team-Chat`}
                className="ml-2"
              />
            )}
          </span>
        </Link>
      )}
    </div>
  );

  // Determine which flat items come before / after the accordion insertion points
  // Order: Home, groups, supervisions, search, activities, rooms, staff,
  // substitutions, database, coming soon, bottom pinned.
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

  // Gruppenzugriff (admin only, flat) — nur bei festen Gruppen relevant
  const substitutionsItem = mainNavItems.find(
    (item) => item.href === "/substitutions",
  );

  // Coming soon items
  const comingSoonItems = mainNavItems.filter((item) => item.comingSoon);

  // Get current search params for group/room selection
  const currentGroupParam = searchParams.get("group");
  const currentRoomParam = searchParams.get("room");
  const currentSessionParam = searchParams.get("session");

  // On child pages (e.g. student detail with ?from=/ogs-groups), determine
  // which sub-item should stay highlighted using the last selection from localStorage.
  const isChildOfAccordion =
    pathname.startsWith("/students/") && pathname !== "/students/search";
  const childFromParam = isChildOfAccordion ? fromParam : null;
  const childGroupId = useLocalStorageValue(
    "sidebar-last-group",
    childFromParam?.startsWith("/ogs-groups") ?? false,
  );
  const childRoomId = useLocalStorageValue(
    "sidebar-last-room",
    childFromParam?.startsWith("/active-supervisions") ?? false,
  );
  const childSessionId = useLocalStorageValue(
    "supervision-last-session",
    childFromParam?.startsWith("/active-supervisions") ?? false,
  );

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
      databaseSubPages.some((p) => pathname === p.href)
    ) {
      localStorage.setItem("sidebar-last-database", pathname);
    }
  }, [pathname, databaseSubPages]);

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
      // Prefer the precise session key (#2265); the room key is the legacy
      // fallback for state written before session tracking existed.
      const savedSessionId = localStorage.getItem("supervision-last-session");
      const savedRoomId = localStorage.getItem("sidebar-last-room");
      const targetRoom =
        (savedSessionId
          ? supervisedRooms.find((r) =>
              r.isSchulhof
                ? savedSessionId === "schulhof"
                : r.groupId === savedSessionId,
            )
          : undefined) ??
        (savedRoomId
          ? supervisedRooms.find((r) => r.id === savedRoomId)
          : undefined) ??
        supervisedRooms[0];
      if (targetRoom) {
        const sessionId = targetRoom.isSchulhof
          ? "schulhof"
          : targetRoom.groupId;
        router.push(`/active-supervisions?session=${sessionId}`);
      } else {
        router.push("/active-supervisions");
      }
    }
  }, [toggle, pathname, supervisedRooms, router]);

  const handleDatabaseToggle = useCallback(() => {
    if (!pathname.startsWith("/database")) {
      // Not on any database page, expand accordion and navigate to hub
      toggle("database");
      router.push("/database");
    } else if (pathname === "/database") {
      // On hub page, just toggle collapse or expand
      toggle("database");
    } else {
      // On a sub-page like /database/rooms, navigate back to hub
      router.push("/database");
    }
  }, [toggle, pathname, router]);

  const activeEnrollmentSubPageHref = getActiveEnrollmentSubPageHref(pathname);
  const isOnEnrollmentsPage = activeEnrollmentSubPageHref !== null;

  const handleEnrollmentsToggle = useCallback(() => {
    // Hub = the request review queue. Mirrors handleDatabaseToggle's
    // navigate-on-expand behavior so clicking the section label always
    // ends up on a useful page rather than just toggling chevrons.
    const onSection = getActiveEnrollmentSubPageHref(pathname) !== null;
    if (!onSection) {
      toggle("enrollments");
      router.push("/admin/enrollments");
    } else if (pathname === "/admin/enrollments") {
      toggle("enrollments");
    } else {
      router.push("/admin/enrollments");
    }
  }, [toggle, pathname, router]);

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

  // Caregivers see their own supervision. A successful overview request also
  // covers effective admins and verified staff under all_staff (#2380).
  // overviewEnabled avoids the synthetic Schulhof entry triggering the
  // accordion when the school keeps everyone on their own supervisions.
  const showStaffAccordions = userIsCaregiver || overviewEnabled;

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
          {/* Home (admin only) */}
          {beforeAccordionItems
            .filter((item) => item.href === "/dashboard")
            .map(renderNavItem)}

          {/* Meine Gruppen accordion (staff only; hidden in open-care group
              mode because there is no concept of "meine Gruppe" — staff work
              with all children instead, #1544) */}
          {showStaffAccordions && !openCareGroupMode && (
            <SidebarAccordionSection
              icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
              concept="groups"
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
              activeColor="text-moto-green"
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
              concept="supervision"
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
                      ? `/active-supervisions?session=schulhof`
                      : `/active-supervisions?session=${room.groupId}`
                  }
                  label={room.name}
                  isActive={isRoomSubItemActive(
                    childSessionId,
                    childRoomId,
                    room.isSchulhof ? "schulhof" : room.groupId,
                    room.isSchulhof ? "schulhof" : room.id,
                    pathname,
                    currentSessionParam,
                    currentRoomParam,
                    index,
                  )}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Alle Kinder (flat) */}
          {beforeAccordionItems
            .filter((item) => item.href === "/students/search")
            .map(renderNavItem)}

          {/* Flat middle items: Aktivitaten, Raume, Mitarbeiter */}
          {middleItems.map(renderNavItem)}

          {/* Gruppenzugriff (admin, flat) — nur bei festen Gruppen (#1940) */}
          {substitutionsItem &&
            !openCareGroupMode &&
            renderNavItem(substitutionsItem)}

          {/* Eltern accordion — bundles the parent-communication surfaces
              (Nachrichten, Konto-Anfragen, Mitteilungen, Essensplan) behind an
              overview hub. Shown to all staff; sub-items are gated per item.
              Die Elternanfragen leben seit #2429 im Top-Level-Modul
              "Anfragen". */}
          <SidebarAccordionSection
            icon={navigationIcons.parents}
            concept="parents"
            label={PARENT_SECTION.label}
            activeColor="text-moto-blue"
            isExpanded={expanded === "eltern"}
            onToggle={handleParentToggle}
            isActive={isOnParentPage}
            isIconActive={isOnParentPage}
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
                // entirely ("/messages" → wrong slug / missing route), so
                // prefix all of them via tenantPath — matching the accordion
                // header's tenant-aware router.push and the /eltern page's
                // card links. No-op in subdomain mode.
                href={tenantPath(page.href)}
                label={page.label}
                isActive={activeParentSubPageHref === page.href}
                badgeCount={
                  page.feature === "messages" ? messagesUnreadCount : 0
                }
              />
            ))}
          </SidebarAccordionSection>

          {/* Datenverwaltung accordion (admin only) */}
          {userIsAdmin && (
            <SidebarAccordionSection
              icon="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4"
              concept="database"
              label={DATABASE_SECTION.label}
              activeColor="text-gray-500"
              isExpanded={expanded === "database"}
              onToggle={handleDatabaseToggle}
              isActive={isAccordionSectionActive(
                "/database",
                databaseSubPages.some((p) => pathname === p.href),
              )}
              isIconActive={pathname.startsWith("/database")}
              hasChildren={databaseSubPages.length > 0}
            >
              {databaseSubPages.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  // Tenant-scoped [tenant]/… routes: in path-routing mode a
                  // bare "/database/exports" makes the router read "database"
                  // as the tenant slug, so prefix via tenantPath like the
                  // Eltern accordion. No-op in subdomain mode.
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={pathname === page.href}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Planung accordion (#1946) — bündelt Betreuungsplan, Dienstplan,
              Vertretung und Kalenderzeiträume für Admins. Bei explizit
              ausgeschaltetem timetable.enabled bleiben für Admins
              Kalenderzeiträume und Abrechnung übrig.

              Nicht-Admins erreichen die Betreuungsplan-Leseansicht (#2283)
              als Tab in "Mein Kalender"; das Akkordeon zeigt ihnen nur
              Seiten mit gehaltener nonAdminPermission (heute: Abrechnung
              über config:manage). */}
          {planningSubPages.length > 0 && (
            <SidebarAccordionSection
              icon={navigationIcons.betreuungsplan}
              concept="carePlan"
              label={PLANNING_SECTION.label}
              activeColor="text-moto-blue"
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
                  // via tenantPath prefixen (No-op im Subdomain-Modus),
                  // wie beim Eltern-/Datenverwaltung-Akkordeon.
                  href={tenantPath(page.href)}
                  label={page.label}
                  isActive={activePlanningSubPageHref === page.href}
                />
              ))}
            </SidebarAccordionSection>
          )}

          {/* Anmeldungen accordion (admin only). Bundles the setup hub,
              enrollment periods, offers and enrollment forms for admins. */}
          {userIsAdmin && (
            <SidebarAccordionSection
              icon="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              concept="enrollments"
              label={ENROLLMENT_SECTION.label}
              activeColor="text-moto-green"
              isExpanded={expanded === "enrollments"}
              onToggle={handleEnrollmentsToggle}
              isActive={isOnEnrollmentsPage}
              isIconActive={isOnEnrollmentsPage}
              hasChildren={ENROLLMENT_SUB_PAGES.length > 0}
            >
              {ENROLLMENT_SUB_PAGES.map((page) => (
                <SidebarSubItem
                  key={page.href}
                  href={page.href}
                  label={page.label}
                  isActive={activeEnrollmentSubPageHref === page.href}
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
