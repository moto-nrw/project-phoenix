"use client";

/**
 * /timetables — admin weekly planner.
 *
 * Planner surface for calendar periods, series, materialized instances,
 * one-off appointments, lifecycle actions, and plan-quality checks.
 */

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";

import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { ConflictWarningsBanner } from "~/components/timetable/conflict-warnings-banner";
import {
  InstanceDetailSlideOver,
  type LifecycleAction,
} from "~/components/timetable/instance-detail-slide-over";
import { PlanningMenu } from "~/components/timetable/planning-menu";
import { MonthPlannerGrid } from "~/components/timetable/month-planner-grid";
import { PeriodSwitcherDropdown } from "~/components/timetable/period-switcher-dropdown";
import { PlanQualityPanel } from "~/components/timetable/plan-quality-panel";
import { TemplateList } from "~/components/timetable/template-list";
import { TimetableEventModal } from "~/components/timetable/timetable-event-modal";
import {
  DENSITY_TO_HOUR_HEIGHT_PX,
  TimetableToolbar,
  type TimetableView,
  type WeekDensity,
} from "~/components/timetable/timetable-toolbar";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { YearPlannerGrid } from "~/components/timetable/year-planner-grid";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import {
  findPeriodForDate,
  mapPeriodsForDates,
  uniqueAssignedPeriods,
} from "~/lib/calendar-period-helpers";
import { timetableService } from "~/lib/timetable-api";
import {
  chunkDateRange,
  formatWeekLabel,
  formatMonthLabel,
  formatYearLabel,
  getMonthDays,
  getMonthRange,
  getWeekRange,
  getWeekdays,
  getYearMonths,
  getYearRange,
  toISODate,
} from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  MaterializeWarning,
  TimetableTemplate,
  WeeklyInstancesResponse,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "TimetablesPage" });
const PERIODS_SWR_KEY = "database-calendar-periods-list";

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

function TimetablesContent() {
  const searchParams = useSearchParams();
  const { status } = useSession({ required: true });
  const {
    success: toastSuccess,
    error: toastError,
    warning: toastWarning,
  } = useToast();
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
  const [editingInstance, setEditingInstance] =
    useState<EnrichedInstance | null>(null);
  const [editingTemplate, setEditingTemplate] =
    useState<TimetableTemplate | null>(null);
  const [convertingInstance, setConvertingInstance] =
    useState<EnrichedInstance | null>(null);
  const [periodModalOpen, setPeriodModalOpen] = useState(false);
  // null = create mode (no active period yet); otherwise edit the named period.
  const [editingPeriod, setEditingPeriod] = useState<CalendarPeriod | null>(
    null,
  );
  // Density picker for the week grid (Kompakt/Normal/Komfortabel maps to
  // 60/90/120 px per hour). Local-only — not synced to URL because density
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
          instance: null,
          day: null,
        });
        return;
      }

      if (view === "year") {
        updateUrlParams({
          year: yearParam(visibleDate),
          instance: null,
          day: null,
        });
        return;
      }

      const offset = weekOffsetForDate(visibleDateISO);
      updateUrlParams({
        week: offset === 0 ? null : String(offset),
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

  // Week range. useMemo prevents re-derivation on every render — the SWR
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
  const {
    data: periods,
    error: periodsError,
    isLoading: periodsLoading,
  } = useSWRAuth(status === "authenticated" ? PERIODS_SWR_KEY : null, () =>
    calendarPeriodService.list(),
  );

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
    toastError(`Stundenplan konnte nicht geladen werden: ${errorMessage}`);
  }, [errorMessage, toastError]);

  useEffect(() => {
    if (!periodsError) return;
    const message =
      periodsError instanceof Error
        ? periodsError.message
        : String(periodsError);
    logger.error("periods_load_failed", { error: message });
    toastError(`Planungsperioden konnten nicht geladen werden: ${message}`);
  }, [periodsError, toastError]);

  useEffect(() => {
    if (!planQualityErrorMessage) return;
    logger.error("plan_quality_load_failed", {
      error: planQualityErrorMessage,
    });
    toastError("Planstatus konnte nicht vollständig geladen werden.");
  }, [planQualityErrorMessage, toastError]);

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

  const handleMaterialize = useCallback(async () => {
    if (!weekHasFullPeriodCoverage) {
      toastWarning(
        "Lege zuerst eine aktive Planungsperiode für diese Woche an.",
      );
      openPeriodCreate();
      return;
    }

    try {
      const result = await timetableService.materialize(fromISO, toISO);

      // The backend reports preconditions (no active calendar period, no
      // templates) as typed warnings rather than HTTP errors so the run
      // logs cleanly. Surface them as warnings so the admin sees the actual
      // reason instead of a misleading "0 angelegt" success message.
      if (result.warnings.length > 0) {
        for (const w of result.warnings) {
          toastWarning(w.message);
        }
        // For "no_active_period" specifically: open the period editor so
        // the admin can fix the precondition without leaving the planner.
        if (result.warnings.some((w) => w.code === "no_active_period")) {
          openPeriodCreate();
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
        return;
      }

      toastSuccess(
        `Woche geplant: ${result.instancesCreated} ${
          result.instancesCreated === 1 ? "Termin" : "Termine"
        } angelegt`,
      );
      await tenantMutate(swrKey);
      await tenantMutate(gapsSWRKey);
      await tenantMutate(exceptionConflictsSWRKey);
    } catch (err) {
      logger.error("materialize_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError("Woche konnte nicht geplant werden");
    }
  }, [
    fromISO,
    toISO,
    swrKey,
    gapsSWRKey,
    exceptionConflictsSWRKey,
    toastSuccess,
    toastError,
    toastWarning,
    tenantMutate,
    openPeriodCreate,
    weekHasFullPeriodCoverage,
  ]);

  const handleLifecycle = useCallback(
    async (action: LifecycleAction) => {
      if (!selectedInstance) return;
      try {
        if (action === "start") {
          const res = await timetableService.start(selectedInstance.id);
          if (res.warnings.length > 0) {
            toastSuccess(
              `Gestartet — ${res.warnings.length} Hinweis(e): ${res.warnings.map((w) => w.message).join(", ")}`,
            );
          } else {
            toastSuccess("Aktivität gestartet");
          }
        } else if (action === "complete") {
          await timetableService.complete(selectedInstance.id);
          toastSuccess("Aktivität beendet");
        } else {
          await timetableService.cancel(selectedInstance.id);
          toastSuccess("Aktivität abgesagt");
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
        toastError(
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
      toastSuccess,
      toastError,
    ],
  );

  const handleReplanWeek = useCallback(async () => {
    try {
      const result = await timetableService.replanWeek(fromISO, toISO);
      if (result.warnings.length > 0) {
        for (const warning of result.warnings) {
          toastError(warning.message);
        }
      } else {
        toastSuccess(
          `Woche neu berechnet: ${result.instancesCreated} Termine angelegt`,
        );
      }
      await tenantMutate(swrKey);
      await tenantMutate(gapsSWRKey);
      await tenantMutate(exceptionConflictsSWRKey);
    } catch (err) {
      logger.error("replan_week_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError(
        err instanceof Error
          ? err.message
          : "Woche konnte nicht neu berechnet werden",
      );
      throw err;
    }
  }, [
    fromISO,
    toISO,
    swrKey,
    gapsSWRKey,
    exceptionConflictsSWRKey,
    tenantMutate,
    toastError,
    toastSuccess,
  ]);

  const handleSubstitute = useCallback(
    async (absentStaffId: string, substituteStaffId: string, date: string) => {
      try {
        const result = await timetableService.substitute(
          absentStaffId,
          substituteStaffId,
          date,
        );
        toastSuccess(
          `Ersatz eingetragen: ${result.affectedInstances.length} Termin(e) aktualisiert`,
        );
        if (result.warnings.length > 0) {
          toastError(
            `${result.warnings.length} mögliche Zeitüberschneidung(en) prüfen.`,
          );
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("substitute_failed", {
          absent_staff_id: absentStaffId,
          substitute_staff_id: substituteStaffId,
          date,
          error: err instanceof Error ? err.message : String(err),
        });
        toastError(
          err instanceof Error
            ? err.message
            : "Ersatz konnte nicht eingetragen werden",
        );
        throw err;
      }
    },
    [
      exceptionConflictsSWRKey,
      gapsSWRKey,
      swrKey,
      tenantMutate,
      toastError,
      toastSuccess,
    ],
  );

  const handleAttendancePatch = useCallback(
    async (
      instanceId: string,
      studentId: string,
      body: Parameters<typeof timetableService.patchAttendance>[2],
    ) => {
      try {
        await timetableService.patchAttendance(instanceId, studentId, body);
        toastSuccess("Kinderstatus aktualisiert");
        await tenantMutate(swrKey);
        await tenantMutate(exceptionConflictsSWRKey);
      } catch (err) {
        logger.error("attendance_patch_failed", {
          instance_id: instanceId,
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
        toastError(
          err instanceof Error
            ? err.message
            : "Kinderstatus konnte nicht aktualisiert werden",
        );
        throw err;
      }
    },
    [exceptionConflictsSWRKey, swrKey, tenantMutate, toastError, toastSuccess],
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

  const handleApplyTemplate = useCallback(
    async (template: TimetableTemplate) => {
      // Resolve which calendar period the template's schedules belong to.
      // Falls back to the period currently in scope (defaultTemplatePeriod)
      // — if none is active, ask the admin to create one before continuing.
      const scheduleWithPeriod = template.schedules.find(
        (s) => s.calendarPeriodId,
      );
      const periodId =
        scheduleWithPeriod?.calendarPeriodId ?? defaultTemplatePeriod?.id;
      const period = periodId
        ? calendarPeriods.find((p) => p.id === periodId)
        : null;
      if (!period) {
        toastWarning(
          "Keine aktive Planungsperiode - Serie kann nicht eingeplant werden.",
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
          // A precondition like "no_active_period" applies to every chunk —
          // stop hammering the backend once we've seen one.
          if (result.warnings.some((w) => w.code === "no_active_period")) {
            break;
          }
        }
        const warnings = Array.from(warningsByCode.values());
        if (warnings.length > 0) {
          for (const warning of warnings) {
            toastWarning(warning.message);
          }
          if (warnings.some((w) => w.code === "no_active_period")) {
            openPeriodCreate();
          }
        } else {
          toastSuccess(
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
        toastError(
          err instanceof Error
            ? err.message
            : "Serie konnte nicht eingeplant werden",
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
      toastError,
      toastWarning,
      toastSuccess,
    ],
  );

  if (status === "loading") {
    return (
      <div className="p-6">
        <Loading />
      </div>
    );
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
    <div className="flex flex-col gap-4 p-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold text-slate-900">Stundenplan</h1>
        <div className="flex flex-wrap items-center gap-2">
          <PeriodSwitcherDropdown
            periods={calendarPeriods}
            weekDays={periodContextDays}
            view={view}
            selectedPeriodId={
              view === "series" ? (templatePeriodID ?? null) : null
            }
            isLoading={periodsLoading}
            onCreate={openPeriodCreate}
            onEdit={openPeriodEdit}
            onSelect={jumpToPeriod}
          />
        </div>
      </header>

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
        onAddInstance={openEventCreate}
        planWeekAction={
          view === "week" && weekHasFullPeriodCoverage ? (
            <PlanningMenu
              onMaterialize={handleMaterialize}
              onReplan={handleReplanWeek}
              weekLabel={weekLabel}
              emphasis="accent"
              label="Angebote einplanen"
            />
          ) : null
        }
      />

      {shouldLoadInstances && (
        <ConflictWarningsBanner conflictCount={conflictCount} />
      )}

      {view === "month" &&
        (isInstanceDataLoading ? (
          <div className="rounded-xl border border-slate-200 bg-white p-8">
            <Loading />
          </div>
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
          <div className="rounded-xl border border-slate-200 bg-white p-8">
            <Loading />
          </div>
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
            <div className="rounded-xl border border-slate-200 bg-white p-8">
              <Loading />
            </div>
          ) : (
            <WeeklyCalendarGrid
              weekDays={weekDays}
              instances={instances}
              selectedId={selectedInstanceId}
              onInstanceClick={handleSelectInstance}
              todayISO={todayISO}
              dayStartHour={dayStartHour}
              dayEndHour={dayEndHour}
              hourHeightPx={hourHeightPx}
              emptyState={
                instances.length === 0 && !error
                  ? {
                      title: weekHasFullPeriodCoverage
                        ? "Diese Woche hat noch keine Termine"
                        : "Diese Woche hat keine Planungsperiode",
                      description: weekHasFullPeriodCoverage
                        ? "Plane Angebote über die Toolbar oder lege einen einzelnen Termin an."
                        : "Lege zuerst eine aktive Planungsperiode an.",
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
            <Loading />
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
            />
          )}
        </>
      )}

      <InstanceDetailSlideOver
        instance={selectedInstance}
        onClose={() => handleSelectInstance(null)}
        onLifecycleAction={handleLifecycle}
        onEdit={(instance) => {
          setEditingInstance(instance);
          setEditingTemplate(null);
          setConvertingInstance(null);
          setEventDefaultRepeat("none");
          setEventModalOpen(true);
        }}
        onRepeat={(instance) => {
          if (assignedPeriods.length === 0) {
            toastWarning(
              "Lege zuerst eine Planungsperiode für diese Woche an.",
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
        isOpen={eventModalOpen}
        onClose={() => {
          setEventModalOpen(false);
          setEditingInstance(null);
          setEditingTemplate(null);
          setConvertingInstance(null);
          setEventDefaultRepeat("none");
        }}
        defaultDate={selectedDay ?? fromISO}
        weekFrom={fromISO}
        weekTo={toISO}
        calendarPeriods={assignedPeriods}
        defaultCalendarPeriodId={defaultTemplatePeriod?.id ?? null}
        showPeriodField={showTemplatePeriodField}
        initialInstance={editingInstance}
        initialSeries={editingTemplate}
        convertInstance={convertingInstance}
        defaultRepeat={eventDefaultRepeat}
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
      />
    </div>
  );
}

export default function TimetablesPage() {
  return (
    <Suspense
      fallback={
        <div className="p-6">
          <Loading />
        </div>
      }
    >
      <TimetablesContent />
    </Suspense>
  );
}
