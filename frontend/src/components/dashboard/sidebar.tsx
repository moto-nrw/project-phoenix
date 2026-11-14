// components/dashboard/sidebar.tsx
"use client";

import { Fragment, Suspense, useCallback, useEffect, useMemo } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import { COUNTER_TONE } from "~/lib/counter-tones";
import { useTenantRouter } from "~/lib/tenant-router";
import { normalizeTenantPathname, useTenantAwarePath } from "~/lib/tenant-path";
import {
  useChildSearchOnlyNavigation,
  useStaffMessagingEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
  useMealPlanEnabled,
  useTimetableEnabled,
} from "~/lib/tenant-context";
import { useSession } from "next-auth/react";
import { useTranslations } from "next-intl";
import { useOptionalSupervision } from "~/lib/supervision-context";
import {
  isSharedOpenRoom,
  type SupervisedRoom,
} from "~/lib/supervision-derive";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTraegerReturnHref } from "~/lib/hooks/use-traeger-return-href";
import { TRAEGER_RETURN_LABEL } from "~/lib/traeger-handoff";
import { isDemoBannerShown } from "~/components/demo/demo-banner";
import { useDemoEveningActive } from "~/components/demo/demo-evening-notice";
import {
  canReadSupervisionData,
  hasEffectiveAdminScope,
  hasPermission,
  hasRole,
  isCaregiver,
  leadsSchool,
} from "~/lib/auth-utils";
import { canOpenActiveSupervision } from "~/lib/active-supervision-access";
import { canReviewGuardianApprovals } from "~/lib/guardian-approval-access";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { useChangeRequestAccess } from "~/lib/hooks/use-change-request-access";
import { operatorPath } from "~/lib/operator-url";
import { useSidebarAccordion } from "~/lib/hooks/use-sidebar-accordion";
import { useSidebarGroups } from "~/lib/hooks/use-sidebar-groups";
import { useSimpleStaffNav } from "~/lib/hooks/use-simple-staff-nav";
import { useSidebarCollapsed } from "~/lib/hooks/use-sidebar-collapsed";
import {
  isStaffSettingsPath,
  StaffSettingsSidebarNav,
} from "./staff-settings-nav";
import { useSidebarCollapseTransition } from "~/lib/hooks/use-sidebar-collapse-transition";
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useSidebarMemory } from "~/lib/sidebar-memory";
import { useArrivalOwner } from "~/lib/hooks/use-arrival-owner";
import {
  isOwnGroup,
  OGS_GROUPS_LABEL,
  OGS_GROUPS_OVERVIEW_PATH,
  ogsGroupHref,
} from "~/lib/ogs-groups-navigation";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useMessagesUnread } from "~/lib/hooks/use-messages-unread";
import { useStaffMessagesUnread } from "~/lib/hooks/use-staff-messages-unread";
import { useStaffNoticesPending } from "~/lib/hooks/use-staff-notices-pending";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useEnrollmentRequestsPending } from "~/lib/hooks/use-enrollment-requests-pending";
import { useEnrollmentsUnread } from "~/lib/hooks/use-enrollments-unread";
import { useModuleNavigation } from "~/lib/hooks/use-module-navigation";
import { useSchoolParentNewsEnabled } from "~/lib/hooks/use-school-switches";
import type { ModuleVisibility } from "~/lib/module-register";
import { ModuleLockMark } from "~/components/ui/module-lock-mark";
import { useHelpHref } from "~/lib/hooks/use-help-href";
import { useGroupAttendanceCounts } from "~/lib/group-attendance-count-context";
import { SidebarAccordionSection } from "~/components/ui/sidebar-accordion-section";
import { SidebarGroup } from "~/components/ui/sidebar-group";
import { ListBulletsIcon } from "@phosphor-icons/react/ssr";
import {
  combineSidebarRowBadges,
  SidebarRowButton,
  SidebarRowLink,
  SidebarRowPlaceholder,
  SidebarSectionHeading,
  type SidebarRowBadge,
} from "~/components/ui/sidebar-row";
import { SidebarSubItem } from "~/components/ui/sidebar-sub-item";
import { NavSceneIcon } from "~/components/ui/nav-scene-icon/nav-scene-icon";
import {
  SIDEBAR_ICON_CLASSES,
  SIDEBAR_NAV_GAP,
  SIDEBAR_NAV_PADDING,
  portalSidebarTopOffset,
} from "~/components/ui/sidebar-geometry";
import { PortalSidebarFrame } from "~/components/ui/portal-sidebar";
import type { MotoConcept } from "~/lib/moto-concepts";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { Skeleton } from "~/components/ui/skeleton";
import {
  getActivePlanningSubPageHref,
  isPlanningPageHref,
  PLANNING_SUB_PAGES,
} from "~/lib/planning-navigation";
import {
  COMMUNICATION_SUB_PAGES,
  DATABASE_SECTION,
  DATABASE_SUB_PAGES,
  ENROLLMENT_SECTION,
  ENROLLMENT_SUB_PAGES,
  getActiveEnrollmentSubPageHref,
  hasAnyDatabasePagePermission,
  PARENT_SUB_PAGES,
  PLANNING_CATALOG_HREFS,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";
import {
  STAFF_NAV_ITEMS,
  staffNavBorrowedActiveHref,
  staffNavItemPermitted,
  staffNavPathActive,
  staffNavSettingOn,
  useStaffNavSettings,
  type StaffNavItem,
} from "./staff-nav-items";
import {
  getStaffNavDefaultOpenGroups,
  STAFF_NAV_BOTTOM,
  STAFF_NAV_CONCEPTS,
  STAFF_NAV_GROUPS,
  STAFF_NAV_SIMPLE_ENTRIES,
  STAFF_NAV_TOP,
  type StaffNavEntry,
  type StaffNavSectionKey,
} from "~/lib/staff-navigation";
import { databaseConcept } from "~/lib/concepts/database";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { groupsConcept } from "~/lib/concepts/groups";
import { supervisionConcept } from "~/lib/concepts/supervision";
import { accountsConcept } from "~/lib/concepts/accounts";
import { mealPlanConcept } from "~/lib/concepts/mealPlan";
import { announcementsConcept } from "~/lib/concepts/announcements";
import { devicesConcept } from "~/lib/concepts/devices";
import { operatorsConcept } from "~/lib/concepts/operators";
import { organizationsConcept } from "~/lib/concepts/organizations";
import { peopleConcept } from "~/lib/concepts/people";
import { reportsConcept } from "~/lib/concepts/reports";
import { rfidConcept } from "~/lib/concepts/rfid";
import { schoolsConcept } from "~/lib/concepts/schools";

// Eine Zeile der Seitenleiste. Die Seiten der Mitarbeiter-Navigation kommen
// aus der geteilten Liste (staff-nav-items.ts), Operator-Seiten von hier.
interface NavItem {
  readonly href: string;
  readonly label: string;
  readonly concept: MotoConcept;
  readonly comingSoon?: boolean;
  readonly newTab?: boolean;
  // Modul nicht gebucht (#4894): nur Leitungen sehen den Eintrag, mit
  // Schloss. Er führt auf die Seite, die das erklärt.
  readonly locked?: boolean;
}

/**
 * Regel des Modul-Registers (#4894) für einen Eintrag: `hidden` fällt weg,
 * `locked` ist für die Leitung sichtbar, unabhängig vom Recht der Zielseite:
 * die Erklärung zeigt, dass die Schule das Modul zuerst buchen muss. `open`
 * braucht Recht und Einstellung.
 */
function moduleGate(
  visibility: ModuleVisibility,
  permitted: boolean,
  settingOn: boolean,
): { readonly show: boolean; readonly locked: boolean } {
  if (visibility === "hidden") {
    return { show: false, locked: false };
  }
  if (visibility === "locked") return { show: true, locked: true };
  if (!permitted) return { show: false, locked: false };
  return { show: settingOn, locked: false };
}

interface OperatorNavSection {
  readonly label: string;
  readonly items: readonly NavItem[];
}

// Operator navigation, grouped into static labeled sections.
const OPERATOR_NAV_SECTIONS: readonly OperatorNavSection[] = [
  {
    label: "VERWALTUNG",
    items: [
      {
        href: "/operator/organizations",
        label: "Träger",
        concept: organizationsConcept,
      },
      {
        href: "/operator/schools",
        label: "Schulen",
        concept: schoolsConcept,
      },
      {
        href: "/operator/accounts",
        label: "Konten",
        concept: accountsConcept,
      },
      {
        href: "/operator/caterers",
        label: "Caterer",
        concept: mealPlanConcept,
      },
      {
        href: "/operator/devices",
        label: "Geräte",
        concept: devicesConcept,
      },
      {
        href: "/operator/persons",
        label: "Personen",
        concept: peopleConcept,
      },
      {
        href: "/operator/unregistered-tags",
        label: "Unbekannte RFID",
        concept: rfidConcept,
      },
    ],
  },
  {
    label: "AUSWERTUNG",
    items: [
      {
        href: "/operator/billing",
        label: "Stichtagszahlen",
        concept: reportsConcept,
      },
    ],
  },
  {
    label: "KOMMUNIKATION",
    items: [
      {
        href: "/operator/announcements",
        label: "Ankündigungen",
        concept: announcementsConcept,
      },
    ],
  },
  {
    label: "TEAM",
    items: [
      {
        href: "/operator/operators",
        label: "Operatoren",
        concept: operatorsConcept,
      },
    ],
  },
];

// Tenant-scoped [tenant]/… routes. In path-routing mode a bare "/messages"
// leaves the current tenant path entirely and a bare "/eltern/…" is read as
// the tenant slug, so these hrefs go through tenantPath — matching what the
// former Eltern/Kommunikation/Planung accordions did for their sub-items.
// Everything else stays bare: /help is host-agnostic and must not carry the
// slug. No-op in subdomain mode.
const TENANT_SCOPED_HREFS = new Set<string>([
  "/anfragen",
  STAFF_FLAT_PAGES.payroll.href,
  ...PARENT_SUB_PAGES.map((page) => page.href),
  ...COMMUNICATION_SUB_PAGES.map((page) => page.href),
  ...PLANNING_SUB_PAGES.map((page) => page.href),
]);

// Rows that carry a counter, with the wording a screen reader gets.
// Vorlesetext des Anmeldungen-Zählers (#3778).
const ENROLLMENTS_UNREAD_NOUN = "ungelesene Anmeldungen";

// `one` ist die Einzahl („1 offene Anfrage“), `noun` die Mehrzahl.
interface CounterNoun {
  readonly one: string;
  readonly noun: string;
}

const ENROLLMENTS_COUNTER: CounterNoun = {
  one: "ungelesene Anmeldung",
  noun: ENROLLMENTS_UNREAD_NOUN,
};

const ROW_BADGE_LABELS: Readonly<
  Record<
    string,
    CounterNoun & {
      readonly tone: (typeof COUNTER_TONE)["staff" | "parents"];
    }
  >
> = {
  "/anfragen": {
    tone: COUNTER_TONE.staff,
    one: "offene Anfrage",
    noun: "offene Anfragen",
  },
  "/messages": {
    tone: COUNTER_TONE.parents,
    one: "ungelesene Nachricht",
    noun: "ungelesene Nachrichten",
  },
  // Wie die Zeile benannt, sonst klingt er wie der Eltern-Chat (#6566).
  "/team-chat": {
    tone: COUNTER_TONE.staff,
    one: "ungelesene Nachricht im Team-Chat",
    noun: "ungelesene Nachrichten im Team-Chat",
  },
  "/tagesinformationen": {
    tone: COUNTER_TONE.staff,
    one: "offener Hinweis der Leitung",
    noun: "offene Hinweise der Leitung",
  },
};

/** Das gezählte Wort in Einzahl oder Mehrzahl („offene Anfrage“ / „… Anfragen“). */
function counterNoun(count: number, noun: CounterNoun): string {
  return count === 1 ? noun.one : noun.noun;
}

/** Der Vorlesetext eines Zählers, „1 offene Anfrage“ / „3 offene Anfragen“. */
function counterAriaLabel(count: number, noun: CounterNoun): string {
  return `${count} ${counterNoun(count, noun)}`;
}

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
  // Auf der Auswahlseite ist keine einzelne Gruppe geöffnet (#4351).
  if (pathname.startsWith(OGS_GROUPS_OVERVIEW_PATH)) return false;
  if (currentGroupParam) return currentGroupParam === groupId;
  return index === 0;
}

/**
 * Determine if a supervision sub-item should be highlighted as active.
 * Sessions are keyed by active-group ID (`?session=`, #2265); the legacy
 * room key still resolves for old links and stored state.
 */
// A released room ("offener Raum", #3065) has no session of its own: several
// sessions can run in it and none of them is the entry. It is addressed by its
// room id alone, so `sessionId` is null for it and the session comparisons are
// skipped — a session id that happens to equal a room id must not light up the
// wrong entry.
function isRoomSubItemActive(
  childSessionId: string | null,
  childRoomId: string | null,
  sessionId: string | null,
  sessionIds: readonly string[],
  roomId: string,
  pathname: string,
  currentSessionParam: string | null,
  currentRoomParam: string | null,
  index: number,
): boolean {
  if (currentSessionParam) {
    if (sessionId) return currentSessionParam === sessionId;
    if (sessionIds.includes(currentSessionParam)) return true;
  }
  if (childSessionId) return sessionId !== null && childSessionId === sessionId;
  if (childRoomId) return childRoomId === roomId;
  if (!pathname.startsWith("/active-supervisions")) return false;
  if (currentRoomParam) return currentRoomParam === roomId;
  return index === 0;
}

/**
 * The link that opens one navigation entry. Released rooms travel by room id
 * (`?room=`), own supervisions by session id (`?session=`) — the same two keys
 * the target page resolves, so sidebar, mobile navigation and page agree.
 */
function supervisionHref(room: SupervisedRoom): string {
  return room.isOpenRoom
    ? `/active-supervisions?room=${room.id}`
    : `/active-supervisions?session=${room.groupId}`;
}

interface SidebarProps {
  readonly className?: string;
}

// Die Tenant-Shell setzt das mit `min-h-0!` außer Kraft (app-shell.tsx).
const SIDEBAR_MIN_HEIGHT = "min-h-screen";

interface SidebarContentProps extends SidebarProps {
  // Einklappbare Seitenleiste (#2825): der Zustand lebt im äußeren Sidebar-
  // Wrapper (useSidebarCollapsed), damit auch der Suspense-Fallback die
  // richtige Breite rendert. Umgeschaltet wird über den Toggle-Button in
  // der Kopfzeile (header.tsx), der denselben Store nutzt.
  readonly collapsed: boolean;
  readonly onExpandSidebar: () => void;
}

function SidebarContent({
  className = "",
  collapsed,
  onExpandSidebar,
}: SidebarContentProps) {
  const tParentNav = useTranslations("parentNav");
  // Bezeichnungen, Chevrons und Zähler folgen der Breitenänderung, statt am
  // Anfang oder Ende aufzublitzen (#2923).
  const { labelsMounted, labelsVisible } =
    useSidebarCollapseTransition(collapsed);
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  // Prefixes tenant-scoped hrefs with the slug in path-routing mode (no-op in
  // subdomain/operator mode). Used for tenant-scoped navigation links.
  const tenantPath = useTenantAwarePath();
  const { data: session } = useSession();
  const { mode, isPreview } = useShellAuth();
  const traegerReturnHref = useTraegerReturnHref();
  const demoBannerShown = isDemoBannerShown({ mode, isPreview });
  const eveningRow = useDemoEveningActive() && demoBannerShown;
  const changeRequestAccess = useChangeRequestAccess();
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
  // Tagesinformationen (#2180): heutige Hinweise, deren Kenntnisnahme noch
  // aussteht. Ohne Startseite für alle ist das Badge der einzige Weg, wie ein
  // Hinweis der Leitung einem begegnet, statt gesucht werden zu müssen.
  const { unreadCount: staffNoticesPendingCount } = useStaffNoticesPending();
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
  // Ungelesene Anmeldungen (#3778): eigener Zähler am Bereich „Anmeldungen",
  // keine Anfrage und deshalb nicht im Anfragen-Zähler.
  const { unreadCount: enrollmentsUnreadCount } = useEnrollmentsUnread();
  const requestsPendingCount =
    changeRequestsPendingCount +
    staffAbsencesPendingCount +
    enrollmentRequestsPendingCount +
    careWithdrawalsPendingCount;

  // Accordion state passes `from` param so child pages (e.g. student detail)
  // keep the originating accordion section open
  const fromParam = searchParams.get("from");
  const { expanded, toggle } = useSidebarAccordion(
    pathname,
    fromParam,
    "groups",
  );
  const userIsAdmin = hasRole(session, "admin");
  const userHasEffectiveAdminScope = hasEffectiveAdminScope(session);
  const userIsCaregiver = isCaregiver(session);
  const canReadSupervisions = canReadSupervisionData(session);
  // Leitung der Einrichtung: Adminzuschnitt oder config:manage, das Recht
  // hinter Anmeldungen und Einstellungen (#3469). Entscheidet über die
  // Anmeldungen, die Rolle der Hilfe und die beim ersten Besuch offenen
  // Gruppen, nicht über einzelne Rechte.
  const userLeadsSchool = leadsSchool(session);
  // Die fünf Gruppen der Seitenleiste (#2826): beim ersten Besuch stehen der
  // Tagesbetrieb und die Gruppe der eigenen Arbeit offen (#6670), der Rest
  // bleibt zu, bis die Person etwas aufklappt oder eine Seite darin öffnet.
  const { isGroupOpen, toggleGroup } = useSidebarGroups(
    pathname,
    fromParam,
    getStaffNavDefaultOpenGroups(userLeadsSchool),
  );
  // Elternmitteilungen (#1669) authoring is ADMIN-ONLY in v1: every
  // /api/parent-announcements route is guarded by the admin:* wildcard
  // (backend api/announcement/api.go), because the service does no per-caller
  // audience scoping. Mirror that exact permission so a non-admin never sees a
  // nav entry whose every list/create/publish call would 403. A future
  // delegated-announcer role must relax the backend route first.
  const canAnnounce = hasPermission(session, "admin:*");
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";
  // Gebuchte Module (#4894): offen, mit Schloss oder gar nicht.
  const moduleNav = useModuleNavigation();
  // Schulschalter der Seiten aus der geteilten Liste.
  const navSettings = useStaffNavSettings();
  // Betreuungsplan-Flag für den Tagesplan-Eintrag (#2383): vom Tenant-Resolve,
  // damit es auch für Betreuungskräfte ohne config:read aufgelöst ist.
  const tagesplanEnabled = useTimetableEnabled();
  const staffMessagingEnabled = useStaffMessagingEnabled();
  const { counts: groupAttendanceCounts } = useGroupAttendanceCounts();
  const canShowGroupAttendanceCounts = pathname.startsWith("/ogs-groups");
  // Aus dem Navigationskontext (#4996): die Navigation lädt für ihre Schulschalter
  // nicht das ganze Settings-Schema.
  const parentNewsEnabled = useSchoolParentNewsEnabled();

  // Vom Tenant-Resolve (#3873): eine Küchenrolle hält meals:*, aber kein
  // config:read und liest das Settings-Schema nicht.
  const mealPlanEnabled = useMealPlanEnabled();

  // Planung-Akkordeon (#1946): dasselbe timetable.enabled wie der
  // Tagesplan-Eintrag und das Route-Gate in betreuungsplan-view.tsx.
  const timetableEnabled = tagesplanEnabled;

  // Nur die Seiten des Betreuungsplans hängen an timetable.enabled.
  // Kalenderzeiträume bleiben erreichbar (Anmeldephasen verknüpfen sich mit
  // ihnen); Dienstplan und Abrechnung folgen ihren Modulen (#4894).
  // Gruppenübergaben (#1940) sind nur bei
  // festen Gruppen sinnvoll; bei offener Betreuung arbeiten ohnehin alle
  // Berechtigten mit allen Kindern.
  const openCareGroupMode = useOpenCareGroupMode();
  const planningSubPages = PLANNING_SUB_PAGES.flatMap((page) => {
    // Nicht-Admins sehen nur Seiten mit gehaltener nonAdminPermission
    // (#2283), bei offener Betreuung auch mit openCarePermission (#4769).
    const gate = moduleGate(
      moduleNav.page(page.href),
      userIsAdmin ||
        (page.nonAdminPermission !== undefined &&
          hasPermission(session, page.nonAdminPermission)) ||
        (openCareGroupMode &&
          page.openCarePermission !== undefined &&
          hasPermission(session, page.openCarePermission)),
      !page.requiresTimetable || timetableEnabled,
    );
    return gate.show ? [{ ...page, locked: gate.locked }] : [];
  });

  // Kinder nur über „Alle Kinder“ (#1321): Gruppen und Aktuelle
  // Aufsicht verlassen die Leiste und stehen dort als Reiter.
  const childSearchOnlyNavigation = useChildSearchOnlyNavigation();
  // Einfache Seitenleiste (#6956): nur die Seiten des Betreuungsalltags,
  // ohne Gruppen. Die Person kann selbst alle Seiten einblenden.
  const simpleNav = useSimpleStaffNav(session ?? null);

  // Die Adresse hinter dem Eintrag `Hilfe` ganz unten, mit dem Kontext der
  // Sitzung. Die mobile Navigation holt sie aus demselben Hook (#3575).
  const helpHref = useHelpHref();

  const formatGroupAttendanceCount = (groupId: string) => {
    if (!canShowGroupAttendanceCounts) return undefined;
    const count = groupAttendanceCounts[groupId.toString()];
    return count ? `${count.present}/${count.total}` : undefined;
  };

  const databaseSubPages = useMemo(
    () =>
      DATABASE_SUB_PAGES.flatMap((page) => {
        const gate = moduleGate(
          moduleNav.page(page.href),
          // Jede Seite trägt das Recht ihrer Route
          // (`DATABASE_PAGE_PERMISSIONS`, derselbe Katalog wie im Guard der
          // Route); ohne es führt der Eintrag nur auf ein 403. Der
          // Adminzuschnitt öffnet alle.
          userHasEffectiveAdminScope ||
            hasAnyDatabasePagePermission(session, page.href),
          // Planungsspuren gehören zum Betreuungsplan; ist er ausgeschaltet,
          // gibt es nichts zu ordnen.
          !PLANNING_CATALOG_HREFS.has(page.href) || timetableEnabled,
        );
        return gate.show ? [{ ...page, locked: gate.locked }] : [];
      }),
    [moduleNav, userHasEffectiveAdminScope, session, timetableEnabled],
  );

  // Visible "Eltern" accordion sub-pages. Same per-item gating the flat
  // NAV_ITEMS carried before consolidation: overview + Nachrichten for all
  // staff, the rest per permission / feature flag.
  const parentSubPages = useMemo(
    () =>
      PARENT_SUB_PAGES.flatMap((page) => {
        const rule = ((): readonly [permitted: boolean, settingOn: boolean] => {
          switch (page.feature) {
            case "messages":
              // Elternnachrichten verlangen users:read (#4573: nicht für
              // die AG-Leitung).
              return [
                userIsAdmin || hasPermission(session, "users:read"),
                true,
              ];
            case "approvals":
              // Lesen users:manage, Entscheiden users:update (#3469).
              return [canReviewGuardianApprovals(session), true];
            case "announcements":
              return [canAnnounce, parentNewsEnabled];
            case "bankDetails":
              return [hasPermission(session, "guardians:financial"), true];
            case "mealPlan":
              return [
                userIsAdmin ||
                  hasPermission(session, "meals:read") ||
                  hasPermission(session, "meals:register"),
                mealPlanEnabled,
              ];
          }
        })();
        const gate = moduleGate(moduleNav.page(page.href), ...rule);
        return gate.show ? [{ ...page, locked: gate.locked }] : [];
      }),
    [
      moduleNav,
      userIsAdmin,
      session,
      canAnnounce,
      parentNewsEnabled,
      mealPlanEnabled,
    ],
  );

  // Team-interne Seiten: Team-Chat ist Opt-out (operations.
  // staff_messaging_enabled, Default an seit #3254) und fällt trotzdem
  // fail-closed weg, wenn das Flag fehlt; die
  // Tagesinformationen liest jede Mitarbeiterin.
  const communicationSubPages = useMemo(
    () =>
      COMMUNICATION_SUB_PAGES.filter((page) => {
        switch (page.feature) {
          case "teamChat":
            return staffMessagingEnabled;
          case "staffNotices":
            return hasPermission(session, "users:read");
          case "surveys":
            // Jede Person im Team beantwortet hier; die Verwaltung trennt
            // die Seite selbst nach surveys:manage (#4781).
            return true;
        }
      }),
    [staffMessagingEnabled, session],
  );

  // Zähler an einzelnen Zeilen. Eine zugeklappte Gruppe summiert die Zähler
  // ihrer Zeilen auf ihre Kopfzeile; die Elternanfragen zählen seit #2429 am
  // Eintrag "Anfragen", nicht bei Nachrichten.
  const rowBadgeCounts: Readonly<Record<string, number>> = {
    "/anfragen": requestsPendingCount,
    "/messages": messagesUnreadCount,
    "/team-chat": teamChatUnreadCount,
    "/tagesinformationen": staffNoticesPendingCount,
  };

  // Die Seiten aus der geteilten Liste (staff-nav-items.ts). Das Modul der
  // Seite entscheidet zuerst (#4894): nicht gebucht heißt weg oder Schloss.
  const navViewer = {
    session,
    isAdmin: userIsAdmin,
    isCaregiver: userIsCaregiver,
    openCareGroupMode,
  };
  const filteredNavItems: NavItem[] = STAFF_NAV_ITEMS.flatMap((item) => {
    // Der Binärmodus schließt diese Seiten vollständig aus. Auch eine Leitung
    // darf hier kein Schloss sehen, weil die Zielseite nicht erklärt, sondern
    // als nicht passende Anwesenheitsart gesperrt ist.
    if (isBinaryMode && item.binaryHidden) return [];
    const gate = moduleGate(
      moduleNav.page(item.href),
      isNavItemPermitted(item),
      true,
    );
    return gate.show
      ? [
          {
            href: item.href,
            label: item.label,
            concept: item.concept,
            newTab: item.newTab,
            locked: gate.locked,
          },
        ]
      : [];
  });

  function isNavItemPermitted(item: StaffNavItem): boolean {
    // Anfragen (#2429/#2911): JWT-Rechte allein reichen nicht. Für
    // Elternanfragen muss zusätzlich der aktuelle serverseitige Prüfbereich
    // (Admin, Gruppenleitung oder Vertretung) gelten.
    if (item.href === STAFF_FLAT_PAGES.anfragen.href) {
      return changeRequestAccess.canOpenRequestsPage;
    }
    return (
      staffNavSettingOn(item, navSettings) &&
      staffNavItemPermitted(item, navViewer)
    );
  }

  // Helper to determine active href for student detail pages based on referrer
  const getStudentDetailActiveHref = (from: string | null): string => {
    if (!from) return "/students/search";
    // Gruppen- und Aufsichtsseiten bleiben als Arbeitsseiten erreichbar,
    // teilen sich bei eingeschalteter Navigation aber den sichtbaren
    // Einstieg „Alle Kinder“ – auch nach dem Öffnen einer Kindakte.
    if (
      childSearchOnlyNavigation &&
      (from.startsWith("/ogs-groups") ||
        from.startsWith("/active-supervisions"))
    ) {
      return "/students/search";
    }
    if (from.startsWith("/ogs-groups")) return "/ogs-groups";
    if (from.startsWith("/active-supervisions")) return "/active-supervisions";
    if (from.startsWith("/day-log")) return "/day-log";
    // Drill-in from a room ("Kinder im Raum"): the room page /rooms/{id}
    // (#3115) and old /rooms?room={id} links count, so the sidebar reflects
    // the actual entry path in either flow.
    if (from.startsWith("/rooms/") || from.startsWith("/rooms?"))
      return "/rooms";
    // Die Kinderdaten der Datenverwaltung verlinken auf die Kindakte (#3115).
    if (from.startsWith("/database/students")) return "/database/students";
    if (from.startsWith("/students/search")) return "/students/search";
    return "/students/search";
  };

  // Ein offener Raum ohne eigene Aufsicht, geöffnet über seine Raumseite,
  // gehört in der Navigation zu „Räume“, nicht zu „Aktuelle Aufsicht“ (#4372).
  const isViewingSharedOpenRoom =
    pathname.startsWith("/active-supervisions") &&
    isSharedOpenRoom(supervisedRooms, {
      roomId: searchParams.get("room"),
      sessionId: searchParams.get("session"),
    });

  // Seiten, die ein anderer Eintrag markiert (geteilte Regel mit der mobilen
  // Leiste): offener Raum → Räume, „Unterwegs“ aus „Alle Kinder“ → Alle
  // Kinder.
  const borrowedActiveHref = staffNavBorrowedActiveHref(
    pathname,
    searchParams.get("from"),
    isViewingSharedOpenRoom,
  );

  // Die Objektrouten (Kindakte, Personalakte, Raumseite) haben keinen eigenen
  // Eintrag in der Seitenleiste: markiert wird die Sammlung, aus der man kam
  // (`?from=`). Die Register der Datenverwaltung verlinken seit #3115 auf
  // dieselben Routen wie die Übersichten; ohne den Rückweg leuchtete dann
  // immer die Übersicht, auch wer aus der Datenverwaltung kam.
  const objectRouteActiveHref = (() => {
    if (borrowedActiveHref !== null) return borrowedActiveHref;
    const from = searchParams.get("from");
    if (pathname.startsWith("/students/") && pathname !== "/students/search") {
      return getStudentDetailActiveHref(from);
    }
    if (pathname.startsWith("/staff/")) {
      return from?.startsWith("/database/personal")
        ? "/database/personal"
        : "/staff";
    }
    if (pathname.startsWith("/rooms/")) {
      return from?.startsWith("/database/rooms") ? "/database/rooms" : "/rooms";
    }
    return null;
  })();

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
    if (objectRouteActiveHref !== null) {
      return objectRouteActiveHref === href;
    }
    const operatorDrillInHref = getOperatorDrillInActiveHref();
    if (operatorDrillInHref) {
      return href === operatorDrillInHref;
    }
    if (href === "/home") return pathname === "/home";
    // Ohne eigene Einträge gehören Gruppen- und Aufsichtsseite zu „Alle
    // Kinder“, von wo aus sie geöffnet werden (#1321).
    if (
      href === STAFF_FLAT_PAGES.studentSearch.href &&
      childSearchOnlyNavigation &&
      (pathname.startsWith("/ogs-groups") ||
        pathname.startsWith("/active-supervisions"))
    ) {
      return true;
    }
    if (href === "/parents") return pathname === "/parents" || pathname === "/";
    // Jede kanonische Planungsseite markiert ihren eigenen Eintrag.
    if (isPlanningPageHref(href)) {
      return getActivePlanningSubPageHref(pathname) === href;
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
    return staffNavPathActive(href, pathname);
  };

  // Check if a section's parent header should be highlighted
  // Parent highlights only when on the section page WITHOUT a sub-item selected
  const isAccordionSectionActive = (
    parentHref: string,
    hasSubItemSelected: boolean,
  ) => {
    if (objectRouteActiveHref !== null) {
      // Aus einem Register der Datenverwaltung heraus zählt dessen Eintrag,
      // nicht der Bereichskopf.
      if (objectRouteActiveHref.startsWith(`${parentHref}/`)) return false;
      if (objectRouteActiveHref !== parentHref) return false;
      // If a sub-item is highlighted on the child page, don't highlight the parent
      return !hasSubItemSelected;
    }
    if (!pathname.startsWith(parentHref)) return false;
    // If a sub-item is selected, don't highlight the parent
    return !hasSubItemSelected;
  };

  const renderNavIcon = (item: NavItem) => (
    <MotoNavIcon
      scene
      concept={item.concept}
      active={!item.comingSoon && isActiveLink(item.href)}
      className={SIDEBAR_ICON_CLASSES}
    />
  );

  // Zähler an der Zeile; wie er beim Klappen wandert, regelt die Kit-Zeile.
  const rowBadge = (href: string): SidebarRowBadge | null => {
    const count = rowBadgeCounts[href] ?? 0;
    const badge = ROW_BADGE_LABELS[href];
    if (count <= 0 || !badge) return null;
    return {
      count,
      tone: badge.tone,
      ariaLabel: counterAriaLabel(count, badge),
    };
  };

  const collapseState = { collapsed, labelsMounted, labelsVisible };

  // Gemeinsame Klapp-Eigenschaften jedes Akkordeon-Bereichs. Der Klick aus
  // dem eingeklappten Streifen heraus klappt die Leiste auf und öffnet den
  // Bereich — sein Inhalt passt nicht in 64px Breite. `onExpandFromRail`
  // räumt dabei den Unterzustand des Bereichs auf, den der Streifen nicht
  // zeigen konnte.
  const sectionProps = (
    accordion: string,
    onToggle: () => void,
    onExpandFromRail?: () => void,
  ) => ({
    collapsed,
    labelsMounted,
    labelsVisible,
    onToggle: () => {
      if (!collapsed) {
        onToggle();
        return;
      }
      onExpandSidebar();
      onExpandFromRail?.();
      if (expanded !== accordion) onToggle();
    },
  });

  const renderNavItem = (item: NavItem) => {
    // "Bald"-Einträge sagen im Streifen nichts aus: ein Icon ohne Ziel und
    // ohne Hinweis. Sie bleiben der ausgeklappten Leiste vorbehalten.
    if (item.comingSoon && collapsed && !labelsMounted) return null;

    return (
      <div key={item.comingSoon ? item.label : item.href}>
        {item.comingSoon ? (
          <SidebarRowPlaceholder
            {...collapseState}
            icon={renderNavIcon(item)}
            label={item.label}
            hint="Bald"
            title={tParentNav("comingSoonTooltip")}
          />
        ) : (
          <SidebarRowLink
            {...collapseState}
            href={
              item.href === "/help"
                ? helpHref
                : TENANT_SCOPED_HREFS.has(item.href)
                  ? tenantPath(item.href)
                  : item.href
            }
            active={isActiveLink(item.href)}
            icon={renderNavIcon(item)}
            label={item.label}
            // Ziel der geführten Touren (#2832, #3748).
            data-setup-tour={`nav-${item.href}`}
            collapsedLabel={
              item.locked ? `${item.label}, nicht gebucht` : undefined
            }
            newTab={item.newTab}
            trailing={item.locked ? <ModuleLockMark /> : undefined}
            badge={item.locked ? null : rowBadge(item.href)}
          />
        )}
      </div>
    );
  };

  // Die Katalogseiten der Gruppen Eltern, Team und Planung als Zeilen, mit
  // denselben Sichtbarkeitsregeln wie früher als Unterpunkte ihrer
  // Akkordeons. Symbol aus dem geteilten Baum, damit Desktop und Mobil
  // dasselbe Bild zeigen.
  const catalogNavItems: NavItem[] = [
    ...parentSubPages,
    ...communicationSubPages,
    ...planningSubPages,
  ].flatMap((page) => {
    // Jede Katalogseite hat ein Bild (staff-nav-items.test.ts).
    const concept = STAFF_NAV_CONCEPTS[page.href];
    return concept
      ? [
          {
            href: page.href,
            label: page.label,
            concept,
            locked: "locked" in page ? page.locked : false,
          },
        ]
      : [];
  });

  // Alles, was die Person sehen darf, nach Pfad. Der Baum in
  // staff-navigation.ts bestimmt, wo (und ob) eine Zeile erscheint.
  const visibleItemsByHref = new Map<string, NavItem>();
  for (const item of [...filteredNavItems, ...catalogNavItems]) {
    visibleItemsByHref.set(item.href, item);
  }

  // Get current search params for group/room selection
  const currentGroupParam = searchParams.get("group");
  const currentRoomParam = searchParams.get("room");
  const currentSessionParam = searchParams.get("session");
  // Unter „Gruppen“ stehen nur die eigenen Gruppen; alle übrigen erreicht
  // man über die Auswahlseite (#4351).
  const personalGroups = useMemo(() => groups.filter(isOwnGroup), [groups]);

  // On child pages (e.g. student detail with ?from=/ogs-groups), determine
  // which sub-item should stay highlighted using the last selection of this account.
  const isChildOfAccordion =
    pathname.startsWith("/students/") && pathname !== "/students/search";
  const childFromParam = isChildOfAccordion ? fromParam : null;
  const memory = useSidebarMemory();
  const childGroupKey = memory.key("sidebar-last-group");
  const childRoomKey = memory.key("sidebar-last-room");
  const childSessionKey = memory.key("supervision-last-session");
  const childGroupId = useLocalStorageValue(
    childGroupKey ?? "",
    childGroupKey !== null &&
      (childFromParam?.startsWith("/ogs-groups") ?? false),
  );
  const childRoomId = useLocalStorageValue(
    childRoomKey ?? "",
    childRoomKey !== null &&
      (childFromParam?.startsWith("/active-supervisions") ?? false),
  );
  // Welche Gruppe die Gruppenseite zeigt: die aus `?group=`, sonst die
  // zuletzt gewählte, die die Seite ohne Parameter wieder öffnet. Ist die
  // gemerkte nicht (mehr) in der Liste, öffnet die Seite die erste Gruppe
  // (#6863).
  const onGroupPage =
    pathname.startsWith("/ogs-groups") &&
    !pathname.startsWith(OGS_GROUPS_OVERVIEW_PATH);
  const rememberedGroupId = useLocalStorageValue(
    childGroupKey ?? "",
    childGroupKey !== null && onGroupPage && !currentGroupParam,
  );
  const shownGroupParam =
    currentGroupParam ??
    (rememberedGroupId && groups.some((group) => group.id === rememberedGroupId)
      ? rememberedGroupId
      : null);
  // Eine fremde Gruppe, die gerade offen ist (direkt oder als Herkunft einer
  // Kinderseite), steht markiert unter den eigenen. Sonst sähe eine Leitung
  // ohne eigene Gruppe „Keine eigenen Gruppen“, während sie in einer Gruppe
  // ist, und nichts zeigte, wo sie gerade ist (#6863).
  const openGroupId = childGroupId ?? (onGroupPage ? shownGroupParam : null);
  const openForeignGroup = useMemo(
    () =>
      openGroupId
        ? groups.find((group) => group.id === openGroupId && !isOwnGroup(group))
        : undefined,
    [groups, openGroupId],
  );
  const childSessionId = useLocalStorageValue(
    childSessionKey ?? "",
    childSessionKey !== null &&
      (childFromParam?.startsWith("/active-supervisions") ?? false),
  );
  // „Aktuelle Aufsicht“ zeigt nur laufende Aufsichten: eigene, mit der
  // Schulübersicht auch fremde. Ein offener Raum (#3065) steht hier nur, wenn
  // die Person dort selbst Aufsicht führt (#4349). Alle anderen offenen Räume
  // erreicht sie über „Räume“, wo sie oben stehen (#4372).
  const supervisionNavRooms = useMemo(
    () =>
      supervisedRooms.filter(
        (room) => !room.isOpenRoom || room.isOwnSupervision,
      ),
    [supervisedRooms],
  );
  // Who opened what the URL shows, and whose groups and rooms the sidebar
  // holds (#5000). After a switch to another account the URL still shows what
  // the previous person opened, and the lists stay until the new account's
  // arrive. The sidebar remembers only what this account opened, and a name
  // only from this account's list. Declared before the effects below, so the
  // owners are current when those run.
  const shownLocation = `${pathname}|${currentGroupParam ?? ""}|${currentRoomParam ?? ""}`;
  const locationOwner = useArrivalOwner(shownLocation, memory.owner);
  const groupsOwner = useArrivalOwner(groups, memory.owner);
  const roomsOwner = useArrivalOwner(supervisedRooms, memory.owner);

  // Persist last selected sub-item per accordion section (per school and account).
  // Pages read this on mount to restore the user's last selection.
  useEffect(() => {
    if (locationOwner.current !== memory.owner) return;
    if (pathname.startsWith("/ogs-groups") && currentGroupParam) {
      memory.write("sidebar-last-group", currentGroupParam);
      const group = groups.find((g) => g.id.toString() === currentGroupParam);
      if (group && groupsOwner.current === memory.owner) {
        memory.write("sidebar-last-group-name", group.name);
      }
    }
  }, [memory, pathname, currentGroupParam, groups, locationOwner, groupsOwner]);

  useEffect(() => {
    if (locationOwner.current !== memory.owner) return;
    if (pathname.startsWith("/active-supervisions") && currentRoomParam) {
      memory.write("sidebar-last-room", currentRoomParam);
      const roomName = supervisedRooms.find(
        (r) => r.id === currentRoomParam,
      )?.name;
      if (roomName && roomsOwner.current === memory.owner) {
        memory.write("sidebar-last-room-name", roomName);
      }
    }
  }, [
    memory,
    pathname,
    currentRoomParam,
    supervisedRooms,
    locationOwner,
    roomsOwner,
  ]);

  useEffect(() => {
    if (locationOwner.current !== memory.owner) return;
    if (
      pathname.startsWith("/database/") &&
      databaseSubPages.some((p) => pathname === p.href)
    ) {
      memory.write("sidebar-last-database", pathname);
    }
  }, [memory, pathname, databaseSubPages, locationOwner]);

  // Toggle accordion AND navigate to the correct URL (with last-selected sub-item).
  // Reads the memory at click-time so the page loads with the right param immediately.
  // „Gruppen“ führt auf die Auswahlseite (#4351), so wie die
  // Datenverwaltung auf ihren Hub. Auf der Auswahlseite selbst klappt der
  // Eintrag nur auf und zu; ebenso in der einzigen Gruppe, denn die
  // Auswahlseite öffnet sie dann ohnehin wieder direkt.
  const handleGroupsToggle = useCallback(() => {
    const onGroupsPage = pathname.startsWith("/ogs-groups");
    if (
      pathname.startsWith(OGS_GROUPS_OVERVIEW_PATH) ||
      (onGroupsPage && groups.length <= 1)
    ) {
      toggle("groups");
      return;
    }
    if (!onGroupsPage) toggle("groups");
    router.push(OGS_GROUPS_OVERVIEW_PATH);
  }, [groups.length, pathname, router, toggle]);

  const handleSupervisionsToggle = useCallback(() => {
    // Aus einem offenen Raum ohne eigene Aufsicht heraus führt der Eintrag
    // in die eigene Aufsicht und bleibt dabei aufgeklappt.
    if (!isViewingSharedOpenRoom || expanded !== "supervisions") {
      toggle("supervisions");
    }
    if (
      !pathname.startsWith("/active-supervisions") ||
      isViewingSharedOpenRoom
    ) {
      // Own supervisions use their session key (#2265). A saved session that
      // belongs to a released room instead identifies that room's shared view.
      // Only entries listed under „Aktuelle Aufsicht“ qualify: a released room
      // without an own supervision lives under „Räume“ (#4372).
      const savedSessionId = memory.read("supervision-last-session");
      const savedRoomId = memory.read("sidebar-last-room");
      const targetRoom =
        (savedSessionId
          ? (supervisionNavRooms.find(
              (r) => !r.isOpenRoom && r.groupId === savedSessionId,
            ) ??
            supervisionNavRooms.find((r) =>
              r.sessionIds?.includes(savedSessionId),
            ))
          : undefined) ??
        (savedRoomId
          ? supervisionNavRooms.find((r) => r.id === savedRoomId)
          : undefined) ??
        // Ohne gespeicherte Wahl die eigene Aufsicht, nicht die erste fremde
        // aus dem Überblick (#5035).
        supervisionNavRooms.find((r) => r.isOwnSupervision) ??
        supervisionNavRooms[0];
      if (targetRoom) {
        router.push(supervisionHref(targetRoom));
      } else {
        router.push("/active-supervisions");
      }
    }
  }, [
    toggle,
    expanded,
    memory,
    pathname,
    isViewingSharedOpenRoom,
    supervisionNavRooms,
    router,
  ]);

  const handleDatabaseToggle = useCallback(() => {
    // Die Leitung landet auf dem Hub. Delegierte Personen haben nur einen
    // oder mehrere Kataloge und starten deshalb beim ersten erreichbaren
    // Unterpunkt statt auf einer fast leeren Hub-Seite.
    const databaseLandingPath =
      userHasEffectiveAdminScope || userLeadsSchool
        ? "/database"
        : databaseSubPages[0]?.href;
    if (!pathname.startsWith("/database")) {
      // Not on any database page, expand accordion and navigate to its
      // reachable landing page.
      toggle("database");
      if (databaseLandingPath) router.push(databaseLandingPath);
    } else if (pathname === "/database") {
      // On hub page, just toggle collapse or expand
      toggle("database");
    } else if (!userHasEffectiveAdminScope) {
      // Delegated users have no hub to return to; keep them on their allowed
      // catalog and let the control act as the accordion toggle.
      toggle("database");
    } else {
      // On a sub-page like /database/rooms, navigate back to hub
      router.push("/database");
    }
  }, [
    databaseSubPages,
    toggle,
    pathname,
    router,
    userHasEffectiveAdminScope,
    userLeadsSchool,
  ]);

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

  // Everyone with group-read access sees own supervisions. A successful
  // overview request also covers effective admins and verified staff under
  // all_staff (#2380).
  // overviewEnabled keeps a released room (#3065) from opening the accordion:
  // it is reachable for everyone and grants no supervision, so it must not
  // stand for one when the school keeps everyone on their own supervisions.
  const showStaffAccordions = canReadSupervisions || overviewEnabled;
  const showGroupAccordion = showStaffAccordions || userHasEffectiveAdminScope;

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
      <SidebarRowLink
        {...collapseState}
        key={item.href}
        href={item.href}
        active={isActiveLink(item.href)}
        icon={renderNavIcon(item)}
        label={item.label}
      />
    );

    return (
      <PortalSidebarFrame
        collapsed={collapsed}
        className={`${SIDEBAR_MIN_HEIGHT} ${className}`}
        topOffset={portalSidebarTopOffset(
          isPreview === true || demoBannerShown,
          eveningRow,
        )}
      >
        <nav
          className={`${collapsed ? "scrollbar-hidden" : ""} flex-1 overflow-y-auto ${SIDEBAR_NAV_PADDING}`}
        >
          {resolvedOperatorSections.map((section, index) => (
            <div key={section.label} className={index > 0 ? "mt-5" : undefined}>
              <SidebarSectionHeading
                collapsed={collapsed}
                labelsVisible={labelsVisible}
              >
                {section.label}
              </SidebarSectionHeading>
              <div className={SIDEBAR_NAV_GAP}>
                {section.items.map(renderOperatorItem)}
              </div>
            </div>
          ))}
        </nav>
      </PortalSidebarFrame>
    );
  }

  // Die vier Akkordeon-Bereiche, jeder als Zeile innerhalb seiner Gruppe
  // (#2826). Gruppen: tenant staff/admin, hidden in open-care mode. Ein
  // Eintrag führt auf die Auswahlseite, darunter stehen die eigenen Gruppen
  // (#4351).
  const renderGroupsSection = () =>
    showGroupAccordion && !openCareGroupMode && !childSearchOnlyNavigation ? (
      <SidebarAccordionSection
        concept={groupsConcept}
        label={OGS_GROUPS_LABEL}
        tourId="nav-section-groups"
        isExpanded={expanded === "groups"}
        {...sectionProps("groups", handleGroupsToggle)}
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
        emptyText="Keine eigenen Gruppen"
        hasChildren={personalGroups.length > 0 || Boolean(openForeignGroup)}
      >
        {personalGroups.map((group, index) => (
          <SidebarSubItem
            key={group.id}
            href={ogsGroupHref(group.id)}
            label={group.name}
            count={formatGroupAttendanceCount(group.id)}
            isActive={isGroupSubItemActive(
              childGroupId,
              group.id.toString(),
              pathname,
              shownGroupParam,
              index,
            )}
          />
        ))}
        {openForeignGroup ? (
          <SidebarSubItem
            key={openForeignGroup.id}
            href={ogsGroupHref(openForeignGroup.id)}
            label={openForeignGroup.name}
            count={formatGroupAttendanceCount(openForeignGroup.id)}
            isActive
          />
        ) : null}
      </SidebarAccordionSection>
    ) : null;

  // Aktuelle Aufsicht (staff only; hidden in binary mode because room-level
  // supervision has no meaning without visits). Offene Räume ohne eigene
  // Aufsicht stehen unter „Räume“ (#4372). Mit `schedules:read` allein
  // (#4573) zeigt die Seite die eigene laufende Aufsicht.
  const renderSupervisionsSection = () =>
    (overviewEnabled || canOpenActiveSupervision(session)) &&
    !isBinaryMode &&
    !childSearchOnlyNavigation ? (
      <>
        <SidebarAccordionSection
          concept={supervisionConcept}
          label={
            supervisionNavRooms.length > 1
              ? "Aktuelle Aufsichten"
              : "Aktuelle Aufsicht"
          }
          tourId="nav-section-supervisions"
          isExpanded={expanded === "supervisions"}
          {...sectionProps("supervisions", handleSupervisionsToggle)}
          isActive={
            !isViewingSharedOpenRoom &&
            isAccordionSectionActive(
              "/active-supervisions",
              Boolean(currentRoomParam) ||
                Boolean(childRoomId) ||
                supervisionNavRooms.length > 0,
            )
          }
          isIconActive={
            !isViewingSharedOpenRoom &&
            (pathname.startsWith("/active-supervisions") ||
              Boolean(childRoomId))
          }
          isLoading={isLoadingSupervision}
          emptyText="Keine aktive Aufsicht"
          hasChildren={supervisionNavRooms.length > 0}
        >
          {supervisionNavRooms.map((room, index) => (
            <SidebarSubItem
              key={`${room.id}-${room.isOpenRoom ? "open" : room.groupId}`}
              href={supervisionHref(room)}
              label={room.name}
              isActive={isRoomSubItemActive(
                childSessionId,
                childRoomId,
                room.isOpenRoom ? null : room.groupId,
                room.sessionIds ?? [],
                room.id,
                pathname,
                currentSessionParam,
                currentRoomParam,
                index,
              )}
            />
          ))}
        </SidebarAccordionSection>
      </>
    ) : null;

  // Datenverwaltung: Hub-Seite plus Unterseiten für berechtigte Personen.
  const renderDatabaseSection = () =>
    databaseSubPages.length > 0 ? (
      <SidebarAccordionSection
        concept={databaseConcept}
        label={DATABASE_SECTION.label}
        isExpanded={expanded === "database"}
        tourId="nav-database"
        {...sectionProps("database", handleDatabaseToggle)}
        isActive={isAccordionSectionActive(
          "/database",
          databaseSubPages.some(
            (p) => pathname === p.href || objectRouteActiveHref === p.href,
          ),
        )}
        isIconActive={
          pathname.startsWith("/database") ||
          objectRouteActiveHref?.startsWith("/database/") === true
        }
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
            locked={page.locked}
            tourId={`nav-${page.href}`}
            // Auch aktiv, wenn die Objektroute (Kindakte, Personalakte,
            // Raumseite) aus diesem Register geöffnet wurde (#3115).
            isActive={
              pathname === page.href || objectRouteActiveHref === page.href
            }
          />
        ))}
      </SidebarAccordionSection>
    ) : null;

  // Anmeldungen: the setup hub, enrollment periods, offers and enrollment
  // forms. Every route behind it needs config:manage, so the section follows
  // the school lead (#3469). Sits in the Eltern group, below the parent pages.
  // Ohne das Modul Online-Anmeldung steht für die Leitung eine Zeile mit
  // Schloss statt des Bereichs (#4894).
  const enrollmentsVisibility = moduleNav.section("enrollments");
  // Ungelesene Anmeldungen (#3778): am Bereich, an seinem Überblick und in
  // der Summe der Gruppe Eltern derselbe Zähler.
  const enrollmentsBadge: SidebarRowBadge = {
    count: enrollmentsUnreadCount,
    tone: COUNTER_TONE.staff,
    ariaLabel: counterAriaLabel(enrollmentsUnreadCount, ENROLLMENTS_COUNTER),
  };
  const showEnrollments = userLeadsSchool && enrollmentsVisibility === "open";
  const renderEnrollmentsSection = () =>
    userLeadsSchool && enrollmentsVisibility === "locked" ? (
      renderNavItem({
        href: ENROLLMENT_SECTION.href,
        label: ENROLLMENT_SECTION.label,
        concept: enrollmentsConcept,
        locked: true,
      })
    ) : showEnrollments ? (
      <SidebarAccordionSection
        concept={enrollmentsConcept}
        label={ENROLLMENT_SECTION.label}
        isExpanded={expanded === "enrollments"}
        {...sectionProps("enrollments", handleEnrollmentsToggle)}
        isActive={isOnEnrollmentsPage}
        isIconActive={isOnEnrollmentsPage}
        hasChildren={ENROLLMENT_SUB_PAGES.length > 0}
        badge={enrollmentsBadge}
      >
        {ENROLLMENT_SUB_PAGES.map((page) => (
          <SidebarSubItem
            key={page.href}
            href={page.href}
            label={page.label}
            isActive={activeEnrollmentSubPageHref === page.href}
            badge={
              page.href === ENROLLMENT_SECTION.href ? enrollmentsBadge : null
            }
          />
        ))}
      </SidebarAccordionSection>
    ) : null;

  const renderSection = (key: StaffNavSectionKey) => {
    switch (key) {
      case "groups":
        return renderGroupsSection();
      case "supervisions":
        return renderSupervisionsSection();
      case "database":
        return renderDatabaseSection();
      case "enrollments":
        return renderEnrollmentsSection();
    }
  };

  // Eine Zeile des Baums: eine sichtbare Seite oder ein Akkordeon-Bereich.
  // `null`, wenn die Person die Seite nicht sehen darf — die Gruppe rückt
  // dann zusammen.
  const renderEntry = (entry: StaffNavEntry) => {
    if (entry.kind === "page") {
      const item = visibleItemsByHref.get(entry.href);
      return item ? renderNavItem(item) : null;
    }
    const node = renderSection(entry.section);
    return node ? <Fragment key={entry.section}>{node}</Fragment> : null;
  };

  // Steht die aktuelle Seite in diesem Eintrag? Die Kopfzeile einer
  // zugeklappten Gruppe zeigt das an ihrem Icon.
  const isEntryActive = (entry: StaffNavEntry): boolean => {
    if (entry.kind === "page") return isActiveLink(entry.href);
    switch (entry.section) {
      case "groups":
        return pathname.startsWith("/ogs-groups") || Boolean(childGroupId);
      case "supervisions":
        return (
          pathname.startsWith("/active-supervisions") || Boolean(childRoomId)
        );
      case "database":
        return pathname.startsWith("/database");
      case "enrollments":
        return isOnEnrollmentsPage;
    }
  };

  // Was eine Zeile der Gruppe zählt; die Kopfzeile der Gruppe nennt jeden
  // Teil (#6566).
  const entryBadge = (entry: StaffNavEntry): SidebarRowBadge | null => {
    if (entry.kind === "page") {
      const item = visibleItemsByHref.get(entry.href);
      return item && !item.locked ? rowBadge(entry.href) : null;
    }
    return entry.section === "enrollments" && showEnrollments
      ? enrollmentsBadge
      : null;
  };

  return (
    <PortalSidebarFrame
      collapsed={collapsed}
      className={`${SIDEBAR_MIN_HEIGHT} ${className}`}
      topOffset={portalSidebarTopOffset(
        isPreview === true || demoBannerShown,
        eveningRow,
      )}
    >
      {/* Rückweg ins Träger-Portal (#5687): nur in einer Sitzung, die über
            das Träger-Büro kam. Außerhalb der Rollfläche, damit er immer
            sichtbar bleibt. */}
      {traegerReturnHref ? (
        <nav
          aria-label="Rückweg"
          className={`border-b border-gray-200 ${SIDEBAR_NAV_PADDING}`}
        >
          <SidebarRowLink
            {...collapseState}
            href={traegerReturnHref}
            active={false}
            label={TRAEGER_RETURN_LABEL}
            icon={
              <MotoNavIcon
                scene
                concept={organizationsConcept}
                active={false}
                className={SIDEBAR_ICON_CLASSES}
              />
            }
          />
        </nav>
      ) : null}

      {/* In den Einstellungen wechselt der Inhalt derselben Leiste (#5750). */}
      {isStaffSettingsPath(pathname) ? (
        <StaffSettingsSidebarNav
          collapsed={collapsed}
          labelsMounted={labelsMounted}
          labelsVisible={labelsVisible}
        />
      ) : (
        <>
          {/* Main navigation, scrollable.
                Der Rollbalken bleibt ausgeklappt sichtbar — er ist dort der
                einzige Hinweis, dass unten noch Einträge folgen. Nur im
                64px-Streifen wird er ausgeblendet, wo er ein Viertel der Breite
                einnähme (Muster: VS-Code-Aktivitätsleiste). */}
          <nav
            className={`${collapsed ? "scrollbar-hidden" : ""} flex-1 overflow-y-auto ${SIDEBAR_NAV_GAP} ${SIDEBAR_NAV_PADDING}`}
          >
            {/* Startseite der Rolle: Home für Admins, Tagesplan (#2383) für
                  Betreuungskräfte — über den Gruppen, wie bisher. */}
            {STAFF_NAV_TOP.map(renderEntry)}

            {/* Einfache Seitenleiste (#6956): die Seiten des
                  Betreuungsalltags als eine Liste, ohne Gruppen zum
                  Aufklappen. */}
            {simpleNav.active
              ? STAFF_NAV_SIMPLE_ENTRIES.map(renderEntry)
              : null}

            {/* Die fünf Gruppen (#2826), in der Reihenfolge des Baums. Eine
                  Gruppe ohne sichtbare Zeile fällt weg. */}
            {simpleNav.active
              ? null
              : STAFF_NAV_GROUPS.map((group) => {
                  const rows = group.entries.map(renderEntry);
                  if (!rows.some(Boolean)) return null;
                  return (
                    <div key={group.key} className="pt-2">
                      <SidebarGroup
                        label={group.label}
                        icon={group.icon}
                        tourId={`nav-group-${group.key}`}
                        isOpen={isGroupOpen(group.key)}
                        onToggle={() => toggleGroup(group.key)}
                        containsActive={group.entries.some(isEntryActive)}
                        badge={combineSidebarRowBadges(
                          group.entries.map(entryBadge),
                          group.key === "eltern"
                            ? COUNTER_TONE.parents
                            : COUNTER_TONE.staff,
                        )}
                        collapsed={collapsed}
                        labelsMounted={labelsMounted}
                        labelsVisible={labelsVisible}
                      >
                        {rows}
                      </SidebarGroup>
                    </div>
                  );
                })}
          </nav>

          {/* Bottom pinned items — gleiches Raster wie oben, damit der untere
                Block in beiden Zuständen zur Hauptnavigation passt. */}
          <nav
            className={`border-t border-gray-200 ${SIDEBAR_NAV_GAP} ${SIDEBAR_NAV_PADDING}`}
          >
            {STAFF_NAV_BOTTOM.map(renderEntry)}
            {/* Ausgeblendet heißt nicht gesperrt (#6956): jede Person
                  wechselt selbst zwischen beiden Ansichten. */}
            {simpleNav.available ? (
              <SidebarRowButton
                {...collapseState}
                label={simpleNav.toggleLabel}
                icon={
                  <NavSceneIcon
                    icon={ListBulletsIcon}
                    className={SIDEBAR_ICON_CLASSES}
                  />
                }
                onClick={simpleNav.toggle}
              />
            ) : null}
          </nav>
        </>
      )}
    </PortalSidebarFrame>
  );
}

export function Sidebar({ className = "" }: SidebarProps) {
  // Klappzustand lebt hier statt in SidebarContent, damit der Suspense-
  // Fallback dieselbe Breite rendert und beim Laden nichts springt.
  const { collapsed, expandSidebar } = useSidebarCollapsed();
  return (
    <Suspense
      fallback={
        <PortalSidebarFrame
          collapsed={collapsed}
          className={`${SIDEBAR_MIN_HEIGHT} ${className}`}
        >
          <div className={SIDEBAR_NAV_PADDING}>
            {/* Platzhalter im selben Raster wie die fertigen Zeilen: 40px
                hoch, Icon an derselben Stelle — der Wechsel vom Platzhalter
                zur Navigation verschiebt nichts. */}
            <nav className={SIDEBAR_NAV_GAP}>
              {["w-24", "w-28", "w-20", "w-24"].map((widthClass, index) => (
                <div key={index} className="flex h-10 items-center px-3">
                  <Skeleton className="h-5 w-5 shrink-0" />
                  {!collapsed && (
                    <Skeleton className={`ml-3 h-4 ${widthClass}`} />
                  )}
                </div>
              ))}
            </nav>
          </div>
        </PortalSidebarFrame>
      }
    >
      <SidebarContent
        className={className}
        collapsed={collapsed}
        onExpandSidebar={expandSidebar}
      />
    </Suspense>
  );
}
