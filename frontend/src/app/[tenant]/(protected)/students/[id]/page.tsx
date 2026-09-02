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
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import { BackButton } from "~/components/ui/back-button";
import { studentService } from "~/lib/api";
import { schoolCheckinStudent } from "~/lib/student-api";
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
  AggregatedRequestList,
  type AggregatedRequestFilters,
} from "~/components/students/aggregated-request-list";
import { FamilyProtectionControl } from "~/components/students/family-protection-control";
import { StudentConsentsReadOnly } from "~/components/students/student-consents-read-only";
import { SectionCard } from "~/components/ui/section-card";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  StudentSickReportSection,
  StudentExcusedReportSection,
  StudentStatusActionsMenu,
} from "~/components/students/student-checkout-section";
import { createLogger } from "~/lib/logger";
import { canReviewCareWithdrawals } from "~/lib/change-request-access";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";
import { CarePlanView } from "~/components/students/care-plan-view";
import { CareScheduleManager } from "~/components/students/care-schedule-manager";
import { CareExitModal } from "~/components/students/care-exit-modal";
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
  StudentStatusDayPartialAbsenceConflictError,
  type StudentStatusDay,
  type StudentStatusKind,
} from "~/lib/student-status-days-api";
import { formatDate as formatCalendarDate } from "~/lib/date-helpers";
import {
  fetchStudentCareWithdrawal,
  type CareWithdrawalCompletion,
} from "~/lib/care-exit-api";
import { fetchStudentCarePlanDay } from "~/lib/student-care-plan-api";
import {
  deleteStudentPartialAbsence,
  fetchStudentPartialAbsences,
  saveStudentPartialAbsence,
} from "~/lib/student-partial-absences-api";
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
  | "aenderungsprotokoll"
  | "historie";

const TAB_LABELS: Record<StudentTabId, string> = {
  stammdaten: "Stammdaten",
  nachrichten: "Nachrichten",
  erziehungsberechtigte: "Erziehungsberechtigte",
  betreuungsplan: "Betreuungsplan",
  betreuungszeiten: "Betreuungszeiten",
  anmeldungen: "Anmeldungen",
  dokumente: "Dokumente",
  aenderungsprotokoll: "Änderungsprotokoll",
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
  "aenderungsprotokoll",
  "historie",
];
const LIMITED_ACCESS_BASE_TABS: StudentTabId[] = [
  "stammdaten",
  "erziehungsberechtigte",
  "aenderungsprotokoll",
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
  "aenderungsprotokoll",
  "historie",
];
const LIMITED_ACCESS_TABS_WITH_ENROLLMENTS: StudentTabId[] = [
  "stammdaten",
  "erziehungsberechtigte",
  "anmeldungen",
  "aenderungsprotokoll",
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
 *   `has_full_access` on the student response: true for admins and every
 *   verified staff member (#2329), false for guest/guardian accounts. See
 *   api/students/authorization.go — `checkStudentReadAccess` (this flag) vs
 *   `checkStudentFullAccess` (the write flag).
 */
function studentTabs(
  hasStudentReadAccess: boolean,
  canViewEnrollments: boolean,
  canViewCarePlan: boolean,
  canViewDocuments: boolean,
  canViewRequestLog: boolean,
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
  // so the two can never disagree. Every verified staff member lands in the
  // full-access set and DOES get the tab (#2329); a caller without read access
  // (guest/guardian) gets 403 from the care-plan endpoints, so widening the
  // tab would only surface a permanently failing panel, and
  // ?tab=betreuungsplan is clamped away for the same reason.
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
  const withDocuments = canViewDocuments
    ? withCarePlan
    : withCarePlan.filter((tab) => tab !== "dokumente");
  // Änderungsprotokoll (#2437) mirrors the aggregate route's gate exactly:
  // RequiresAnyPermission(users:update, users:absence). Without either, the
  // list would only answer 403, so the tab stays away.
  return canViewRequestLog
    ? withDocuments
    : withDocuments.filter((tab) => tab !== "aenderungsprotokoll");
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
    hasAbsenceWriteAccess,
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
  // Same gate the aggregate request route uses (#2437).
  const canViewRequestLog =
    sessionStatus === "authenticated" &&
    (hasPermission(session, "users:update") ||
      hasPermission(session, "users:absence"));
  // Mirrors the backend gate on POST /students/{id}/school-checkin exactly.
  const canCheckin =
    sessionStatus === "authenticated" &&
    hasPermission(session, "users:checkin");
  const canCompleteCareWithdrawal =
    sessionStatus === "authenticated" && canReviewCareWithdrawals(session);
  const [careWithdrawal, setCareWithdrawal] =
    useState<CareWithdrawalCompletion | null>(null);
  const [careWithdrawalLoadFailed, setCareWithdrawalLoadFailed] =
    useState(false);
  const [careWithdrawalModalOpen, setCareWithdrawalModalOpen] = useState(false);
  const visibleTabs = useMemo(
    () =>
      studentTabs(
        hasFullAccess,
        canViewEnrollments,
        canViewCarePlan,
        canViewDocuments,
        canViewRequestLog,
      ),
    [
      canViewEnrollments,
      canViewCarePlan,
      hasFullAccess,
      canViewDocuments,
      canViewRequestLog,
    ],
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
  // Status days follow the absence gate, not the read gate: whoever may write
  // a child's absences may read them (the backend widened the per-child check
  // on GET /{id}/status-days for exactly this, #2232). Without this the
  // planning dialog opened empty for a school without feste Gruppen — existing
  // sick days stayed invisible and could not be cleared.
  const canReadStatusDays = hasFullAccess || hasAbsenceWriteAccess;
  // A partial excusal ("Ab Uhrzeit") is a pickup exception, not a status day:
  // its endpoints require users:update at the route AND full care access to the
  // child in the handler. The absence permission grants neither, so the scope
  // switch stays hidden for an absence-only staffer instead of offering a save
  // that would fail (#2232).
  const canPlanPartialExcusal =
    hasFullAccess &&
    hasWriteAccess &&
    sessionStatus === "authenticated" &&
    hasPermission(session, "users:update");
  const { data: statusDays = [], mutate: mutateStatusDays } = useSWRAuth(
    canReadStatusDays && studentId
      ? `student-status-days-${studentId}-${statusDayRange.from}-${statusDayRange.to}`
      : null,
    async () =>
      fetchStudentStatusDays(studentId, statusDayRange.from, statusDayRange.to),
    { revalidateOnFocus: false },
  );
  const { data: partialAbsences = [], mutate: mutatePartialAbsences } =
    useSWRAuth(
      hasFullAccess && studentId
        ? `student-partial-absences-${studentId}-${statusDayRange.from}-${statusDayRange.to}`
        : null,
      async () =>
        fetchStudentPartialAbsences(
          studentId,
          statusDayRange.from,
          statusDayRange.to,
        ),
      { revalidateOnFocus: false },
    );
  const ensureStatusDayRange = useCallback((from: string, to: string) => {
    setStatusDayRange((current) => extendStatusDayRange(current, [from, to]));
  }, []);
  const loadPlannedStatusExistingDays = useCallback(
    (from: string, to: string) => {
      ensureStatusDayRange(from, to);
      return fetchStudentStatusDays(studentId, from, to);
    },
    [ensureStatusDayRange, studentId],
  );
  const loadPlannedPartialAbsences = useCallback(
    (from: string, to: string) =>
      fetchStudentPartialAbsences(studentId, from, to),
    [studentId],
  );
  const loadPlannedCarePlanDay = useCallback(
    (date: string) => fetchStudentCarePlanDay(studentId, date),
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
    if (!canCompleteCareWithdrawal || !studentId) {
      setCareWithdrawal(null);
      setCareWithdrawalLoadFailed(false);
      return;
    }
    let cancelled = false;
    const load = () => {
      void fetchStudentCareWithdrawal(studentId)
        .then((result) => {
          if (cancelled) return;
          setCareWithdrawal(result);
          setCareWithdrawalLoadFailed(false);
        })
        .catch((error: unknown) => {
          if (cancelled) return;
          logger.warn("care_withdrawal_warning_load_failed", {
            student_id: studentId,
            error: error instanceof Error ? error.message : String(error),
          });
          setCareWithdrawal(null);
          setCareWithdrawalLoadFailed(true);
        });
    };
    load();
    window.addEventListener("change-requests-refresh", load);
    return () => {
      cancelled = true;
      window.removeEventListener("change-requests-refresh", load);
    };
  }, [canCompleteCareWithdrawal, studentId]);

  useEffect(() => {
    if (loading || !student || sessionStatus === "loading") return;
    if (urlTab !== null && urlTab !== activeTab) {
      handleTabChange(activeTab);
    }
  }, [loading, student, sessionStatus, urlTab, activeTab, handleTabChange]);

  // Show loading state. `referrer` needs no fetched data (it's derived from
  // the `?from=` query param above), so the BackButton renders for real here
  // instead of a placeholder — see page-skeleton.tsx. The rest of the body
  // stays skeletonized: `visibleTabs` and the FullAccessView/LimitedAccessView
  // split both depend on `hasFullAccess`, a field on the student object that
  // isn't known until this fetch resolves.
  if (loading) {
    return <StudentDetailSkeleton referrer={referrer} />;
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
      await schoolCheckinStudent(studentId, "out");
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
    if (!student) return;

    setCheckingIn(true);
    try {
      await schoolCheckinStudent(studentId, "in");
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
      if (err instanceof StudentStatusDayPartialAbsenceConflictError) {
        toast.warning(err.message);
      } else if (err instanceof StudentStatusDayConflictError) {
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
      // Re-throw so the modal does not clear local conflict state for a
      // delete that never landed on the server.
      throw err;
    } finally {
      setDeletingPlannedStatusDayId(null);
    }
  };

  const handleSavePartialAbsence = async (
    partialAbsenceId: string | null,
    date: string,
    fromTime: string,
    reason?: string,
  ) => {
    if (!student) return;
    setPlannedStatusLoading(true);
    try {
      await saveStudentPartialAbsence(
        studentId,
        partialAbsenceId,
        date,
        fromTime,
        reason,
      );
      setStatusDayRange((current) => extendStatusDayRange(current, [date]));
      await Promise.all([
        mutatePartialAbsences(),
        mutate(`pickup-data-${studentId}`),
      ]);
      refreshData();
      toast.success(
        partialAbsenceId
          ? `Entschuldigung für ${student.name} wurde aktualisiert`
          : `Entschuldigung für ${student.name} wurde gespeichert`,
      );
      setPlannedStatusModal(null);
    } catch (err) {
      logger.error("partial_absence_save_failed", {
        student_id: studentId,
        partial_absence_id: partialAbsenceId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Entschuldigung konnte nicht gespeichert werden");
      throw err;
    } finally {
      setPlannedStatusLoading(false);
    }
  };

  const handleDeletePartialAbsence = async (partialAbsenceId: string) => {
    setPlannedStatusLoading(true);
    try {
      await deleteStudentPartialAbsence(studentId, partialAbsenceId);
      await Promise.all([
        mutatePartialAbsences(),
        mutate(`pickup-data-${studentId}`),
      ]);
      refreshData();
      toast.success("Teilentschuldigung wurde entfernt");
    } catch (err) {
      logger.error("partial_absence_delete_failed", {
        student_id: studentId,
        partial_absence_id: partialAbsenceId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Teilentschuldigung konnte nicht entfernt werden");
      throw err;
    } finally {
      setPlannedStatusLoading(false);
    }
  };

  // =============================================================================
  // COMPUTED VALUES
  // =============================================================================

  const isAtHome =
    !student.current_location || student.current_location.startsWith("Zuhause");
  const showCheckout = canCheckin && !isAtHome;
  const showCheckin = canCheckin && isAtHome;

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

        {careWithdrawalLoadFailed ? (
          <div className="mt-4">
            <Alert
              type="error"
              message="Die offene Abmeldung konnte nicht geladen werden."
            />
          </div>
        ) : careWithdrawal ? (
          <div className="mt-4">
            <Alert
              type="warning"
              message={`Abmeldung noch abschließen. Ab ${formatCalendarDate(careWithdrawal.firstBookinglessDay)} ist kein Betreuungstag mehr gebucht.`}
              action={
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  onClick={() => setCareWithdrawalModalOpen(true)}
                >
                  Betreuung beenden
                </Button>
              }
            />
            <CareExitModal
              isOpen={careWithdrawalModalOpen}
              studentIds={[studentId]}
              completionId={careWithdrawal.id}
              firstBookinglessDay={careWithdrawal.firstBookinglessDay}
              onClose={() => setCareWithdrawalModalOpen(false)}
              onFinished={() => {
                setCareWithdrawalModalOpen(false);
                setCareWithdrawal(null);
                window.dispatchEvent(new Event("change-requests-refresh"));
              }}
            />
          </div>
        ) : null}

        {hasFullAccess ? (
          <FullAccessView
            student={student}
            studentId={studentId}
            hasWriteAccess={hasWriteAccess}
            hasAbsenceWriteAccess={hasAbsenceWriteAccess}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            activeTab={activeTab}
            tabs={visibleTabs}
            canViewEnrollments={canViewEnrollments}
            canViewCarePlan={canViewCarePlan}
            canManageFamilyProtection={canViewEnrollments}
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
            hasAbsenceWriteAccess={hasAbsenceWriteAccess}
            activeTab={activeTab}
            tabs={visibleTabs}
            canViewEnrollments={canViewEnrollments}
            onTabChange={handleTabChange}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
            onSickClick={handleSickClick}
            sickLoading={sickLoading}
            isQuickExcused={isQuickExcused}
            onExcusedClick={handleExcusedClick}
            excusedLoading={excusedLoading}
            onClassTripClick={() => setPlannedStatusModal("class_trip")}
            plannedStatusLoading={plannedStatusLoading}
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
      >
        <p>
          Möchten Sie <strong>{student.name}</strong> jetzt anmelden?
        </p>
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
        existingPartialAbsences={partialAbsences}
        canPlanPartialExcusal={canPlanPartialExcusal}
        deletingStatusDayId={deletingPlannedStatusDayId}
        onClose={() => setPlannedStatusModal(null)}
        loadExistingDays={loadPlannedStatusExistingDays}
        loadPartialAbsences={loadPlannedPartialAbsences}
        loadCarePlanDay={loadPlannedCarePlanDay}
        onSubmit={handleCreatePlannedStatus}
        onDeleteStatusDay={handleDeletePlannedStatus}
        onSubmitPartialAbsence={handleSavePartialAbsence}
        onDeletePartialAbsence={handleDeletePartialAbsence}
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
  /** Absence actions are gated separately from read access: a school without
   *  feste Gruppen lets staff report absences for children whose record they
   *  may only see redacted (#2232). */
  hasAbsenceWriteAccess: boolean;
  activeTab: StudentTabId;
  tabs: StudentTabId[];
  canViewEnrollments: boolean;
  onTabChange: (tab: string) => void;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
  onSickClick: () => void;
  sickLoading: boolean;
  isQuickExcused: boolean;
  onExcusedClick: () => void;
  excusedLoading: boolean;
  onClassTripClick: () => void;
  plannedStatusLoading: boolean;
}

function LimitedAccessView({
  student,
  studentId,
  attendanceLogEnabled,
  feedbackEnabled,
  supervisors,
  showCheckout,
  showCheckin,
  hasAbsenceWriteAccess,
  activeTab,
  tabs,
  canViewEnrollments,
  onTabChange,
  onCheckoutClick,
  onCheckinClick,
  onSickClick,
  sickLoading,
  isQuickExcused,
  onExcusedClick,
  excusedLoading,
  onClassTripClick,
  plannedStatusLoading,
}: Readonly<LimitedAccessViewProps>) {
  const historyRouter = useTenantRouter();
  const changeProtocolFilters = useMemo<AggregatedRequestFilters>(
    () => ({
      search: "",
      studentId,
      includeEnrollment: false,
      includeCareWithdrawals: true,
      types: [],
      statuses: [],
    }),
    [studentId],
  );
  // Siehe FullAccessView: das Änderungsprotokoll lädt erst beim ersten Öffnen.
  const [protocolTabSeen, setProtocolTabSeen] = useState(
    activeTab === "aenderungsprotokoll",
  );
  useEffect(() => {
    if (activeTab === "aenderungsprotokoll") setProtocolTabSeen(true);
  }, [activeTab]);
  return (
    <>
      {(showCheckout || showCheckin || hasAbsenceWriteAccess) && (
        <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
          {hasAbsenceWriteAccess && (
            <StudentSickReportSection
              isSick={student.sick ?? false}
              sickSince={student.sick_since}
              onToggle={onSickClick}
              isLoading={sickLoading}
            />
          )}
          {hasAbsenceWriteAccess && (
            <StudentExcusedReportSection
              isExcused={isQuickExcused}
              excusedSince={isQuickExcused ? student.excused_since : undefined}
              onToggle={onExcusedClick}
              isLoading={excusedLoading}
            />
          )}
          {hasAbsenceWriteAccess && (
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

        {tabs.includes("aenderungsprotokoll") && (
          <TabsContent
            value="aenderungsprotokoll"
            forceMount
            className={TAB_CONTENT_CLASS}
          >
            {protocolTabSeen && (
              <SectionCard
                kicker="Kinderkartei"
                title="Änderungsprotokoll"
                description="Was sich an Buchungen, Betreuungszeiten, Stammdaten und Abwesenheiten dieses Kindes geändert hat"
              >
                <AggregatedRequestList
                  view="history"
                  filters={changeProtocolFilters}
                />
              </SectionCard>
            )}
          </TabsContent>
        )}

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
  /** See LimitedAccessViewProps.hasAbsenceWriteAccess (#2232). */
  hasAbsenceWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  showCheckout: boolean;
  showCheckin: boolean;
  activeTab: StudentTabId;
  tabs: StudentTabId[];
  canViewEnrollments: boolean;
  canViewCarePlan: boolean;
  canManageFamilyProtection: boolean;
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
  hasAbsenceWriteAccess,
  attendanceLogEnabled,
  feedbackEnabled,
  showCheckout,
  showCheckin,
  activeTab,
  tabs,
  canViewEnrollments,
  canViewCarePlan,
  canManageFamilyProtection,
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
  const changeProtocolFilters = useMemo<AggregatedRequestFilters>(
    () => ({
      search: "",
      studentId,
      includeEnrollment: false,
      includeCareWithdrawals: true,
      types: [],
      statuses: [],
    }),
    [studentId],
  );
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
  // Das Änderungsprotokoll fragt fünf Historien-Quellen ab (#2437) — erst beim
  // ersten Öffnen laden, danach gemountet lassen.
  const [protocolTabSeen, setProtocolTabSeen] = useState(
    activeTab === "aenderungsprotokoll",
  );
  useEffect(() => {
    if (activeTab === "aenderungsprotokoll") setProtocolTabSeen(true);
  }, [activeTab]);
  return (
    <>
      {(showCheckout || showCheckin || hasAbsenceWriteAccess) && (
        <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
          {hasAbsenceWriteAccess && (
            <StudentSickReportSection
              isSick={student.sick ?? false}
              sickSince={student.sick_since}
              onToggle={onSickClick}
              isLoading={sickLoading}
            />
          )}
          {hasAbsenceWriteAccess && (
            <StudentExcusedReportSection
              isExcused={isQuickExcused}
              excusedSince={isQuickExcused ? student.excused_since : undefined}
              onToggle={onExcusedClick}
              isLoading={excusedLoading}
            />
          )}
          {hasAbsenceWriteAccess && (
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
          <div className="space-y-4">
            <PersonalInfoReadOnly
              student={student}
              enrollmentExtraGroups={enrollmentExtraGroups}
              showEditButton={hasWriteAccess}
              onEditClick={hasWriteAccess ? onOpenPersonalInfoModal : undefined}
            />
            <StudentConsentsReadOnly consents={student.consents} />
            {canManageFamilyProtection ? (
              <SectionCard
                kicker="Private Angaben"
                title="Familienschutz"
                description="Verhindert, dass Eltern Anfragen zu diesem Kind miteinander teilen."
              >
                <FamilyProtectionControl studentId={studentId} canManage />
              </SectionCard>
            ) : null}
          </div>
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

        {tabs.includes("aenderungsprotokoll") && (
          <TabsContent
            value="aenderungsprotokoll"
            forceMount
            className={TAB_CONTENT_CLASS}
          >
            {protocolTabSeen && (
              <SectionCard
                kicker="Kinderkartei"
                title="Änderungsprotokoll"
                description="Was sich an Buchungen, Betreuungszeiten, Stammdaten und Abwesenheiten dieses Kindes geändert hat"
              >
                <AggregatedRequestList
                  view="history"
                  filters={changeProtocolFilters}
                />
              </SectionCard>
            )}
          </TabsContent>
        )}

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
