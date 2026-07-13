"use client";

/**
 * Betreuungsplan view (ehemals /timetables): admin weekly planner. Hosted as
 * a tab of /planung (#1886).
 *
 * Planner surface for calendar periods, series, materialized instances,
 * one-off appointments, lifecycle actions, and plan-quality checks.
 */

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { CalendarOff } from "lucide-react";

import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { PageHeader } from "~/components/ui/page-header/PageHeader";
import { useToast } from "~/contexts/ToastContext";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { ConflictWarningsBanner } from "~/components/timetable/conflict-warnings-banner";
import {
  InstanceDetailSlideOver,
  type LifecycleAction,
} from "~/components/timetable/instance-detail-slide-over";
import { TimetableAddMenu } from "~/components/timetable/timetable-add-menu";
import { MonthPlannerGrid } from "~/components/timetable/month-planner-grid";
import { PeriodSwitcherDropdown } from "~/components/timetable/period-switcher-dropdown";
import { PlanQualityPanel } from "~/components/timetable/plan-quality-panel";
import { TimetableOverview } from "~/components/timetable/timetable-overview";
import { TimetableSetupGuide } from "~/components/timetable/timetable-setup-guide";
import { TemplateList } from "~/components/timetable/template-list";
import { TimetableEventModal } from "~/components/timetable/timetable-event-modal";
import { hasPermission } from "~/lib/auth-utils";
import {
  DENSITY_TO_HOUR_HEIGHT_PX,
  TimetableToolbar,
  type TimetableView,
  type WeekDensity,
} from "~/components/timetable/timetable-toolbar";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { YearPlannerGrid } from "~/components/timetable/year-planner-grid";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
} from "~/lib/settings-api";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import { listPhases } from "~/lib/enrollment-phase-api";
import { listCareOfferings } from "~/lib/care-offering-api";
import { formatDate } from "~/lib/date-helpers";
import {
  findPeriodForDate,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
} from "~/lib/calendar-period-helpers";
import { timetableService } from "~/lib/timetable-api";
import {
  chunkDateRange,
  countPlanned,
  countUnderstaffedInstances,
  countUnderstaffedTemplates,
  formatWeekLabel,
  formatMonthLabel,
  formatYearLabel,
  getMonthDays,
  getMonthRange,
  getWeekRange,
  getWeekdays,
  getYearMonths,
  getYearRange,
  resolveTemplateCalendarPeriodId,
  toISODate,
  type TimetableCareOfferingLinkStatus,
} from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  MaterializeWarning,
  TimetableTemplate,
  WeeklyInstancesResponse,
} from "~/lib/timetable-types";
import {
  TimetableContentSkeleton,
  TimetablePageSkeleton,
} from "./betreuungsplan-skeleton";

const logger = createLogger({ component: "TimetablesPage" });
const PERIODS_SWR_KEY = "database-calendar-periods-list";

interface CareOfferingLinkSummary {
  total: number;
  linked: number;
}

/**
 * Load the optional enrollment linkage summary used by the setup guide.
 * A partial result would be actively misleading, so any failed phase request
 * makes the whole summary unknown.
 */
export async function loadCareOfferingLinkSummary(): Promise<CareOfferingLinkSummary | null> {
  try {
    const phases = await listPhases();
    const activePhaseIds = new Set(
      phases.filter((phase) => phase.is_active).map((phase) => phase.id),
    );
    const offeringLists = await Promise.all(
      [...activePhaseIds].map((phaseId) => listCareOfferings(phaseId)),
    );
    const offerings = offeringLists
      .flat()
      .filter(
        (offering) =>
          offering.is_active && activePhaseIds.has(offering.phase_id),
      );

    return {
      total: offerings.length,
      linked: offerings.filter((offering) => offering.activity_group_id != null)
        .length,
    };
  } catch (err) {
    logger.warn("timetable_care_offering_link_load_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    return null;
  }
}

function plannedPeriodLabel(
  view: TimetableView,
  isSeriesView: boolean,
): string {
  if (isSeriesView) return "als Regeltermin";
  if (view === "week") return "diese Woche";
  if (view === "month") return "diesen Monat";
  return "dieses Jahr";
}

function careOfferingLinkPresentation(summary: CareOfferingLinkSummary | null) {
  if (summary === null) {
    return {
      status: "unknown" as const,
      label: null,
    };
  }

  return {
    status: summary.linked > 0 ? ("linked" as const) : ("unlinked" as const),
    label:
      summary.total === 0
        ? "Noch keine Angebote"
        : `${summary.linked} von ${summary.total} Angeboten verknüpft`,
  };
}

function calendarPeriodUsage(period: CalendarPeriod | null) {
  if (!period) return undefined;
  return {
    enrollmentPhaseCount: period.enrollmentPhaseCount ?? 0,
    activityGroupCount: period.activityGroupCount ?? 0,
    scheduleCount: period.scheduleCount ?? 0,
    studentEnrollmentCount: period.studentEnrollmentCount ?? 0,
    supervisorCount: period.supervisorCount ?? 0,
    activityInstanceCount: period.activityInstanceCount ?? 0,
  };
}

function parseWeekOffset(raw: string | null): number {
  if (raw === null) return 0;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : 0;
}

function parseView(raw: string | null): TimetableView {
  if (raw === "week" || raw === "year" || raw === "series") return raw;
  if (raw === "templates") return "series";
  return "month";
}

function parseMonth(raw: string | null): Date {
  if (raw && /^\d{4}-\d{2}$/.test(raw)) {
    const [year, month] = raw.split("-").map(Number);
    return new Date(year!, month! - 1, 1);
  }
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), 1);
}

function monthParam(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function parseYear(raw: string | null, month: Date): Date {
  if (raw && /^\d{4}$/.test(raw)) {
    return new Date(Number(raw), 0, 1);
  }
  return new Date(month.getFullYear(), 0, 1);
}

function yearParam(date: Date): string {
  return String(date.getFullYear());
}

function weekOffsetForDate(dateISO: string): number {
  const current = getWeekRange(new Date(), 0).from;
  const target = getWeekRange(new Date(`${dateISO}T00:00:00`), 0).from;
  return Math.round((target.getTime() - current.getTime()) / 604800000);
}

function firstVisibleSchoolDateInPeriod(
  period: CalendarPeriod,
  targetISO: string,
): string {
  const periodStart = new Date(`${period.startDate}T00:00:00`);
  const periodEnd = new Date(`${period.endDate}T00:00:00`);
  const target = new Date(`${targetISO}T00:00:00`);
  const day = target.getDay();

  if (day === 6) {
    const nextMonday = new Date(target);
    nextMonday.setDate(target.getDate() + 2);
    if (nextMonday <= periodEnd) return toISODate(nextMonday);

    const previousFriday = new Date(target);
    previousFriday.setDate(target.getDate() - 1);
    if (previousFriday >= periodStart) return toISODate(previousFriday);
  }

  if (day === 0) {
    const nextMonday = new Date(target);
    nextMonday.setDate(target.getDate() + 1);
    if (nextMonday <= periodEnd) return toISODate(nextMonday);

    const previousFriday = new Date(target);
    previousFriday.setDate(target.getDate() - 2);
    if (previousFriday >= periodStart) return toISODate(previousFriday);
  }

  return targetISO;
}

function schoolYearPeriodDefaults(anchor: Date): {
  name: string;
  startDate: string;
  endDate: string;
} {
  const anchorYear = anchor.getFullYear();
  const startsInCurrentYear = anchor.getMonth() >= 7;
  const startYear = startsInCurrentYear ? anchorYear : anchorYear - 1;
  const endYear = startYear + 1;

  return {
    name: `Schuljahr ${startYear}/${endYear}`,
    startDate: `${startYear}-08-01`,
    endDate: `${endYear}-07-31`,
  };
}

/**
 * Rendered when the tenant setting `timetable.enabled` resolves to false.
 * The sidebar already hides the nav entry for disabled tenants; this guard
 * covers direct navigation without redirecting (no redirect loops).
 */
function TimetableDisabledState() {
  return (
    <div className="flex flex-col gap-4" data-testid="timetable-disabled-state">
      <PageHeader title="Betreuungsplan" />
      <div className={`${timetableSurface} p-10 text-center`}>
        <CalendarOff className="mx-auto h-10 w-10 text-gray-300" aria-hidden />
        <h2 className="mt-4 text-base font-semibold text-gray-900">
          Betreuungsplan ist deaktiviert
        </h2>
        <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-600">
          Der Betreuungsplan ist für diese Schule ausgeschaltet. Er kann in den
          Einstellungen unter „Betrieb“ wieder aktiviert werden.
        </p>
      </div>
    </div>
  );
}

function TimetablesContent() {
  const searchParams = useSearchParams();
  const { data: session, status } = useSession({ required: true });
  const canCheckShiftCoverage =
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "users:read");
  const toast = useToast();
  const tenantMutate = useTenantMutate();

  const [view, setView] = useState<TimetableView>(() =>
    parseView(searchParams.get("view")),
  );
  const [weekOffset, setWeekOffset] = useState(() =>
    parseWeekOffset(searchParams.get("week")),
  );
  const [monthDate, setMonthDate] = useState(() =>
    parseMonth(searchParams.get("month")),
  );
  const [yearDate, setYearDate] = useState(() =>
    parseYear(searchParams.get("year"), parseMonth(searchParams.get("month"))),
  );
  const [selectedInstanceId, setSelectedInstanceId] = useState<string | null>(
    () => searchParams.get("instance"),
  );
  const [selectedDay, setSelectedDay] = useState<string | null>(() =>
    searchParams.get("day"),
  );
  const [selectedPeriodId, setSelectedPeriodId] = useState<string | null>(() =>
    searchParams.get("period"),
  );
  const [eventModalOpen, setEventModalOpen] = useState(false);
  const [eventDefaultRepeat, setEventDefaultRepeat] = useState<
    "none" | "weekly"
  >("none");
  // US-1 quick create: set when the modal was opened from an empty week
  // slot. Non-null switches the event modal into the "quick" variant with
  // prefilled date and times; cleared when the modal closes.
  const [quickPrefill, setQuickPrefill] = useState<{
    date: string;
    startTime: string;
    endTime: string;
  } | null>(null);
  const [editingInstance, setEditingInstance] =
    useState<EnrichedInstance | null>(null);
  const [editingTemplate, setEditingTemplate] =
    useState<TimetableTemplate | null>(null);
  const [archivingTemplate, setArchivingTemplate] =
    useState<TimetableTemplate | null>(null);
  const [archiveLoading, setArchiveLoading] = useState(false);
  const [convertingInstance, setConvertingInstance] =
    useState<EnrichedInstance | null>(null);
  const [periodModalOpen, setPeriodModalOpen] = useState(false);
  // null = create mode (no active period yet); otherwise edit the named period.
  const [editingPeriod, setEditingPeriod] = useState<CalendarPeriod | null>(
    null,
  );
  // "Schuljahre & Ferien" in the toolbar overflow menu forces the period
  // switcher pill visible even when it would otherwise be hidden (single
  // fully-covering period), so list + edit stays reachable.
  const [periodManagementVisible, setPeriodManagementVisible] = useState(false);
  // StrictMode double-invokes effects; the ref keeps the (idempotent)
  // bootstrap POST from firing twice per mount.
  const bootstrapAttemptedRef = useRef(false);
  // Density picker for the week grid (Kompakt/Normal/Komfortabel maps to
  // 60/90/120 px per hour). Local-only, not synced to URL because density
  // is a cosmetic preference, and we never expose pixel values in the UI.
  const [density, setDensity] = useState<WeekDensity>("normal");
  const hourHeightPx = DENSITY_TO_HOUR_HEIGHT_PX[density];
  const { dayStartHour, dayEndHour } = useTimetableDayHours();

  const openPeriodCreate = useCallback(() => {
    setEditingPeriod(null);
    setPeriodModalOpen(true);
  }, []);
  const openPeriodEdit = useCallback((period: CalendarPeriod) => {
    setEditingPeriod(period);
    setPeriodModalOpen(true);
  }, []);

  const updateUrlParams = useCallback(
    (patch: Record<string, string | null>) => {
      const next =
        typeof window === "undefined"
          ? new URLSearchParams(searchParams.toString())
          : new URLSearchParams(window.location.search);
      for (const [key, value] of Object.entries(patch)) {
        if (value === null || value === "") {
          next.delete(key);
        } else {
          next.set(key, value);
        }
      }
      const qs = next.toString();
      setView(parseView(next.get("view")));
      setWeekOffset(parseWeekOffset(next.get("week")));
      const parsedMonth = parseMonth(next.get("month"));
      setMonthDate(parsedMonth);
      setYearDate(parseYear(next.get("year"), parsedMonth));
      setSelectedInstanceId(next.get("instance"));
      setSelectedDay(next.get("day"));
      setSelectedPeriodId(next.get("period"));

      if (typeof window !== "undefined") {
        window.history.replaceState(
          null,
          "",
          `${window.location.pathname}${qs ? `?${qs}` : ""}`,
        );
      }
    },
    [searchParams],
  );

  useEffect(() => {
    const syncFromLocation = () => {
      const next = new URLSearchParams(window.location.search);
      setView(parseView(next.get("view")));
      setWeekOffset(parseWeekOffset(next.get("week")));
      const parsedMonth = parseMonth(next.get("month"));
      setMonthDate(parsedMonth);
      setYearDate(parseYear(next.get("year"), parsedMonth));
      setSelectedInstanceId(next.get("instance"));
      setSelectedDay(next.get("day"));
      setSelectedPeriodId(next.get("period"));
    };
    window.addEventListener("popstate", syncFromLocation);
    return () => window.removeEventListener("popstate", syncFromLocation);
  }, []);

  const handleWeekChange = useCallback(
    (newOffset: number) => {
      updateUrlParams({
        week: newOffset === 0 ? null : String(newOffset),
        // changing weeks closes the slide-over to avoid stale selection
        instance: null,
        day: null,
      });
    },
    [updateUrlParams],
  );

  const handleViewChange = useCallback(
    (nextView: TimetableView) => {
      const visibleDate =
        view === "week"
          ? new Date(
              `${selectedDay ?? toISODate(getWeekRange(new Date(), weekOffset).from)}T00:00:00`,
            )
          : view === "month"
            ? monthDate
            : view === "year"
              ? yearDate
              : new Date();
      const viewPatch: Record<string, string | null> = {
        view: nextView === "month" ? null : nextView,
        instance: null,
        day: null,
      };
      if (nextView === "year") {
        viewPatch.year = yearParam(visibleDate);
      }
      if (nextView === "month") {
        viewPatch.month = monthParam(visibleDate);
      }
      updateUrlParams({
        ...viewPatch,
      });
    },
    [monthDate, selectedDay, updateUrlParams, view, weekOffset, yearDate],
  );

  const handleMonthChange = useCallback(
    (delta: number) => {
      const next = new Date(monthDate);
      next.setMonth(next.getMonth() + delta);
      updateUrlParams({
        month: monthParam(next),
        instance: null,
      });
    },
    [monthDate, updateUrlParams],
  );

  const handleYearChange = useCallback(
    (delta: number) => {
      const next = new Date(yearDate);
      next.setFullYear(next.getFullYear() + delta);
      updateUrlParams({
        year: yearParam(next),
        instance: null,
      });
    },
    [yearDate, updateUrlParams],
  );

  const openWeekForDay = useCallback(
    (dateISO: string) => {
      const offset = weekOffsetForDate(dateISO);
      updateUrlParams({
        view: "week",
        week: offset === 0 ? null : String(offset),
        day: dateISO,
        instance: null,
      });
    },
    [updateUrlParams],
  );

  const openMonthFromYear = useCallback(
    (month: Date) => {
      updateUrlParams({
        view: null,
        month: monthParam(month),
        instance: null,
        day: null,
      });
    },
    [updateUrlParams],
  );

  const jumpToPeriod = useCallback(
    (period: CalendarPeriod) => {
      const todayISOlocal = toISODate(new Date());
      const target =
        period.startDate > todayISOlocal ? period.startDate : todayISOlocal;
      const targetISO =
        target < period.startDate
          ? period.startDate
          : target > period.endDate
            ? period.endDate
            : target;
      const visibleDateISO = firstVisibleSchoolDateInPeriod(period, targetISO);
      const visibleDate = new Date(`${visibleDateISO}T00:00:00`);

      if (view === "series") {
        updateUrlParams({
          period: period.id,
          instance: null,
        });
        return;
      }

      if (view === "month") {
        updateUrlParams({
          month: monthParam(visibleDate),
          period: period.id,
          instance: null,
          day: null,
        });
        return;
      }

      if (view === "year") {
        updateUrlParams({
          year: yearParam(visibleDate),
          period: period.id,
          instance: null,
          day: null,
        });
        return;
      }

      const offset = weekOffsetForDate(visibleDateISO);
      updateUrlParams({
        week: offset === 0 ? null : String(offset),
        period: period.id,
        day: visibleDateISO,
        instance: null,
      });
    },
    [updateUrlParams, view],
  );

  const handleToolbarPrev = useCallback(() => {
    if (view === "week") handleWeekChange(weekOffset - 1);
    else if (view === "month") handleMonthChange(-1);
    else if (view === "year") handleYearChange(-1);
  }, [view, weekOffset, handleWeekChange, handleMonthChange, handleYearChange]);
  const handleToolbarNext = useCallback(() => {
    if (view === "week") handleWeekChange(weekOffset + 1);
    else if (view === "month") handleMonthChange(1);
    else if (view === "year") handleYearChange(1);
  }, [view, weekOffset, handleWeekChange, handleMonthChange, handleYearChange]);
  const handleToolbarToday = useCallback(() => {
    if (view === "week") {
      handleWeekChange(0);
    } else if (view === "month") {
      const now = new Date();
      updateUrlParams({
        month: monthParam(new Date(now.getFullYear(), now.getMonth(), 1)),
        instance: null,
      });
    } else if (view === "year") {
      const now = new Date();
      updateUrlParams({
        year: yearParam(new Date(now.getFullYear(), 0, 1)),
        instance: null,
      });
    }
  }, [view, handleWeekChange, updateUrlParams]);

  const handleSelectInstance = useCallback(
    (instance: EnrichedInstance | null) => {
      updateUrlParams({ instance: instance?.id ?? null });
    },
    [updateUrlParams],
  );

  // Week range. useMemo prevents re-derivation on every render, the SWR
  // key depends on these strings.
  const weekRange = useMemo(
    () =>
      selectedDay
        ? getWeekRange(new Date(`${selectedDay}T00:00:00`), 0)
        : getWeekRange(new Date(), weekOffset),
    [selectedDay, weekOffset],
  );
  const fromISO = toISODate(weekRange.from);
  const toISO = toISODate(weekRange.to);
  const monthRange = useMemo(() => getMonthRange(monthDate), [monthDate]);
  const monthDays = useMemo(() => getMonthDays(monthDate), [monthDate]);
  const yearRange = useMemo(() => getYearRange(yearDate), [yearDate]);
  const yearMonths = useMemo(() => getYearMonths(yearDate), [yearDate]);
  const yearFetchFromISO = toISODate(yearRange.from);
  const yearFetchToISO = toISODate(yearRange.to);
  const fetchFromISO =
    view === "month"
      ? toISODate(monthRange.from)
      : view === "year"
        ? yearFetchFromISO
        : fromISO;
  const fetchToISO =
    view === "month"
      ? toISODate(monthRange.to)
      : view === "year"
        ? yearFetchToISO
        : toISO;
  const weekDays = useMemo(() => getWeekdays(weekRange.from), [weekRange.from]);
  const weekDayISOs = useMemo(() => weekDays.map(toISODate), [weekDays]);
  const periodContextDays =
    view === "month" ? monthDays : view === "year" ? yearMonths : weekDays;
  const periodContextDayISOs = useMemo(
    () => periodContextDays.map(toISODate),
    [periodContextDays],
  );
  const todayISO = useMemo(() => toISODate(new Date()), []);
  const weekLabel = useMemo(
    () => formatWeekLabel(weekRange.from, weekRange.to),
    [weekRange.from, weekRange.to],
  );
  const monthLabel = useMemo(() => formatMonthLabel(monthDate), [monthDate]);
  const yearLabel = useMemo(() => formatYearLabel(yearDate), [yearDate]);

  const swrKey = `timetable-${view}-${fetchFromISO}-${fetchToISO}`;
  const shouldLoadInstances = view !== "series";
  const qualityFromISO = fromISO < todayISO ? todayISO : fromISO;
  const shouldLoadPlanQuality = view === "week" && toISO >= todayISO;
  const gapsSWRKey = `timetable-gaps-${qualityFromISO}-${toISO}`;
  const exceptionConflictsSWRKey = `timetable-exception-conflicts-${qualityFromISO}-${toISO}`;
  const { data, error, isLoading } = useSWRAuth(
    status === "authenticated" && shouldLoadInstances ? swrKey : null,
    async () => {
      if (view !== "year") {
        return timetableService.getWeek(fetchFromISO, fetchToISO);
      }

      const chunks = chunkDateRange(fetchFromISO, fetchToISO, 56);
      const responses = await Promise.all(
        chunks.map((chunk) => timetableService.getWeek(chunk.from, chunk.to)),
      );
      return responses.reduce<WeeklyInstancesResponse>(
        (merged, response) => ({
          from: merged.from,
          to: response.to,
          instances: [...merged.instances, ...response.instances],
        }),
        { from: fetchFromISO, to: fetchToISO, instances: [] },
      );
    },
  );
  const {
    data: gapsData,
    error: gapsError,
    isLoading: gapsLoading,
  } = useSWRAuth(
    status === "authenticated" && shouldLoadPlanQuality ? gapsSWRKey : null,
    () => timetableService.getGaps(qualityFromISO, toISO),
  );
  const {
    data: exceptionConflictData,
    error: conflictsError,
    isLoading: conflictsLoading,
  } = useSWRAuth(
    status === "authenticated" && shouldLoadPlanQuality
      ? exceptionConflictsSWRKey
      : null,
    () => timetableService.getExceptionConflicts(qualityFromISO, toISO),
  );
  const { data: staffData } = useSWRAuth(
    status === "authenticated" ? "timetable-staff-list" : null,
    () => staffService.getAllStaff(),
  );
  const { data: studentData } = useSWRAuth(
    status === "authenticated" ? "timetable-student-list" : null,
    () => fetchStudents({ page_size: 500 }),
  );
  // The optional "Mit der Anmeldung verknüpfen" setup step is done only when a
  // care offering actually points at a Regeltermin — an active enrollment phase
  // alone proves no linkage (issue #1651). Missing permission or an incomplete
  // per-phase result degrades to the neutral "unknown" status instead of
  // failing the page or displaying a false partial count.
  const { data: careOfferingLink } = useSWRAuth(
    status === "authenticated" ? "timetable-care-offering-link" : null,
    loadCareOfferingLinkSummary,
    { revalidateOnFocus: false },
  );
  const {
    data: periods,
    error: periodsError,
    isLoading: periodsLoading,
  } = useSWRAuth(status === "authenticated" ? PERIODS_SWR_KEY : null, () =>
    calendarPeriodService.list(),
  );
  // H4 route guard: same source the sidebar uses to hide the nav entry
  // (settings schema -> timetable.enabled). fetchSettingsSchema returns
  // null when the user cannot read settings; the page then renders
  // normally (graceful default, mirroring the sidebar).
  const { data: settingsSchema, isLoading: settingsSchemaLoading } = useSWRAuth(
    status === "authenticated" ? SETTINGS_SCHEMA_SWR_KEY : null,
    fetchSettingsSchema,
    { revalidateOnFocus: false, revalidateOnReconnect: false },
  );
  const timetableDisabled =
    settingsSchema?.tabs
      .flatMap((tab) => tab.categories)
      .flatMap((category) => category.items)
      .find((item) => item.key === "timetable.enabled")?.value === false;

  // SWR retries (errorRetryCount=3) produce a fresh Error per attempt. Keying
  // the effect on the message string keeps the toast from firing once per
  // retry when the underlying message is unchanged.
  const errorMessage = error
    ? error instanceof Error
      ? error.message
      : String(error)
    : null;
  const gapsErrorMessage = gapsError
    ? gapsError instanceof Error
      ? gapsError.message
      : String(gapsError)
    : null;
  const conflictsErrorMessage = conflictsError
    ? conflictsError instanceof Error
      ? conflictsError.message
      : String(conflictsError)
    : null;
  const planQualityErrorMessage =
    gapsErrorMessage && conflictsErrorMessage
      ? `Personal-Lücken und Ausnahmen konnten nicht geprüft werden: ${gapsErrorMessage}; ${conflictsErrorMessage}`
      : gapsErrorMessage
        ? `Personal-Lücken konnten nicht geprüft werden: ${gapsErrorMessage}`
        : conflictsErrorMessage
          ? `Ausnahmen konnten nicht geprüft werden: ${conflictsErrorMessage}`
          : null;

  useEffect(() => {
    if (!errorMessage) return;
    logger.error("week_load_failed", { error: errorMessage });
    toast.error(`Betreuungsplan konnte nicht geladen werden: ${errorMessage}`);
  }, [errorMessage, toast]);

  useEffect(() => {
    if (!periodsError) return;
    const message =
      periodsError instanceof Error
        ? periodsError.message
        : String(periodsError);
    logger.error("periods_load_failed", { error: message });
    toast.error(`Planungszeiträume konnten nicht geladen werden: ${message}`);
  }, [periodsError, toast]);

  // Phase 2: silently create the default school-year period when the
  // tenant has none. The backend POST /periods/bootstrap is idempotent;
  // failures are logged but non-fatal (the planner just stays empty).
  useEffect(() => {
    if (periodsLoading || periodsError || periods === undefined) return;
    if (periods.length > 0) return;
    if (settingsSchemaLoading || timetableDisabled) return;
    if (bootstrapAttemptedRef.current) return;
    bootstrapAttemptedRef.current = true;
    void calendarPeriodService
      .bootstrap()
      .then(() => tenantMutate(PERIODS_SWR_KEY))
      .catch((err: unknown) => {
        // 403 (missing SchedulesCreate permission) is expected for
        // non-admin staff opening the planner — warn instead of error.
        const httpStatus =
          typeof err === "object" && err !== null && "httpStatus" in err
            ? (err as { httpStatus?: unknown }).httpStatus
            : undefined;
        const context = {
          error: err instanceof Error ? err.message : String(err),
          ...(typeof httpStatus === "number" ? { status: httpStatus } : {}),
        };
        if (httpStatus === 403) {
          logger.warn("periods_bootstrap_failed", context);
        } else {
          logger.error("periods_bootstrap_failed", context);
        }
      });
  }, [
    periods,
    periodsError,
    periodsLoading,
    settingsSchemaLoading,
    tenantMutate,
    timetableDisabled,
  ]);

  useEffect(() => {
    if (!planQualityErrorMessage) return;
    logger.error("plan_quality_load_failed", {
      error: planQualityErrorMessage,
    });
    toast.error("Planstatus konnte nicht vollständig geladen werden.");
  }, [planQualityErrorMessage, toast]);

  // Memoise on data?.instances so the reference is stable between renders
  // when SWR returns the same response (the linter warns when arrays are
  // derived inline because `?? []` produces a new array each render).
  const instances = useMemo(() => data?.instances ?? [], [data?.instances]);
  const gaps = useMemo(() => gapsData?.gaps ?? [], [gapsData?.gaps]);
  const exceptionConflicts = useMemo(
    () => exceptionConflictData?.conflicts ?? [],
    [exceptionConflictData?.conflicts],
  );
  const staff = useMemo(() => staffData ?? [], [staffData]);
  const students = useMemo(
    () => studentData?.students ?? [],
    [studentData?.students],
  );
  const staffNames = useMemo(
    () => new Map(staff.map((item) => [item.id, item.name])),
    [staff],
  );
  const studentNames = useMemo(
    () => new Map(students.map((item) => [item.id, item.name])),
    [students],
  );
  const calendarPeriods = useMemo(() => periods ?? [], [periods]);
  const periodAssignments = useMemo(
    () => mapPeriodsForDates(calendarPeriods, periodContextDayISOs),
    [calendarPeriods, periodContextDayISOs],
  );
  const assignedPeriods = useMemo(
    () => uniqueAssignedPeriods(periodAssignments),
    [periodAssignments],
  );
  const weekPeriodAssignments = useMemo(
    () => mapPeriodsForDates(calendarPeriods, weekDayISOs),
    [calendarPeriods, weekDayISOs],
  );
  const weekHasFullPeriodCoverage = useMemo(
    () =>
      weekPeriodAssignments.length > 0 &&
      weekPeriodAssignments.every((assignment) => assignment.period !== null),
    [weekPeriodAssignments],
  );
  const defaultTemplatePeriod = useMemo(
    () =>
      findPeriodForDate(
        calendarPeriods,
        view === "month" ? toISODate(monthDate) : fromISO,
      ),
    [calendarPeriods, fromISO, monthDate, view],
  );
  const templatePeriodID =
    view === "series" && selectedPeriodId
      ? selectedPeriodId
      : (defaultTemplatePeriod?.id ?? assignedPeriods[0]?.id);
  const focusedPeriodID = useMemo(() => {
    if (view === "series") return templatePeriodID ?? null;
    if (
      selectedPeriodId &&
      assignedPeriods.some((period) => period.id === selectedPeriodId)
    ) {
      return selectedPeriodId;
    }
    return defaultTemplatePeriod?.id ?? assignedPeriods[0]?.id ?? null;
  }, [
    assignedPeriods,
    defaultTemplatePeriod,
    selectedPeriodId,
    templatePeriodID,
    view,
  ]);
  const { data: templateData, isLoading: templatesLoading } = useSWRAuth(
    status === "authenticated" && templatePeriodID
      ? `timetable-templates-${templatePeriodID}`
      : null,
    () => timetableService.getTemplates(templatePeriodID),
  );
  const templates = useMemo(
    () => templateData?.templates ?? [],
    [templateData?.templates],
  );
  const modalCalendarPeriods = useMemo(() => {
    const periodIDs = new Set(assignedPeriods.map((period) => period.id));
    if (templatePeriodID) periodIDs.add(templatePeriodID);
    if (editingTemplate) {
      const pinnedPeriodID = resolveTemplateCalendarPeriodId(editingTemplate);
      if (pinnedPeriodID) periodIDs.add(pinnedPeriodID);
    }
    return calendarPeriods.filter((period) => periodIDs.has(period.id));
  }, [assignedPeriods, calendarPeriods, editingTemplate, templatePeriodID]);
  const showTemplatePeriodField = assignedPeriods.length !== 1;
  const periodCreateDefaults = useMemo(() => {
    const defaults = schoolYearPeriodDefaults(weekRange.from);
    return {
      name: defaults.name,
      periodType: "school_year" as const,
      startDate: defaults.startDate,
      endDate: defaults.endDate,
      weekCycleLength: "1",
      weekCycleAnchor: "",
      isActive: true,
    };
  }, [weekRange.from]);
  const conflictCount = useMemo(
    () =>
      instances.reduce((sum, inst) => sum + inst.conflictWarnings.length, 0),
    [instances],
  );

  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );
  const isInstanceDataLoading = shouldLoadInstances && isLoading && !data;

  // --- Overview zone (KPIs + setup guide), rendered in every view ---
  // KPIs are derived from the already-loaded instances/templates so they
  // work in every view. The /api/timetable/gaps endpoint uses a different,
  // zero-coverage rule, so only the Planstatus panel uses it.
  const isSeriesView = view === "series";
  const plannedCount = useMemo(
    () => (isSeriesView ? templates.length : countPlanned(instances)),
    [isSeriesView, templates, instances],
  );
  const understaffedCount = useMemo(
    () =>
      isSeriesView
        ? countUnderstaffedTemplates(templates)
        : countUnderstaffedInstances(instances),
    [isSeriesView, templates, instances],
  );
  const plannedSublabel = plannedPeriodLabel(view, isSeriesView);
  const understaffedSublabel =
    understaffedCount > 0
      ? "zusätzliches Personal nötig"
      : "ausreichend besetzt";
  const hasActivePeriod = useMemo(
    () => calendarPeriods.some((period) => period.isActive),
    [calendarPeriods],
  );
  const activePeriodLabel = useMemo(() => {
    const period =
      findPeriodForDate(calendarPeriods, todayISO) ??
      calendarPeriods.find((item) => item.isActive) ??
      null;
    if (!period) return null;
    return `${period.name} · gültig bis ${formatDate(period.endDate)}`;
  }, [calendarPeriods, todayISO]);
  const hasPlan = instances.length > 0 || templates.length > 0;
  const careOfferingPresentation = careOfferingLinkPresentation(
    careOfferingLink ?? null,
  );
  const careOfferingLinkStatus: TimetableCareOfferingLinkStatus =
    careOfferingPresentation.status;
  const careOfferingLinkLabel = careOfferingPresentation.label;

  const handleLifecycle = useCallback(
    async (action: LifecycleAction) => {
      if (!selectedInstance) return;
      try {
        if (action === "start") {
          const res = await timetableService.start(selectedInstance.id);
          if (res.warnings.length > 0) {
            toast.success(
              `Gestartet: ${res.warnings.length} Hinweis(e): ${res.warnings.map((w) => w.message).join(", ")}`,
            );
          } else {
            toast.success("Aktivität gestartet");
          }
        } else if (action === "complete") {
          await timetableService.complete(selectedInstance.id);
          toast.success("Aktivität beendet");
        } else {
          await timetableService.cancel(selectedInstance.id);
          toast.success("Aktivität abgesagt");
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("lifecycle_action_failed", {
          action,
          instance_id: selectedInstance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Aktion konnte nicht durchgeführt werden",
        );
        throw err;
      }
    },
    [
      selectedInstance,
      swrKey,
      gapsSWRKey,
      exceptionConflictsSWRKey,
      tenantMutate,
      toast,
    ],
  );

  const handleSubstitute = useCallback(
    async (
      instanceId: string,
      absentStaffId: string,
      substituteStaffId: string,
      date: string,
    ) => {
      try {
        // Konsolidierung (#1886): der Gap-Fill nutzt denselben atomaren
        // Deviations-Pfad wie der Vertretungs-Editor; die Zuweisung wirkt
        // weiterhin tagesweit auf alle Blöcke der abwesenden Person.
        const result = await timetableService.applyDeviations(instanceId, {
          substitutions: [{ absentStaffId, substituteStaffId }],
        });
        toast.success(
          `Ersatz eingetragen: ${result.affectedInstances.length} Termin(e) aktualisiert`,
        );
        if (result.warnings.length > 0) {
          toast.error(
            `${result.warnings.length} mögliche Zeitüberschneidung(en) prüfen.`,
          );
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("substitute_failed", {
          instance_id: instanceId,
          absent_staff_id: absentStaffId,
          substitute_staff_id: substituteStaffId,
          date,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Ersatz konnte nicht eingetragen werden",
        );
        throw err;
      }
    },
    [exceptionConflictsSWRKey, gapsSWRKey, swrKey, tenantMutate, toast],
  );

  const handleAttendancePatch = useCallback(
    async (
      instanceId: string,
      studentId: string,
      body: Parameters<typeof timetableService.patchAttendance>[2],
    ) => {
      try {
        await timetableService.patchAttendance(instanceId, studentId, body);
        toast.success("Kinderstatus aktualisiert");
        await tenantMutate(swrKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("attendance_patch_failed", {
          instance_id: instanceId,
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Kinderstatus konnte nicht aktualisiert werden",
        );
        throw err;
      }
    },
    [exceptionConflictsSWRKey, swrKey, tenantMutate, toast],
  );

  const handleDeleteCancelledInstance = useCallback(
    async (instance: EnrichedInstance) => {
      try {
        await timetableService.deleteCancelled(instance.id);
        toast.success("Termin gelöscht");
        updateUrlParams({ instance: null });
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("instance_delete_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Termin konnte nicht gelöscht werden",
        );
        throw err;
      }
    },
    [
      exceptionConflictsSWRKey,
      gapsSWRKey,
      swrKey,
      tenantMutate,
      toast,
      updateUrlParams,
    ],
  );

  const handleDeleteFollowingInstances = useCallback(
    async (instance: EnrichedInstance) => {
      if (!instance.activityGroupId) return;
      try {
        const result = await timetableService.endTemplate(
          instance.activityGroupId,
          {
            effective_date: instance.date,
          },
        );
        toast.success(
          `Regeltermin ab ${formatDate(result.effectiveDate)} beendet`,
        );
        updateUrlParams({ instance: null });
        if (templatePeriodID) {
          await tenantMutate(`timetable-templates-${templatePeriodID}`);
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("template_end_failed", {
          template_id: instance.activityGroupId,
          instance_id: instance.id,
          effective_date: instance.date,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Folgetermine konnten nicht gelöscht werden",
        );
        throw err;
      }
    },
    [
      exceptionConflictsSWRKey,
      gapsSWRKey,
      swrKey,
      templatePeriodID,
      tenantMutate,
      toast,
      updateUrlParams,
    ],
  );

  const handleDeleteSeriesTemplate = useCallback(
    async (template: TimetableTemplate, effectiveDate: string) => {
      try {
        const result = await timetableService.endTemplate(template.id, {
          effective_date: effectiveDate,
        });
        toast.success(
          `Regeltermin ab ${formatDate(result.effectiveDate)} gelöscht`,
        );
        setEventModalOpen(false);
        setEditingInstance(null);
        setEditingTemplate(null);
        setConvertingInstance(null);
        if (templatePeriodID) {
          await tenantMutate(`timetable-templates-${templatePeriodID}`);
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("template_delete_failed", {
          template_id: template.id,
          effective_date: effectiveDate,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Regeltermin konnte nicht gelöscht werden",
        );
        throw err;
      }
    },
    [
      exceptionConflictsSWRKey,
      gapsSWRKey,
      swrKey,
      templatePeriodID,
      tenantMutate,
      toast,
    ],
  );

  const openEventCreate = useCallback(() => {
    setEditingInstance(null);
    setEditingTemplate(null);
    setConvertingInstance(null);
    setEventDefaultRepeat("none");
    setEventModalOpen(true);
  }, []);

  const openSeriesCreate = useCallback(() => {
    setEditingInstance(null);
    setEditingTemplate(null);
    setConvertingInstance(null);
    setEventDefaultRepeat("weekly");
    setEventModalOpen(true);
  }, []);

  // US-1 quick create: clicking an empty hour slot in the week grid opens
  // the event modal in its compact "quick" variant, prefilled with that
  // slot's date and a one-hour window (capped at 23:59).
  const openQuickCreate = useCallback((dateISO: string, hour: number) => {
    const startTime = `${String(hour).padStart(2, "0")}:00`;
    const endTime =
      hour >= 23 ? "23:59" : `${String(hour + 1).padStart(2, "0")}:00`;
    setEditingInstance(null);
    setEditingTemplate(null);
    setConvertingInstance(null);
    setEventDefaultRepeat("none");
    setQuickPrefill({ date: dateISO, startTime, endTime });
    setEventModalOpen(true);
  }, []);

  // The period pill is only chrome when it carries information: several
  // periods exist, or the visible range crosses a period boundary / has
  // uncovered days ("Übergangswoche"). A single fully-covering period is
  // the default and needs no UI; the zero-period state shows nothing at
  // all (the silent bootstrap fills it). "Schuljahre & Ferien" in the
  // toolbar overflow menu forces the pill visible for management.
  const hasUncoveredContextDays = useMemo(
    () => periodAssignments.some((assignment) => assignment.period === null),
    [periodAssignments],
  );
  const showPeriodSwitcher =
    periodManagementVisible ||
    calendarPeriods.length > 1 ||
    (calendarPeriods.length > 0 && hasUncoveredContextDays);

  const handleManagePeriods = useCallback(() => {
    if (calendarPeriods.length === 0) {
      openPeriodCreate();
      return;
    }
    setPeriodManagementVisible(true);
  }, [calendarPeriods.length, openPeriodCreate]);

  const handleApplyTemplate = useCallback(
    async (template: TimetableTemplate) => {
      // Resolve which calendar period the template's schedules belong to.
      // Falls back to the period currently in scope (defaultTemplatePeriod)
      // If none is active, ask the admin to create one before continuing.
      const periodId =
        resolveTemplateCalendarPeriodId(template) ?? defaultTemplatePeriod?.id;
      const period = periodId
        ? calendarPeriods.find((p) => p.id === periodId)
        : null;
      if (!period) {
        toast.warning(
          "Kein aktiver Planungszeitraum. Der Regeltermin kann nicht eingetragen werden.",
        );
        openPeriodCreate();
        return;
      }
      // Backend caps a single materialize call at 56 days. School-year
      // periods are ~365 days, so split into 56-day windows and aggregate.
      const chunks = chunkDateRange(period.startDate, period.endDate, 56);
      try {
        let totalCreated = 0;
        const warningsByCode = new Map<string, MaterializeWarning>();
        for (const chunk of chunks) {
          const result = await timetableService.materialize(
            chunk.from,
            chunk.to,
          );
          totalCreated += result.instancesCreated;
          for (const warning of result.warnings) {
            warningsByCode.set(warning.code, warning);
          }
          // A precondition like "no_active_period" applies to every chunk,
          // stop hammering the backend once we've seen one.
          if (result.warnings.some((w) => w.code === "no_active_period")) {
            break;
          }
        }
        const warnings = Array.from(warningsByCode.values());
        if (warnings.length > 0) {
          for (const warning of warnings) {
            toast.warning(warning.message);
          }
          if (warnings.some((w) => w.code === "no_active_period")) {
            openPeriodCreate();
          }
        } else {
          toast.success(
            `${totalCreated} ${
              totalCreated === 1 ? "Termin" : "Termine"
            } für "${template.name}" angelegt`,
          );
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("template_apply_failed", {
          template_id: template.id,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error
            ? err.message
            : "Regeltermin konnte nicht eingetragen werden",
        );
      }
    },
    [
      calendarPeriods,
      defaultTemplatePeriod,
      exceptionConflictsSWRKey,
      gapsSWRKey,
      openPeriodCreate,
      swrKey,
      tenantMutate,
      toast,
    ],
  );

  const handleArchiveTemplate = useCallback(async () => {
    if (!archivingTemplate) return;

    setArchiveLoading(true);
    try {
      await timetableService.archiveTemplate(archivingTemplate.id);
      toast.success(`Regeltermin "${archivingTemplate.name}" archiviert`);
      setArchivingTemplate(null);
      if (templatePeriodID) {
        await tenantMutate(`timetable-templates-${templatePeriodID}`);
      }
      await tenantMutate(swrKey);
      await tenantMutate(gapsSWRKey);
      await tenantMutate(exceptionConflictsSWRKey);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Regeltermin konnte nicht archiviert werden";
      logger.error("template_archive_failed", {
        template_id: archivingTemplate.id,
        error: message,
      });
      toast.error(message);
    } finally {
      setArchiveLoading(false);
    }
  }, [
    archivingTemplate,
    exceptionConflictsSWRKey,
    gapsSWRKey,
    swrKey,
    templatePeriodID,
    tenantMutate,
    toast,
  ]);

  if (status === "loading") {
    return <TimetablePageSkeleton />;
  }

  // While the settings schema loads we cannot tell yet whether the feature
  // is enabled — show the normal skeleton instead of flashing the planner.
  if (settingsSchemaLoading) {
    return <TimetablePageSkeleton />;
  }

  if (timetableDisabled) {
    return <TimetableDisabledState />;
  }

  const isOnToday =
    view === "week"
      ? weekOffset === 0 && !selectedDay
      : view === "month"
        ? monthDate.getFullYear() === new Date().getFullYear() &&
          monthDate.getMonth() === new Date().getMonth()
        : view === "year"
          ? yearDate.getFullYear() === new Date().getFullYear()
          : true;

  return (
    <div className="flex flex-col gap-4">
      {/* Mobile title via the shared kit PageHeader (md:hidden); on desktop
          the sidebar provides page context, matching every other page. The
          period switcher lives inside the toolbar (below) so it isn't a dead
          top row. */}
      <PageHeader title="Betreuungsplan" />

      <TimetableToolbar
        view={view}
        onViewChange={handleViewChange}
        rangeLabel={
          view === "month"
            ? monthLabel
            : view === "year"
              ? yearLabel
              : weekLabel
        }
        onPrev={handleToolbarPrev}
        onNext={handleToolbarNext}
        onToday={handleToolbarToday}
        isOnToday={isOnToday}
        navDisabled={view === "series"}
        density={density}
        onDensityChange={setDensity}
        onManagePeriods={handleManagePeriods}
        periodSwitcher={
          showPeriodSwitcher ? (
            <PeriodSwitcherDropdown
              periods={calendarPeriods}
              weekDays={periodContextDays}
              view={view}
              selectedPeriodId={focusedPeriodID}
              isLoading={periodsLoading}
              onCreate={openPeriodCreate}
              onEdit={openPeriodEdit}
              onSelect={jumpToPeriod}
            />
          ) : undefined
        }
        planWeekAction={
          <TimetableAddMenu
            onAddInstance={openEventCreate}
            onAddSeries={openSeriesCreate}
          />
        }
      />

      <TimetableOverview
        plannedLabel={isSeriesView ? "Regeltermine" : "Geplant"}
        plannedCount={plannedCount}
        plannedSublabel={plannedSublabel}
        understaffedCount={understaffedCount}
        understaffedSublabel={understaffedSublabel}
        createLabel={
          isSeriesView ? "Regeltermin erstellen" : "Termin erstellen"
        }
        onCreate={isSeriesView ? openSeriesCreate : openEventCreate}
      />

      <TimetableSetupGuide
        hasActivePeriod={hasActivePeriod}
        activePeriodLabel={activePeriodLabel}
        careOfferingLinkStatus={careOfferingLinkStatus}
        careOfferingLinkLabel={careOfferingLinkLabel}
        hasPlan={hasPlan}
        plannedCount={plannedCount}
        onManagePeriods={handleManagePeriods}
        onCreateEvent={openEventCreate}
        careOfferingsHref="/care-offerings"
      />

      {shouldLoadInstances && (
        <ConflictWarningsBanner conflictCount={conflictCount} />
      )}

      {view === "month" &&
        (isInstanceDataLoading ? (
          <TimetableContentSkeleton view="month" />
        ) : (
          <MonthPlannerGrid
            days={monthDays}
            monthDate={monthDate}
            instances={instances}
            todayISO={todayISO}
            onDayClick={openWeekForDay}
          />
        ))}

      {view === "year" &&
        (isInstanceDataLoading ? (
          <TimetableContentSkeleton view="year" />
        ) : (
          <YearPlannerGrid
            months={yearMonths}
            instances={instances}
            todayISO={todayISO}
            onMonthClick={openMonthFromYear}
            onDayClick={openWeekForDay}
          />
        ))}

      {view === "week" && (
        <>
          {isInstanceDataLoading ? (
            <TimetableContentSkeleton view="week" />
          ) : (
            <WeeklyCalendarGrid
              weekDays={weekDays}
              instances={instances}
              selectedId={selectedInstanceId}
              onInstanceClick={handleSelectInstance}
              onSlotClick={openQuickCreate}
              todayISO={todayISO}
              dayStartHour={dayStartHour}
              dayEndHour={dayEndHour}
              hourHeightPx={hourHeightPx}
              emptyState={
                instances.length === 0 && !error
                  ? {
                      title: weekHasFullPeriodCoverage
                        ? "Diese Woche hat noch keine Termine"
                        : "Diese Woche hat keinen Planungszeitraum",
                      description: weekHasFullPeriodCoverage
                        ? "Plane Angebote über die Toolbar oder lege einen einzelnen Termin an."
                        : "Lege zuerst einen aktiven Planungszeitraum an.",
                    }
                  : undefined
              }
            />
          )}

          {instances.length > 0 && (
            <PlanQualityPanel
              instances={instances}
              gaps={gaps}
              conflicts={exceptionConflicts}
              staff={staff}
              loading={gapsLoading || conflictsLoading}
              error={planQualityErrorMessage}
              onSelectInstance={(instanceId) =>
                updateUrlParams({ instance: instanceId })
              }
              onEditInstance={(instanceId) => {
                const instance = instances.find(
                  (item) => item.id === instanceId,
                );
                if (!instance) return;
                setEditingInstance(instance);
                setEditingTemplate(null);
                setConvertingInstance(null);
                setEventDefaultRepeat("none");
                setEventModalOpen(true);
              }}
              onSubstitute={handleSubstitute}
            />
          )}
        </>
      )}

      {view === "series" && (
        <>
          {templatesLoading ? (
            <TimetableContentSkeleton view="series" />
          ) : (
            <TemplateList
              templates={templates}
              onCreate={openSeriesCreate}
              onEdit={(template) => {
                setEditingInstance(null);
                setConvertingInstance(null);
                setEditingTemplate(template);
                setEventDefaultRepeat("weekly");
                setEventModalOpen(true);
              }}
              onApply={(template) => void handleApplyTemplate(template)}
              onArchive={setArchivingTemplate}
            />
          )}
        </>
      )}

      <InstanceDetailSlideOver
        instance={selectedInstance}
        onClose={() => handleSelectInstance(null)}
        onLifecycleAction={handleLifecycle}
        onDeleteCancelled={handleDeleteCancelledInstance}
        onDeleteFollowing={handleDeleteFollowingInstances}
        onEdit={(instance) => {
          setEditingInstance(instance);
          setEditingTemplate(null);
          setConvertingInstance(null);
          setEventDefaultRepeat("none");
          setEventModalOpen(true);
        }}
        onRepeat={(instance) => {
          if (assignedPeriods.length === 0) {
            toast.warning(
              "Lege zuerst einen Planungszeitraum für diese Woche an.",
            );
            openPeriodCreate();
            return;
          }
          setEditingInstance(null);
          setEditingTemplate(null);
          setConvertingInstance(instance);
          setEventDefaultRepeat("weekly");
          setEventModalOpen(true);
        }}
        staffNames={staffNames}
        studentNames={studentNames}
        onAttendancePatch={handleAttendancePatch}
        editDeferred={false}
      />

      <TimetableEventModal
        canCheckShiftCoverage={canCheckShiftCoverage}
        isOpen={eventModalOpen}
        onClose={() => {
          setEventModalOpen(false);
          setEditingInstance(null);
          setEditingTemplate(null);
          setConvertingInstance(null);
          setEventDefaultRepeat("none");
          setQuickPrefill(null);
        }}
        defaultDate={quickPrefill?.date ?? selectedDay ?? fromISO}
        weekFrom={fromISO}
        weekTo={toISO}
        calendarPeriods={modalCalendarPeriods}
        defaultCalendarPeriodId={templatePeriodID ?? null}
        showPeriodField={showTemplatePeriodField}
        initialInstance={editingInstance}
        initialSeries={editingTemplate}
        convertInstance={convertingInstance}
        onDeleteSeries={handleDeleteSeriesTemplate}
        defaultRepeat={eventDefaultRepeat}
        variant={quickPrefill ? "quick" : "full"}
        defaultStartTime={quickPrefill?.startTime}
        defaultEndTime={quickPrefill?.endTime}
        onSaved={(result) => {
          void tenantMutate(swrKey);
          void tenantMutate(gapsSWRKey);
          void tenantMutate(exceptionConflictsSWRKey);
          if (templatePeriodID) {
            void tenantMutate(`timetable-templates-${templatePeriodID}`);
          }
          if (result.kind === "instance" && result.instance.id) {
            updateUrlParams({ instance: result.instance.id });
          } else if (result.kind === "series" && result.linkedInstanceId) {
            updateUrlParams({ instance: result.linkedInstanceId });
          }
          setEditingInstance(null);
          setEditingTemplate(null);
          setConvertingInstance(null);
        }}
      />

      <CalendarPeriodModal
        isOpen={periodModalOpen}
        onClose={() => setPeriodModalOpen(false)}
        initial={editingPeriod}
        onSaved={() => {
          // Refresh both caches: the period header button and the week grid
          // can pick up the new state without a manual reload.
          void tenantMutate("database-calendar-periods-list");
          void tenantMutate(swrKey);
        }}
        onDeleted={() => {
          void tenantMutate("database-calendar-periods-list");
          void tenantMutate(swrKey);
        }}
        createDefaults={periodCreateDefaults}
        usage={calendarPeriodUsage(editingPeriod)}
      />

      <ConfirmationModal
        isOpen={archivingTemplate !== null}
        onClose={() => {
          if (!archiveLoading) setArchivingTemplate(null);
        }}
        onConfirm={() => void handleArchiveTemplate()}
        title="Regeltermin archivieren?"
        confirmText="Archivieren"
        cancelText="Abbrechen"
        isConfirmLoading={archiveLoading}
      >
        <p className="text-sm leading-relaxed text-gray-600">
          Der Regeltermin
          {archivingTemplate ? (
            <>
              {" "}
              <span className="font-semibold text-gray-900">
                „{archivingTemplate.name}“
              </span>
            </>
          ) : null}{" "}
          verschwindet aus der Liste. Bereits eingetragene Termine bleiben im
          Betreuungsplan erhalten.
        </p>
      </ConfirmationModal>
    </div>
  );
}

// BetreuungsplanView is the embeddable Betreuungsplan surface, hosted by
// /planung (#1886 Konsolidierung); formerly the /timetables page.
export function BetreuungsplanView() {
  return (
    <Suspense fallback={<TimetablePageSkeleton />}>
      <TimetablesContent />
    </Suspense>
  );
}
