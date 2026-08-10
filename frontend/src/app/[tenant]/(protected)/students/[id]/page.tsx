"use client";

import { Suspense, useState, useEffect, useMemo, useCallback } from "react";
import {
  useParams,
  usePathname,
  useRouter,
  useSearchParams,
} from "next/navigation";
import { useSession } from "next-auth/react";
import { useSWRConfig } from "swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { hasPermission } from "~/lib/auth-utils";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import { BackButton } from "~/components/ui/back-button";
import { CustomSelect } from "~/components/ui/custom-select";
import { studentService } from "~/lib/api";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup } from "~/lib/active-helpers";
import {
  useStudentData,
  type ExtendedStudent,
} from "~/lib/hooks/use-student-data";
import { useStudentEnrollmentExtraFields } from "~/lib/hooks/use-student-enrollment-extra-fields";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useSWRAuth } from "~/lib/swr";
import type { SupervisorContact } from "~/lib/student-helpers";
import {
  allowedDepartureModesFromDeparture,
  allowedDepartureToDepartureDays,
  departureDaysFromLegacy,
  normalizeBusDays,
  normalizeAllowedDepartureModes,
} from "~/lib/student-helpers";
import {
  StudentDetailHeader,
  SupervisorsCard,
  PersonalInfoReadOnly,
  StudentHistorySection,
} from "~/components/students/student-detail-components";
import { PersonalInfoFormModal } from "~/components/students/personal-info-form-modal";
import { ParentMessagesCard } from "~/components/students/parent-messages-card";
import { StudentEnrollmentsTab } from "~/components/students/student-enrollments-tab";
import { StudentDokumenteTab } from "~/components/students/dokumente-tab";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  StudentSickReportSection,
  StudentExcusedReportSection,
  StudentStatusActionsMenu,
  getStudentActionType,
} from "~/components/students/student-checkout-section";
import { performImmediateCheckin } from "~/lib/checkin-api";
import { createLogger } from "~/lib/logger";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";
import { CarePlanView } from "~/components/students/care-plan-view";
import { CareScheduleManager } from "~/components/students/care-schedule-manager";
import { PlannedStatusDaysModal } from "~/components/students/planned-status-days-modal";
import { fetchStudentPickupData } from "~/lib/pickup-schedule-api";
import { getDayData, formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { fetchArrivalData } from "~/lib/student-arrival-api";
import {
  getDayData as getArrivalDayData,
  formatArrivalTime,
} from "~/lib/arrival-schedule-helpers";
import {
  createStudentStatusDays,
  deleteStudentStatusDay,
  fetchStudentStatusDays,
  StudentStatusDayConflictError,
  type StudentStatusDay,
  type StudentStatusKind,
} from "~/lib/student-status-days-api";
import { formatDate as formatCalendarDate } from "~/lib/date-helpers";
import { StudentDetailSkeleton } from "./page-skeleton";

type TodayArrival = {
  time?: string;
  note?: string;
  isException?: boolean;
  isAbsent?: boolean;
};

const logger = createLogger({ component: "StudentDetailPage" });

// Tabbed navigation for the student detail page (issue #1501). The cross-cutting
// action bar (check-in/out, Krank/Entschuldigt) and the attendance header stay
// ABOVE the tabs — only the data sections are grouped into tabs. The active tab
// lives in the `?tab=` query param so sections are deep-linkable (acceptance
// criterion: "navigate directly to the relevant section").
type StudentTabId =
  | "stammdaten"
  | "nachrichten"
  | "erziehungsberechtigte"
  | "betreuungsplan"
  | "betreuungszeiten"
  | "anmeldungen"
  | "dokumente"
  | "historie";

const TAB_LABELS: Record<StudentTabId, string> = {
  stammdaten: "Stammdaten",
  nachrichten: "Nachrichten",
  erziehungsberechtigte: "Erziehungsberechtigte",
  betreuungsplan: "Betreuungsplan",
  betreuungszeiten: "Betreuungszeiten",
  anmeldungen: "Anmeldungen",
  dokumente: "Dokumente",
  historie: "Historie",
};

// Limited access has no care-schedule data access, so it skips Betreuungszeiten.
// The Nachrichten tab is full-access only — limited-access staff don't see the
// parent-message overview (the backend gates per-child read access anyway).
const FULL_ACCESS_BASE_TABS: StudentTabId[] = [
  "stammdaten",
  "nachrichten",
  "erziehungsberechtigte",
  "betreuungsplan",
  "betreuungszeiten",
  "dokumente",
  "historie",
];
const LIMITED_ACCESS_BASE_TABS: StudentTabId[] = [
  "stammdaten",
  "erziehungsberechtigte",
  "historie",
];
const FULL_ACCESS_TABS_WITH_ENROLLMENTS: StudentTabId[] = [
  "stammdaten",
  "nachrichten",
  "erziehungsberechtigte",
  "betreuungsplan",
  "betreuungszeiten",
  "anmeldungen",
  "dokumente",
  "historie",
];
const LIMITED_ACCESS_TABS_WITH_ENROLLMENTS: StudentTabId[] = [
  "stammdaten",
  "erziehungsberechtigte",
  "anmeldungen",
  "historie",
];

const DEFAULT_TAB: StudentTabId = "stammdaten";

function resolveActiveTab(
  param: string | null,
  allowed: StudentTabId[],
): StudentTabId {
  return allowed.find((tab) => tab === param) ?? DEFAULT_TAB;
}

/**
 * @param hasStudentReadAccess the backend's READ predicate for this child,
 *   `has_full_access` on the student response. Despite the wire name this is
 *   NOT the strict supervisor/admin check — that one is `has_write_access`.
 *   It resolves to `authorize.CanReadStudent`, which honours
 *   `gdpr.student_data_scope`: under `all_staff` EVERY staff member gets `true`
 *   here, under `group_supervisors_only` only the child's supervisors (and
 *   admins) do. See api/students/authorization.go — `checkStudentReadAccess`
 *   (scope-aware, this flag) vs `checkStudentFullAccess` (scope-ignoring, the
 *   write flag).
 */
function studentTabs(
  hasStudentReadAccess: boolean,
  canViewEnrollments: boolean,
  canViewCarePlan: boolean,
  canViewDocuments: boolean,
): StudentTabId[] {
  const base = hasStudentReadAccess
    ? canViewEnrollments
      ? FULL_ACCESS_TABS_WITH_ENROLLMENTS
      : FULL_ACCESS_BASE_TABS
    : canViewEnrollments
      ? LIMITED_ACCESS_TABS_WITH_ENROLLMENTS
      : LIMITED_ACCESS_BASE_TABS;
  // The Betreuungsplan tab reads the timetable (backend requires schedules:read
  // on /timetable/student/{id}/day|week); hide it without that permission so
  // the user can't open a tab that only returns 403s.
  //
  // It is absent from the limited-access sets for the SAME reason, not as an
  // oversight: those routes gate on read access per student too
  // (resolveStudentForRead → authorize.CanReadStudent in
  // api/timetable/student_day.go) — the SAME predicate behind the flag above,
  // so the two can never disagree. A staff member who may read the child under
  // `all_staff` therefore lands in the full-access set and DOES get the tab;
  // one who may not gets 403 from the care-plan endpoints, so widening the tab
  // would only surface a permanently failing panel, and ?tab=betreuungsplan is
  // clamped away for the same reason. Widening staff access to a child's plan
  // is a backend (gdpr.student_data_scope) decision, not a frontend one.
  const withCarePlan = canViewCarePlan
    ? base
    : base.filter((tab) => tab !== "betreuungsplan");
  // Dokumente (#777) mirrors the backend route gate exactly:
  // RequiresAnyPermission(users:update, student_documents:health,
  // student_documents:legal). Gating on write access instead would disagree in
  // both directions — a group supervisor without users:update would see a tab
  // that only answers 403, and a role holding just student_documents:health
  // (which the migration exists to make grantable) could never reach the tab
  // at all.
  return canViewDocuments
    ? withCarePlan
    : withCarePlan.filter((tab) => tab !== "dokumente");
}

// Shared classes for every tab panel. forceMount (below) keeps inactive panels
// mounted. This is deliberate, not just pre-tabs parity: the panel children
// (CareScheduleManager, StudentGuardianManager) fetch on mount and do NOT cache,
// so the lazy alternative — letting Radix unmount inactive panels — would
// re-fire those network calls on every tab revisit. forceMount loads each once
// and keeps every section reachable for deep links. Tradeoff, accepted on
// purpose: every section fetches up front on page open (no lazy-per-tab win),
// but that matches the pre-tabs behaviour where all sections rendered together,
// so it is not a regression. It disables Radix's own
// `hidden` attribute, so `data-[state=inactive]:hidden` does the hiding via CSS
// (display:none — also removes inactive panels from the a11y tree).
const TAB_CONTENT_CLASS =
  "mt-4 focus-visible:ring-0 focus-visible:ring-offset-0 data-[state=inactive]:hidden sm:mt-6";

function StudentTabsList({ tabs }: Readonly<{ tabs: StudentTabId[] }>) {
  return (
    <div className="overflow-x-auto border-b border-gray-200">
      {/* border-b-0 here: the wrapper above already draws the full-width rail,
          so the line variant's own border-b would stack a second 1px line under
          the labels. Matches the detail-panel precedent (database/detail-panel.tsx). */}
      <TabsList variant="line" className="w-max justify-start border-b-0">
        {tabs.map((tab) => (
          <TabsTrigger key={tab} value={tab}>
            {TAB_LABELS[tab]}
          </TabsTrigger>
        ))}
      </TabsList>
    </div>
  );
}

function formatLocalDate(date: Date): string {
  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  const day = date.getDate().toString().padStart(2, "0");
  return `${year}-${month}-${day}`;
}

interface StatusDayRange {
  readonly from: string;
  readonly to: string;
}

function getStatusDayRange(): StatusDayRange {
  const from = new Date();
  from.setDate(from.getDate() - 14);
  const to = new Date();
  to.setDate(to.getDate() + 90);
  return {
    from: formatLocalDate(from),
    to: formatLocalDate(to),
  };
}

function extendStatusDayRange(
  range: StatusDayRange,
  dates: string[],
): StatusDayRange {
  let from = range.from;
  let to = range.to;
  for (const date of dates) {
    if (date < from) from = date;
    if (date > to) to = date;
  }
  return from === range.from && to === range.to ? range : { from, to };
}

function mergeStatusDays(
  current: StudentStatusDay[] | undefined,
  incoming: StudentStatusDay[],
): StudentStatusDay[] {
  const incomingDates = new Set(incoming.map((day) => day.date));
  const byId = new Map<string, StudentStatusDay>();
  for (const day of current ?? []) {
    if (incomingDates.has(day.date)) continue;
    byId.set(day.id, day);
  }
  for (const day of incoming) {
    byId.set(day.id, day);
  }
  return Array.from(byId.values()).sort((a, b) => a.date.localeCompare(b.date));
}

// =============================================================================
// MAIN COMPONENT
// =============================================================================

export default function StudentDetailPage() {
  return (
    <Suspense fallback={null}>
      <StudentDetailPageContent />
    </Suspense>
  );
}

function StudentDetailPageContent() {
  const { mutate } = useSWRConfig();
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const navRouter = useRouter();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  const toast = useToast();
  const { data: session, status: sessionStatus } = useSession();

  // Switch tabs by updating the `?tab=` query param in place (preserves the
  // `from` referrer). We echo the current `usePathname` back verbatim and only
  // swap the query string, so no manual slug prefixing is needed: in subdomain
  // mode `usePathname` is already slug-free (`/students/1`) and in path mode it
  // already carries the slug (`/school-a/students/1`). This is the same
  // browser-visible-path behaviour that `sidebar.tsx` relies on (it strips
  // `/${tenantSlug}/` only when present) — verified against that precedent, so
  // there is no double-slug risk in either mode. scroll:false keeps the
  // viewport steady when switching sections.
  const handleTabChange = useCallback(
    (next: string) => {
      const query = new URLSearchParams(searchParams.toString());
      if (next === DEFAULT_TAB) {
        query.delete("tab");
      } else {
        query.set("tab", next);
      }
      const qs = query.toString();
      navRouter.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [navRouter, pathname, searchParams],
  );

  // Start at the top instead of inheriting the search list's scroll position
  // (Next App Router maintains scroll when the new page is already in view).
  useScrollToTop(studentId);

  // Use custom hook for data fetching
  const {
    student,
    loading,
    error,
    hasFullAccess,
    hasWriteAccess,
    attendanceLogEnabled,
    feedbackEnabled,
    supervisors,
    myGroups,
    myGroupRooms,
    mySupervisedRooms,
    refreshData,
  } = useStudentData(studentId);
  const refreshDataAndHistory = useCallback(() => {
    refreshData();
    return mutate(`/api/students/${studentId}/change-history`).catch((err) => {
      logger.debug("change_history_revalidation_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
    });
  }, [mutate, refreshData, studentId]);
  const canViewEnrollments =
    sessionStatus === "authenticated" &&
    hasPermission(session, "config:manage");
  const canViewCarePlan =
    sessionStatus === "authenticated" &&
    hasPermission(session, "schedules:read");
  // Same three permissions the backend route gate accepts (#777).
  const canViewDocuments =
    sessionStatus === "authenticated" &&
    (hasPermission(session, "users:update") ||
      hasPermission(session, "student_documents:health") ||
      hasPermission(session, "student_documents:legal"));
  const visibleTabs = useMemo(
    () =>
      studentTabs(
        hasFullAccess,
        canViewEnrollments,
        canViewCarePlan,
        canViewDocuments,
      ),
    [canViewEnrollments, canViewCarePlan, hasFullAccess, canViewDocuments],
  );
  const tabResolutionTabs =
    sessionStatus === "loading"
      ? hasFullAccess
        ? FULL_ACCESS_TABS_WITH_ENROLLMENTS
        : LIMITED_ACCESS_TABS_WITH_ENROLLMENTS
      : visibleTabs;

  // Set breadcrumb data, include group/room name for 3-level breadcrumb
  // when navigating from an accordion section (e.g. Meine Gruppe > 1a > Mia Fischer)
  const breadcrumbGroupName = useLocalStorageValue(
    "sidebar-last-group-name",
    referrer.startsWith("/ogs-groups"),
  );
  const breadcrumbRoomName = useLocalStorageValue(
    "sidebar-last-room-name",
    referrer.startsWith("/active-supervisions"),
  );

  useSetBreadcrumb({
    studentName: student?.name,
    referrerPage: referrer,
    ogsGroupName: breadcrumbGroupName ?? undefined,
    activeSupervisionName: breadcrumbRoomName ?? undefined,
  });

  // Personal info modal state
  const [showPersonalInfoModal, setShowPersonalInfoModal] = useState(false);

  // Checkout states
  const [showConfirmCheckout, setShowConfirmCheckout] = useState(false);
  const [checkingOut, setCheckingOut] = useState(false);

  // Check-in states
  const [showConfirmCheckin, setShowConfirmCheckin] = useState(false);
  const [checkingIn, setCheckingIn] = useState(false);

  // Sick toggle state
  const [showConfirmSick, setShowConfirmSick] = useState(false);
  const [sickLoading, setSickLoading] = useState(false);
  const [sickReason, setSickReason] = useState("");
  const sickConfirmText = sickLoading
    ? "Wird gespeichert..."
    : student?.sick
      ? "Gesundmelden"
      : "Krankmelden";

  // Excused toggle state
  const [showConfirmExcused, setShowConfirmExcused] = useState(false);
  const [excusedLoading, setExcusedLoading] = useState(false);
  const isQuickExcused = (student?.excused ?? false) && !student?.class_trip;
  const excusedConfirmText = excusedLoading
    ? "Wird gespeichert..."
    : isQuickExcused
      ? "Entschuldigung aufheben"
      : "Entschuldigen";

  // Switch dialog: shown when the user clicks one flag but the other is set.
  // "sick" = we want to switch TO sick (excused is currently true).
  // "excused" = we want to switch TO excused (sick is currently true).
  const [switchTarget, setSwitchTarget] = useState<"sick" | "excused" | null>(
    null,
  );
  const [switchLoading, setSwitchLoading] = useState(false);
  const [plannedStatusModal, setPlannedStatusModal] =
    useState<StudentStatusKind | null>(null);
  const [plannedStatusLoading, setPlannedStatusLoading] = useState(false);
  const [deletingPlannedStatusDayId, setDeletingPlannedStatusDayId] = useState<
    string | null
  >(null);
  const [selectedActiveGroupId, setSelectedActiveGroupId] =
    useState<string>("");
  const [activeGroups, setActiveGroups] = useState<ActiveGroup[]>([]);
  const [loadingActiveGroups, setLoadingActiveGroups] = useState(false);

  // Today's pickup info (for header display). SWR-cached under a
  // "pickup-data-" key like its arrival twin below, so a Gehzeit write
  // elsewhere in the school reaches this header: the global SSE hook
  // invalidates that key prefix on pickup_schedule_changed. A plain
  // fetch-on-mount effect could not be woken that way and left the header
  // showing the previous pickup time until a manual reload.
  const { data: pickupData } = useSWRAuth(
    hasFullAccess && studentId ? `pickup-data-${studentId}` : null,
    async () => fetchStudentPickupData(studentId),
    { revalidateOnFocus: false },
  );

  const { data: arrivalData } = useSWRAuth(
    hasFullAccess && studentId ? `arrival-data-${studentId}` : null,
    async () => fetchArrivalData(studentId),
    { revalidateOnFocus: false },
  );
  const [statusDayRange, setStatusDayRange] =
    useState<StatusDayRange>(getStatusDayRange);
  const { data: statusDays = [], mutate: mutateStatusDays } = useSWRAuth(
    hasFullAccess && studentId
      ? `student-status-days-${studentId}-${statusDayRange.from}-${statusDayRange.to}`
      : null,
    async () =>
      fetchStudentStatusDays(studentId, statusDayRange.from, statusDayRange.to),
    { revalidateOnFocus: false },
  );
  const ensureStatusDayRange = useCallback((from: string, to: string) => {
    setStatusDayRange((current) => extendStatusDayRange(current, [from, to]));
  }, []);
  const loadPlannedStatusExistingDays = useCallback(
    (from: string, to: string) => fetchStudentStatusDays(studentId, from, to),
    [studentId],
  );
  // Reason attached to today's sick day (set by staff or by a parent via the
  // portal), shown next to the absence badge in the header.
  const currentSickReason = useMemo(() => {
    if (!student?.sick) return undefined;
    const now = new Date();
    const todayIso = `${now.getFullYear()}-${`${now.getMonth() + 1}`.padStart(2, "0")}-${`${now.getDate()}`.padStart(2, "0")}`;
    const row = statusDays.find(
      (s) =>
        s.status === "sick" && !s.cleared_at && s.date === todayIso && s.note,
    );
    return row?.note ?? undefined;
  }, [student?.sick, statusDays]);
  // Load active groups when check-in modal opens
  useEffect(() => {
    if (!showConfirmCheckin) {
      // Reset state when modal closes
      setSelectedActiveGroupId("");
      return;
    }

    const loadActiveGroups = async () => {
      setLoadingActiveGroups(true);
      try {
        const groups = await activeService.getActiveGroups({ active: true });
        // Filter to only groups with rooms
        const groupsWithRooms = groups.filter((g) => g.room?.name);
        setActiveGroups(groupsWithRooms);
      } catch (err) {
        logger.error("failed to load active groups", {
          error: err instanceof Error ? err.message : String(err),
        });
        setActiveGroups([]);
      } finally {
        setLoadingActiveGroups(false);
      }
    };

    void loadActiveGroups();
  }, [showConfirmCheckin]);

  // Today's pickup slot for the header. Mirrors todayArrival below: a failed
  // fetch (e.g. permission denied for non-full-access users) leaves pickupData
  // undefined, which renders the same empty header as "no pickup planned".
  const todayPickup = useMemo<{
    time?: string;
    note?: string;
    isException?: boolean;
  }>(() => {
    if (!hasFullAccess || !pickupData) return {};

    const dayData = getDayData(
      new Date(),
      pickupData.schedules,
      pickupData.exceptions,
      student?.sick ?? false,
      pickupData.notes,
      student?.excused ?? false,
    );

    if (dayData.effectiveTime) {
      return {
        time: formatPickupTime(dayData.effectiveTime),
        note: dayData.effectiveNotes,
        isException: dayData.isException,
      };
    }
    return {};
  }, [pickupData, hasFullAccess, student?.sick, student?.excused]);

  const todayArrival = useMemo<TodayArrival>(() => {
    if (!hasFullAccess || !arrivalData) return {};

    const dayData = getArrivalDayData(
      new Date(),
      arrivalData.schedules,
      arrivalData.exceptions,
      arrivalData.notes,
      student?.sick ?? false,
      student?.excused ?? false,
    );

    if (dayData.isAbsent) {
      return {
        note: dayData.effectiveReason,
        isException: dayData.isException,
        isAbsent: true,
      };
    }
    if (dayData.effectiveTime) {
      return {
        time: formatArrivalTime(dayData.effectiveTime),
        note: dayData.effectiveReason,
        isException: dayData.isException,
        isAbsent: false,
      };
    }
    return {};
  }, [arrivalData, hasFullAccess, student?.excused, student?.sick]);

  // Clamp the URL tab to the set the current access level actually exposes, so a
  // stale deep-link (e.g. ?tab=betreuungszeiten without full access) falls back
  // to the default tab instead of showing an empty panel. Computed BEFORE the
  // loading/error early returns below so the self-heal effect is called on every
  // render (Rules of Hooks) — see the load gate inside the effect.
  const activeTab = resolveActiveTab(
    searchParams.get("tab"),
    tabResolutionTabs,
  );

  // If the URL pins a tab we can't honour — an inaccessible deep-link or an
  // unknown value — resolveActiveTab clamps it but the address bar would keep
  // advertising the bogus tab. Rewrite it to match the tab actually shown.
  // handleTabChange drops the param entirely when we land back on the default,
  // so a clamped link self-heals to a clean URL. Guard on a non-null param so
  // we never touch already-clean URLs, which keeps this from looping. Skip
  // while data is still loading: hasFullAccess defaults to false then, so acting
  // early would wrongly strip a valid full-access deep-link before it resolves.
  const urlTab = searchParams.get("tab");
  useEffect(() => {
    if (loading || !student || sessionStatus === "loading") return;
    if (urlTab !== null && urlTab !== activeTab) {
      handleTabChange(activeTab);
    }
  }, [loading, student, sessionStatus, urlTab, activeTab, handleTabChange]);

  // Show loading state
  if (loading) {
    return <StudentDetailSkeleton />;
  }

  // Show error state
  if (error || !student) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <Alert type="error" message={error ?? "Kind nicht gefunden"} />
        <button
          type="button"
          onClick={() => router.push(referrer)}
          className="bg-moto-blue/10 text-moto-blue-strong hover:bg-moto-blue/20 mt-4 rounded px-4 py-2 transition-colors"
        >
          Zurück
        </button>
      </div>
    );
  }

  // =============================================================================
  // EVENT HANDLERS
  // =============================================================================

  const handleSavePersonal = async (editedStudent: ExtendedStudent) => {
    const allowedDepartureModes = normalizeAllowedDepartureModes(
      editedStudent.allowed_departure_modes ??
        allowedDepartureModesFromDeparture(
          editedStudent.departure_days ??
            departureDaysFromLegacy(
              editedStudent.bus_days,
              editedStudent.pickup_days,
            ),
        ),
    );
    await studentService.updateStudent(studentId, {
      first_name: editedStudent.first_name,
      second_name: editedStudent.second_name,
      school_class: editedStudent.school_class,
      birthday: editedStudent.birthday,
      address_street: editedStudent.address_street,
      address_postal_code: editedStudent.address_postal_code,
      address_city: editedStudent.address_city,
      bus_days: normalizeBusDays(editedStudent.bus_days),
      allowed_departure_modes: allowedDepartureModes,
      departure_days: allowedDepartureToDepartureDays(allowedDepartureModes),
      departure_companion_note: editedStudent.departure_companion_note,
      // Laufgemeinschaft travels with the plan it belongs to: a link is only
      // legal on a day that allows "Anderes Kind", so the backend validates
      // both in one request instead of racing two.
      //
      // Omitted when the modal has no loaded list: the field REPLACES the
      // stored links, so sending [] for "not loaded" would delete them. The
      // backend leaves the links alone when the key is absent.
      //
      ...(editedStudent.companions
        ? {
            companions: editedStudent.companions.map((companion) => ({
              companion_student_id: companion.companion_student_id,
              weekdays: companion.weekdays,
            })),
          }
        : {}),
      // The fingerprint travels on its own, not only with a list: it names the
      // snapshot this save was built on, and the plan above removes links too
      // (the backend trims the weekdays it no longer allows). Forwarding it
      // regardless is what lets the backend refuse a save whose stale plan would
      // delete links someone else committed in the meantime.
      companions_fingerprint: editedStudent.companions_fingerprint,
      extend_companion_plans: editedStudent.extend_companion_plans ?? false,
      // The confirmation is only worth as much as what it names: the backend
      // widens a companion's plan only for these children and weekdays.
      confirmed_companion_extensions:
        editedStudent.confirmed_companion_extensions ?? [],
      health_info: editedStudent.health_info,
      supervisor_notes: editedStudent.supervisor_notes,
      extra_info: editedStudent.extra_info,
      pickup_status: editedStudent.pickup_status,
      pickup_days: editedStudent.pickup_days,
    });

    await refreshDataAndHistory();
    toast.success("Persönliche Informationen erfolgreich aktualisiert");
  };

  const handleConfirmCheckout = async () => {
    if (!student) return;

    setCheckingOut(true);
    try {
      // Use dedicated checkout endpoint which:
      // 1. Ends current visit (if any)
      // 2. Toggles attendance to checked_out (daily checkout)
      await activeService.checkoutStudent(studentId);
      refreshData();
      setShowConfirmCheckout(false);
      toast.success(`${student.name} wurde erfolgreich abgemeldet`);
    } catch (err) {
      logger.error("failed to checkout student", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Abmelden des Kindes");
    } finally {
      setCheckingOut(false);
    }
  };

  const handleConfirmCheckin = async () => {
    if (!student || !selectedActiveGroupId) return;

    setCheckingIn(true);
    try {
      await performImmediateCheckin(
        Number.parseInt(studentId, 10),
        Number.parseInt(selectedActiveGroupId, 10),
      );
      refreshData();
      setShowConfirmCheckin(false);
      toast.success(`${student.name} wurde erfolgreich angemeldet`);
    } catch (err) {
      logger.error("failed to check in student", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Anmelden des Kindes");
    } finally {
      setCheckingIn(false);
    }
  };

  const handleConfirmSickToggle = async () => {
    if (!student) return;

    setSickLoading(true);
    try {
      const newSickStatus = !(student.sick ?? false);
      const trimmedReason = sickReason.trim();
      await studentService.updateStudent(studentId, {
        sick: newSickStatus,
        // Only send a reason when marking sick; clearing carries none.
        ...(newSickStatus && trimmedReason
          ? { sick_reason: trimmedReason }
          : {}),
      });
      refreshData();
      await mutateStatusDays();
      setShowConfirmSick(false);
      setSickReason("");
      toast.success(
        newSickStatus
          ? `${student.name} wurde krankgemeldet`
          : `Krankmeldung für ${student.name} wurde aufgehoben`,
      );
    } catch (err) {
      logger.error("sick_status_toggle_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Ändern des Krankheitsstatus");
    } finally {
      setSickLoading(false);
    }
  };

  const handleConfirmExcusedToggle = async () => {
    if (!student) return;

    setExcusedLoading(true);
    try {
      const newExcusedStatus = !isQuickExcused;
      await studentService.updateStudent(studentId, {
        excused: newExcusedStatus,
      });
      refreshData();
      await mutateStatusDays();
      setShowConfirmExcused(false);
      toast.success(
        newExcusedStatus
          ? `${student.name} wurde als entschuldigt markiert`
          : `Entschuldigung für ${student.name} wurde aufgehoben`,
      );
    } catch (err) {
      logger.error("excused_status_toggle_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Ändern des Entschuldigungsstatus");
    } finally {
      setExcusedLoading(false);
    }
  };

  // Click interceptor for the Krank button. If the student is currently
  // excused, we must first clear the excused flag and show the switch dialog
  // instead of the normal confirm modal.
  const handleSickClick = () => {
    if (student?.sick) {
      setShowConfirmSick(true);
      return;
    }
    if (student?.excused) {
      setSwitchTarget("sick");
      return;
    }
    setPlannedStatusModal("sick");
  };

  const handleExcusedClick = () => {
    if (isQuickExcused) {
      setShowConfirmExcused(true);
      return;
    }
    if (student?.sick) {
      setSwitchTarget("excused");
      return;
    }
    setPlannedStatusModal("excused");
  };

  const handleConfirmSwitch = async () => {
    if (!student || !switchTarget) return;

    setSwitchLoading(true);
    try {
      // Send both flags in one request so the backend's mutual-exclusion guard
      // sees only the final state (one true, the other explicitly false).
      await studentService.updateStudent(studentId, {
        sick: switchTarget === "sick",
        excused: switchTarget === "excused",
      });
      refreshData();
      await mutateStatusDays();
      toast.success(
        switchTarget === "sick"
          ? `${student.name} wurde krankgemeldet (Entschuldigung aufgehoben)`
          : `${student.name} wurde entschuldigt (Krankmeldung aufgehoben)`,
      );
      setSwitchTarget(null);
    } catch (err) {
      logger.error("status_switch_failed", {
        student_id: studentId,
        target: switchTarget,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Wechseln des Status");
    } finally {
      setSwitchLoading(false);
    }
  };

  const handleCreatePlannedStatus = async (
    dates: string[],
    reason?: string,
  ) => {
    if (!plannedStatusModal || !student) return;

    setPlannedStatusLoading(true);
    try {
      const createdStatusDays = await createStudentStatusDays(
        studentId,
        plannedStatusModal,
        dates,
        reason,
      );
      refreshData();
      setStatusDayRange((current) => extendStatusDayRange(current, dates));
      await mutateStatusDays(
        (current) => mergeStatusDays(current, createdStatusDays),
        { revalidate: true },
      );
      const statusLabel =
        plannedStatusModal === "sick"
          ? "Krankmeldung"
          : plannedStatusModal === "class_trip"
            ? "Klassenfahrt"
            : "Entschuldigung";
      toast.success(`${statusLabel} für ${student.name} wurde gespeichert`);
      setPlannedStatusModal(null);
    } catch (err) {
      logger.error("planned_status_create_failed", {
        student_id: studentId,
        status: plannedStatusModal,
        error: err instanceof Error ? err.message : String(err),
      });
      if (err instanceof StudentStatusDayConflictError) {
        const conflicts = err.conflicts
          .map(
            (day) =>
              `${formatCalendarDate(day.date)} (${day.label.toLowerCase()})`,
          )
          .join(", ");
        const wasOrWere = err.conflicts.length === 1 ? "wurde" : "wurden";
        toast.warning(
          `${conflicts} ${wasOrWere} zwischenzeitlich eingetragen und nicht überschrieben. Bitte Auswahl prüfen.`,
        );
      } else {
        toast.error("Geplanter Status konnte nicht gespeichert werden");
      }
      throw err;
    } finally {
      setPlannedStatusLoading(false);
    }
  };

  const handleDeletePlannedStatus = async (statusDayId: string) => {
    if (!student) return;

    setDeletingPlannedStatusDayId(statusDayId);
    try {
      await deleteStudentStatusDay(studentId, statusDayId);
      refreshData();
      await mutateStatusDays();
      toast.success("Geplante Abwesenheit wurde entfernt");
    } catch (err) {
      logger.error("planned_status_delete_failed", {
        student_id: studentId,
        status_day_id: statusDayId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Geplanter Status konnte nicht entfernt werden");
    } finally {
      setDeletingPlannedStatusDayId(null);
    }
  };

  // =============================================================================
  // COMPUTED VALUES
  // =============================================================================

  // Determine what action is available based on access (group membership / room supervision)
  const studentActionType = getStudentActionType(
    { group_id: student.group_id, current_location: student.current_location },
    myGroups,
    mySupervisedRooms,
  );
  const showCheckout = studentActionType === "checkout";
  const showCheckin = studentActionType === "checkin";

  // =============================================================================
  // RENDER HELPERS
  // =============================================================================

  const renderRoomSelector = () => {
    if (loadingActiveGroups) {
      return (
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <svg className="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          Räume werden geladen...
        </div>
      );
    }

    if (activeGroups.length === 0) {
      return (
        <p className="text-moto-amber-strong text-sm">
          Keine aktiven Räume verfügbar. Bitte starten Sie zuerst eine aktive
          Aufsicht in einem Raum über ein NFC-Tablet.
        </p>
      );
    }

    return (
      <CustomSelect
        id="room-select"
        ariaLabelledBy="room-select-label"
        value={selectedActiveGroupId}
        options={[
          // Selectable like the old native <option value="">, so an already
          // chosen room can be cleared again before confirming.
          { value: "", label: "Bitte Raum auswählen..." },
          ...activeGroups.map((group) => ({
            value: group.id,
            label: `${group.room?.name ?? "Unbekannter Raum"} (${group.actualGroup?.name ?? "Gruppe"})`,
          })),
        ]}
        onChange={setSelectedActiveGroupId}
      />
    );
  };

  // =============================================================================
  // RENDER
  // =============================================================================

  return (
    <>
      <div className="mx-auto max-w-7xl">
        <BackButton referrer={referrer} />

        <StudentDetailHeader
          student={student}
          myGroups={myGroups}
          myGroupRooms={myGroupRooms}
          mySupervisedRooms={mySupervisedRooms}
          todayPickupPlannedTime={todayPickup.time}
          todayPickupActualTime={student.actual_pickup_time}
          todayPickupNote={todayPickup.note}
          isPickupException={todayPickup.isException}
          todayArrivalPlannedTime={todayArrival.time}
          todayArrivalActualTime={student.actual_arrival_time}
          isArrivalException={todayArrival.isException}
          todayArrivalNote={todayArrival.note}
          isArrivalAbsent={todayArrival.isAbsent}
          sickReason={currentSickReason}
        />

        {hasFullAccess ? (
          <FullAccessView
            student={student}
            studentId={studentId}
            hasWriteAccess={hasWriteAccess}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            activeTab={activeTab}
            tabs={visibleTabs}
            canViewEnrollments={canViewEnrollments}
            canViewCarePlan={canViewCarePlan}
            onTabChange={handleTabChange}
            statusDays={statusDays}
            onDeleteStatusDay={handleDeletePlannedStatus}
            onVisibleDateRangeChange={ensureStatusDayRange}
            showPersonalInfoModal={showPersonalInfoModal}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
            onOpenPersonalInfoModal={() => setShowPersonalInfoModal(true)}
            onClosePersonalInfoModal={() => setShowPersonalInfoModal(false)}
            onSavePersonal={handleSavePersonal}
            onRefreshData={refreshDataAndHistory}
            onSickClick={handleSickClick}
            sickLoading={sickLoading}
            isQuickExcused={isQuickExcused}
            onExcusedClick={handleExcusedClick}
            excusedLoading={excusedLoading}
            onClassTripClick={() => setPlannedStatusModal("class_trip")}
            plannedStatusLoading={plannedStatusLoading}
          />
        ) : (
          <LimitedAccessView
            student={student}
            studentId={studentId}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            supervisors={supervisors}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            activeTab={activeTab}
            tabs={visibleTabs}
            canViewEnrollments={canViewEnrollments}
            onTabChange={handleTabChange}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
          />
        )}
      </div>

      {/* Checkout Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmCheckout}
        onClose={() => setShowConfirmCheckout(false)}
        onConfirm={handleConfirmCheckout}
        title="Kind abmelden"
        confirmText={checkingOut ? "Wird abgemeldet..." : "Geht nach Hause"}
        cancelText="Abbrechen"
        isConfirmLoading={checkingOut}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          Möchten Sie <strong>{student.name}</strong> jetzt abmelden?
        </p>
      </ConfirmationModal>

      {/* Checkin Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmCheckin}
        onClose={() => setShowConfirmCheckin(false)}
        onConfirm={handleConfirmCheckin}
        title="Kind anmelden"
        confirmText={checkingIn ? "Wird angemeldet..." : "Anmelden"}
        cancelText="Abbrechen"
        isConfirmLoading={checkingIn}
        isConfirmDisabled={!selectedActiveGroupId}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <div className="space-y-4">
          <p>
            Möchten Sie <strong>{student.name}</strong> jetzt anmelden?
          </p>
          <div>
            <label
              id="room-select-label"
              htmlFor="room-select"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Raum auswählen
            </label>
            {renderRoomSelector()}
          </div>
        </div>
      </ConfirmationModal>

      {/* Sick Report Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmSick}
        onClose={() => {
          setShowConfirmSick(false);
          setSickReason("");
        }}
        onConfirm={handleConfirmSickToggle}
        title={student.sick ? "Krankmeldung aufheben" : "Kind krankmelden"}
        confirmText={sickConfirmText}
        cancelText="Abbrechen"
        isConfirmLoading={sickLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {student.sick ? (
            <>
              Möchten Sie die Krankmeldung für <strong>{student.name}</strong>{" "}
              für heute aufheben? Geplante Kranktage in der Zukunft bleiben
              bestehen.
            </>
          ) : (
            <>
              Möchten Sie <strong>{student.name}</strong> als krank melden?
            </>
          )}
        </p>
        {!student.sick && (
          <div className="mt-4">
            <label
              htmlFor="sick-reason"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Grund (optional)
            </label>
            <textarea
              id="sick-reason"
              value={sickReason}
              onChange={(e) => setSickReason(e.target.value)}
              rows={2}
              maxLength={2000}
              placeholder="z. B. Fieber, beim Arzt"
              className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-500 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
            />
          </div>
        )}
      </ConfirmationModal>

      {/* Excused Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmExcused}
        onClose={() => setShowConfirmExcused(false)}
        onConfirm={handleConfirmExcusedToggle}
        title={
          isQuickExcused ? "Entschuldigung aufheben" : "Kind entschuldigen"
        }
        confirmText={excusedConfirmText}
        cancelText="Abbrechen"
        isConfirmLoading={excusedLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {isQuickExcused ? (
            <>
              Möchten Sie die Entschuldigung für <strong>{student.name}</strong>{" "}
              für heute aufheben? Geplante Entschuldigungen in der Zukunft
              bleiben bestehen.
            </>
          ) : (
            <>
              Möchten Sie <strong>{student.name}</strong> als entschuldigt
              markieren?
            </>
          )}
        </p>
      </ConfirmationModal>

      {/* Switch Dialog, shown when user clicks one flag but the other is set */}
      <ConfirmationModal
        isOpen={switchTarget !== null}
        onClose={() => setSwitchTarget(null)}
        onConfirm={handleConfirmSwitch}
        title={
          switchTarget === "sick"
            ? "Als krank melden?"
            : "Als entschuldigt markieren?"
        }
        confirmText={switchLoading ? "Wird gewechselt..." : "Status wechseln"}
        cancelText="Abbrechen"
        isConfirmLoading={switchLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {switchTarget === "sick" ? (
            <>
              <strong>{student.name}</strong> ist aktuell als entschuldigt
              markiert. Stattdessen als krank melden? Die Entschuldigung wird
              dabei aufgehoben.
            </>
          ) : (
            <>
              <strong>{student.name}</strong> ist aktuell als krank gemeldet.
              Stattdessen als entschuldigt markieren? Die Krankmeldung wird
              dabei aufgehoben.
            </>
          )}
        </p>
      </ConfirmationModal>

      <PlannedStatusDaysModal
        isOpen={plannedStatusModal !== null}
        status={plannedStatusModal ?? "sick"}
        studentName={student.name}
        isSubmitting={plannedStatusLoading}
        existingDays={statusDays}
        deletingStatusDayId={deletingPlannedStatusDayId}
        onClose={() => setPlannedStatusModal(null)}
        loadExistingDays={loadPlannedStatusExistingDays}
        onSubmit={handleCreatePlannedStatus}
        onDeleteStatusDay={handleDeletePlannedStatus}
      />
    </>
  );
}

// =============================================================================
// LIMITED ACCESS VIEW
// =============================================================================

interface LimitedAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  supervisors: SupervisorContact[];
  showCheckout: boolean;
  showCheckin: boolean;
  activeTab: StudentTabId;
  tabs: StudentTabId[];
  canViewEnrollments: boolean;
  onTabChange: (tab: string) => void;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
}

function LimitedAccessView({
  student,
  studentId,
  attendanceLogEnabled,
  feedbackEnabled,
  supervisors,
  showCheckout,
  showCheckin,
  activeTab,
  tabs,
  canViewEnrollments,
  onTabChange,
  onCheckoutClick,
  onCheckinClick,
}: Readonly<LimitedAccessViewProps>) {
  const historyRouter = useTenantRouter();
  return (
    <>
      {(showCheckout || showCheckin) && (
        <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={onTabChange}>
        <StudentTabsList tabs={tabs} />

        <TabsContent
          value="stammdaten"
          forceMount
          className={`${TAB_CONTENT_CLASS} space-y-4 sm:space-y-6`}
        >
          <SupervisorsCard
            supervisors={supervisors}
            studentName={student.name}
          />
          <PersonalInfoReadOnly student={student} />
        </TabsContent>

        <TabsContent
          value="erziehungsberechtigte"
          forceMount
          className={TAB_CONTENT_CLASS}
        >
          <StudentGuardianManager studentId={student.id} readOnly={true} />
        </TabsContent>

        {canViewEnrollments ? (
          <TabsContent
            value="anmeldungen"
            forceMount
            className={TAB_CONTENT_CLASS}
          >
            <StudentEnrollmentsTab studentId={student.id} />
          </TabsContent>
        ) : null}

        <TabsContent value="historie" forceMount className={TAB_CONTENT_CLASS}>
          <StudentHistorySection
            studentId={studentId}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            readOnly={true}
            onNavigate={(path) => historyRouter.push(path)}
          />
        </TabsContent>
      </Tabs>
    </>
  );
}

// =============================================================================
// FULL ACCESS VIEW
// =============================================================================

interface FullAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  hasWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  showCheckout: boolean;
  showCheckin: boolean;
  activeTab: StudentTabId;
  tabs: StudentTabId[];
  canViewEnrollments: boolean;
  canViewCarePlan: boolean;
  onTabChange: (tab: string) => void;
  statusDays: StudentStatusDay[];
  onDeleteStatusDay: (statusDayId: string) => Promise<void>;
  onVisibleDateRangeChange: (from: string, to: string) => void;
  showPersonalInfoModal: boolean;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
  onOpenPersonalInfoModal: () => void;
  onClosePersonalInfoModal: () => void;
  onSavePersonal: (student: ExtendedStudent) => Promise<void>;
  onRefreshData: () => void;
  onSickClick: () => void;
  sickLoading: boolean;
  isQuickExcused: boolean;
  onExcusedClick: () => void;
  excusedLoading: boolean;
  onClassTripClick: () => void;
  plannedStatusLoading: boolean;
}

function FullAccessView({
  student,
  studentId,
  hasWriteAccess,
  attendanceLogEnabled,
  feedbackEnabled,
  showCheckout,
  showCheckin,
  activeTab,
  tabs,
  canViewEnrollments,
  canViewCarePlan,
  onTabChange,
  statusDays,
  onDeleteStatusDay,
  onVisibleDateRangeChange,
  showPersonalInfoModal,
  onCheckoutClick,
  onCheckinClick,
  onOpenPersonalInfoModal,
  onClosePersonalInfoModal,
  onSavePersonal,
  onRefreshData,
  onSickClick,
  sickLoading,
  isQuickExcused,
  onExcusedClick,
  excusedLoading,
  onClassTripClick,
  plannedStatusLoading,
}: Readonly<FullAccessViewProps>) {
  const historyRouter = useTenantRouter();
  const { groups: enrollmentExtraGroups } = useStudentEnrollmentExtraFields(
    studentId,
    true,
  );
  // Lazy-mount the Nachrichten tab: ParentMessagesCard runs the inbox-projection
  // query (two correlated COUNT subqueries) on mount, and forceMount would fire
  // it for every student-detail load even when staff never open the tab — paging
  // 40 profiles = 40 such queries. Defer until the tab is first opened, then keep
  // it mounted (forceMount) so revisits don't refetch.
  const [messagesTabSeen, setMessagesTabSeen] = useState(
    activeTab === "nachrichten",
  );
  useEffect(() => {
    if (activeTab === "nachrichten") setMessagesTabSeen(true);
  }, [activeTab]);
  // Same deal for Dokumente (#777): the list request also sweeps for storage
  // cleanup left over from an interrupted upload, so firing it on every
  // student-detail load would put that work behind page views that never open
  // the tab. Mount on first open, then keep it mounted so revisits don't refetch.
  const [documentsTabSeen, setDocumentsTabSeen] = useState(
    activeTab === "dokumente",
  );
  useEffect(() => {
    if (activeTab === "dokumente") setDocumentsTabSeen(true);
  }, [activeTab]);
  return (
    <>
      {(showCheckout || showCheckin || hasWriteAccess) && (
        <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
          {hasWriteAccess && (
            <StudentSickReportSection
              isSick={student.sick ?? false}
              sickSince={student.sick_since}
              onToggle={onSickClick}
              isLoading={sickLoading}
            />
          )}
          {hasWriteAccess && (
            <StudentExcusedReportSection
              isExcused={isQuickExcused}
              excusedSince={isQuickExcused ? student.excused_since : undefined}
              onToggle={onExcusedClick}
              isLoading={excusedLoading}
            />
          )}
          {hasWriteAccess && (
            <StudentStatusActionsMenu
              isClassTrip={student.class_trip ?? false}
              classTripSince={student.class_trip_since}
              onPlanClassTrip={onClassTripClick}
              isLoading={plannedStatusLoading}
            />
          )}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={onTabChange}>
        <StudentTabsList tabs={tabs} />

        <TabsContent
          value="stammdaten"
          forceMount
          className={TAB_CONTENT_CLASS}
        >
          <PersonalInfoReadOnly
            student={student}
            enrollmentExtraGroups={enrollmentExtraGroups}
            showEditButton={hasWriteAccess}
            onEditClick={hasWriteAccess ? onOpenPersonalInfoModal : undefined}
          />
        </TabsContent>

        <TabsContent
          value="nachrichten"
          forceMount
          className={TAB_CONTENT_CLASS}
        >
          {studentId && messagesTabSeen && (
            <ParentMessagesCard
              studentId={studentId}
              studentName={student?.name}
            />
          )}
        </TabsContent>

        <TabsContent
          value="erziehungsberechtigte"
          forceMount
          className={TAB_CONTENT_CLASS}
        >
          <StudentGuardianManager
            studentId={studentId}
            readOnly={!hasWriteAccess}
            onUpdate={hasWriteAccess ? onRefreshData : undefined}
          />
        </TabsContent>

        {canViewCarePlan ? (
          <TabsContent
            value="betreuungsplan"
            forceMount
            className={TAB_CONTENT_CLASS}
          >
            {/* forceMounted for deep-linking; `active` gates the SWR read so it
                only fires when the tab is actually open. Only rendered with
                schedules:read, so the timetable fetch never 403s. */}
            <CarePlanView
              studentId={studentId}
              statusDays={statusDays}
              isSick={student.sick}
              isExcused={student.excused}
              onEditSchedule={() => onTabChange("betreuungszeiten")}
              onVisibleDateRangeChange={onVisibleDateRangeChange}
              active={activeTab === "betreuungsplan"}
            />
          </TabsContent>
        ) : null}

        <TabsContent
          value="betreuungszeiten"
          forceMount
          className={TAB_CONTENT_CLASS}
        >
          <CareScheduleManager
            studentId={studentId}
            readOnly={!hasWriteAccess}
            onUpdate={hasWriteAccess ? onRefreshData : undefined}
            isSick={student.sick}
            isExcused={student.excused}
            statusDays={statusDays}
            onDeleteStatusDay={onDeleteStatusDay}
            onVisibleDateRangeChange={onVisibleDateRangeChange}
          />
        </TabsContent>

        {canViewEnrollments ? (
          <TabsContent
            value="anmeldungen"
            forceMount
            className={TAB_CONTENT_CLASS}
          >
            <StudentEnrollmentsTab studentId={studentId} />
          </TabsContent>
        ) : null}

        <TabsContent value="dokumente" forceMount className={TAB_CONTENT_CLASS}>
          {documentsTabSeen && <StudentDokumenteTab studentId={studentId} />}
        </TabsContent>

        <TabsContent value="historie" forceMount className={TAB_CONTENT_CLASS}>
          <StudentHistorySection
            studentId={studentId}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            canViewChangeHistory={hasWriteAccess}
            onNavigate={(path) => historyRouter.push(path)}
          />
        </TabsContent>
      </Tabs>

      {hasWriteAccess && (
        <PersonalInfoFormModal
          isOpen={showPersonalInfoModal}
          onClose={onClosePersonalInfoModal}
          student={student}
          onSave={onSavePersonal}
        />
      )}
    </>
  );
}
