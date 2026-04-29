"use client";

/**
 * /database/timetables — admin weekly planner.
 *
 * Read-only MVP scope:
 * - Fetches one week of materialized instances via GET /api/timetable/instances
 * - Click on a card opens a right-side slide-over with the full state
 * - Lifecycle actions (Starten, Beenden, Absagen) wired to the existing
 *   POST /instances/{id}/{action} endpoints
 * - "Plan aktualisieren" runs POST /api/timetable/materialize for the
 *   visible week
 *
 * Deferred to a follow-up PR (matching the Read+Edit+Spontan plan):
 * - Editing time/room/staff (PUT /instances/{id})
 * - Spontaneous instance create (POST /instances)
 * - Per-page SSE wiring for instance_* events (today the global SSE
 *   handler does not invalidate timetable caches — lifecycle actions
 *   trigger refetch manually)
 */

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { CalendarPeriodHeaderButton } from "~/components/timetable/calendar-period-header-button";
import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { Loading } from "~/components/ui/loading";
import { useToast } from "~/contexts/ToastContext";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { ConflictWarningsBanner } from "~/components/timetable/conflict-warnings-banner";
import {
  InstanceDetailSlideOver,
  type LifecycleAction,
} from "~/components/timetable/instance-detail-slide-over";
import { MaterializeButton } from "~/components/timetable/materialize-button";
import { MonthPlannerGrid } from "~/components/timetable/month-planner-grid";
import { PlanQualityPanel } from "~/components/timetable/plan-quality-panel";
import { RecurringActivityModal } from "~/components/timetable/recurring-activity-modal";
import { TemplateList } from "~/components/timetable/template-list";
import { TimetableInstanceModal } from "~/components/timetable/timetable-instance-modal";
import { WeekNavigator } from "~/components/timetable/week-navigator";
import { WeeklyPlannerGrid } from "~/components/timetable/weekly-planner-grid";
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
  formatWeekLabel,
  formatMonthLabel,
  getMonthDays,
  getMonthRange,
  getWeekRange,
  getWeekdays,
  toISODate,
} from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  TimetableTemplate,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "DatabaseTimetablesPage" });
const PERIODS_SWR_KEY = "database-calendar-periods-list";
type TimetableView = "month" | "week" | "templates";

function parseWeekOffset(raw: string | null): number {
  if (raw === null) return 0;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : 0;
}

function parseView(raw: string | null): TimetableView {
  if (raw === "week" || raw === "templates") return raw;
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

function weekOffsetForDate(dateISO: string): number {
  const current = getWeekRange(new Date(), 0).from;
  const target = getWeekRange(new Date(`${dateISO}T00:00:00`), 0).from;
  return Math.round((target.getTime() - current.getTime()) / 604800000);
}

function TimetablesContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const { status } = useSession({ required: true });
  const { success: toastSuccess, error: toastError } = useToast();
  const tenantMutate = useTenantMutate();

  const view = parseView(searchParams.get("view"));
  const weekOffset = parseWeekOffset(searchParams.get("week"));
  const monthDate = parseMonth(searchParams.get("month"));
  const selectedInstanceId = searchParams.get("instance");
  const selectedDay = searchParams.get("day");
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [instanceModalOpen, setInstanceModalOpen] = useState(false);
  const [editingInstance, setEditingInstance] =
    useState<EnrichedInstance | null>(null);
  const [editingTemplate, setEditingTemplate] =
    useState<TimetableTemplate | null>(null);
  const [periodModalOpen, setPeriodModalOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] =
    useState<TimetableTemplate | null>(null);
  // null = create mode (no active period yet); otherwise edit the named period.
  const [editingPeriod, setEditingPeriod] = useState<CalendarPeriod | null>(
    null,
  );

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
      const next = new URLSearchParams(searchParams.toString());
      for (const [key, value] of Object.entries(patch)) {
        if (value === null || value === "") {
          next.delete(key);
        } else {
          next.set(key, value);
        }
      }
      const qs = next.toString();
      router.replace(qs ? `?${qs}` : "?", { scroll: false });
    },
    [router, searchParams],
  );

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
      updateUrlParams({
        view: nextView === "month" ? null : nextView,
        instance: null,
        day: null,
      });
    },
    [updateUrlParams],
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
  const fetchFromISO = view === "month" ? toISODate(monthRange.from) : fromISO;
  const fetchToISO = view === "month" ? toISODate(monthRange.to) : toISO;
  const weekDays = useMemo(() => getWeekdays(weekRange.from), [weekRange.from]);
  const periodContextDays = view === "month" ? monthDays : weekDays;
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

  const swrKey = `timetable-${view}-${fetchFromISO}-${fetchToISO}`;
  const shouldLoadInstances = view !== "templates";
  const qualityFromISO = fromISO < todayISO ? todayISO : fromISO;
  const shouldLoadPlanQuality = view === "week" && toISO >= todayISO;
  const gapsSWRKey = `timetable-gaps-${qualityFromISO}-${toISO}`;
  const exceptionConflictsSWRKey = `timetable-exception-conflicts-${qualityFromISO}-${toISO}`;
  const { data, error, isLoading } = useSWRAuth(
    status === "authenticated" && shouldLoadInstances ? swrKey : null,
    () => timetableService.getWeek(fetchFromISO, fetchToISO),
  );
  const { data: gapsData, isLoading: gapsLoading } = useSWRAuth(
    status === "authenticated" && shouldLoadPlanQuality ? gapsSWRKey : null,
    () => timetableService.getGaps(qualityFromISO, toISO),
  );
  const { data: exceptionConflictData, isLoading: conflictsLoading } =
    useSWRAuth(
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
  const defaultTemplatePeriod = useMemo(
    () =>
      findPeriodForDate(
        calendarPeriods,
        view === "month" ? toISODate(monthDate) : fromISO,
      ),
    [calendarPeriods, fromISO, monthDate, view],
  );
  const templatePeriodID = defaultTemplatePeriod?.id ?? assignedPeriods[0]?.id;
  const { data: templateData, isLoading: templatesLoading } = useSWRAuth(
    status === "authenticated" && view === "templates" && templatePeriodID
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
    const end = new Date(weekRange.from);
    end.setFullYear(end.getFullYear() + 1);
    end.setDate(end.getDate() - 1);
    const start = fromISO;
    const endISO = toISODate(end);
    const startYear = weekRange.from.getFullYear();
    const endYear = end.getFullYear();
    return {
      name: `Schuljahr ${startYear}/${endYear}`,
      periodType: "school_year" as const,
      startDate: start,
      endDate: endISO,
      weekCycleLength: "1",
      weekCycleAnchor: "",
      isActive: true,
    };
  }, [fromISO, weekRange.from]);
  const conflictCount = useMemo(
    () =>
      instances.reduce((sum, inst) => sum + inst.conflictWarnings.length, 0),
    [instances],
  );

  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );

  const handleMaterialize = useCallback(async () => {
    try {
      const result = await timetableService.materialize(fromISO, toISO);

      // The backend reports preconditions (no active calendar period, no
      // templates) as typed warnings rather than HTTP errors so the run
      // logs cleanly. Surface them as error toasts so the admin sees the
      // actual reason instead of a misleading "0 angelegt" success message.
      if (result.warnings.length > 0) {
        for (const w of result.warnings) {
          toastError(w.message);
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
    tenantMutate,
    openPeriodCreate,
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
    [selectedInstance, swrKey, tenantMutate, toastSuccess, toastError],
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
    [gapsSWRKey, swrKey, tenantMutate, toastError, toastSuccess],
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
    [swrKey, tenantMutate, toastError, toastSuccess],
  );

  const openTemplateCreate = useCallback(() => {
    if (assignedPeriods.length === 0) {
      toastError("Lege zuerst eine Planungsperiode für diese Woche an.");
      openPeriodCreate();
      return;
    }
    setEditingTemplate(null);
    setCreateModalOpen(true);
  }, [assignedPeriods.length, openPeriodCreate, toastError]);

  const openInstanceCreate = useCallback(() => {
    setEditingInstance(null);
    setInstanceModalOpen(true);
  }, []);

  const handleArchiveTemplate = useCallback(
    async (template: TimetableTemplate) => {
      try {
        await timetableService.archiveTemplate(template.id);
        toastSuccess(`Vorlage "${template.name}" archiviert`);
        setSelectedTemplate(null);
        if (templatePeriodID) {
          await tenantMutate(`timetable-templates-${templatePeriodID}`);
        }
      } catch (err) {
        logger.error("template_archive_failed", {
          template_id: template.id,
          error: err instanceof Error ? err.message : String(err),
        });
        toastError(
          err instanceof Error
            ? err.message
            : "Vorlage konnte nicht archiviert werden",
        );
      }
    },
    [templatePeriodID, tenantMutate, toastError, toastSuccess],
  );

  if (status === "loading" || (shouldLoadInstances && isLoading)) {
    return (
      <div className="p-6">
        <Loading />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 p-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-bold text-slate-900">Stundenplan</h1>
          <p className="text-sm text-slate-500">
            {view === "month"
              ? `Monatsüberblick • ${monthLabel}`
              : view === "templates"
                ? "Vorlagen für wiederkehrende Aktivitäten"
                : `Wochenplan • ${weekLabel}`}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <CalendarPeriodHeaderButton
            periods={calendarPeriods}
            weekDays={periodContextDays}
            isLoading={periodsLoading}
            onCreate={openPeriodCreate}
            onEdit={openPeriodEdit}
          />
          {view === "week" && instances.length > 0 && (
            <MaterializeButton
              onMaterialize={handleMaterialize}
              weekLabel={weekLabel}
              variant="secondary"
            />
          )}
        </div>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <ViewToggle value={view} onChange={handleViewChange} />
        {view === "month" ? (
          <div className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white p-1.5">
            <button
              type="button"
              onClick={() => handleMonthChange(-1)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-600 hover:bg-slate-100"
              aria-label="Vorheriger Monat"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="px-2 text-sm font-semibold text-slate-900">
              {monthLabel}
            </span>
            <button
              type="button"
              onClick={() => handleMonthChange(1)}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-600 hover:bg-slate-100"
              aria-label="Nächster Monat"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        ) : view === "week" ? (
          <WeekNavigator
            weekOffset={weekOffset}
            weekRange={weekRange}
            onChange={handleWeekChange}
          />
        ) : null}
        <div className="flex-1" />
        <button
          type="button"
          onClick={openInstanceCreate}
          className="inline-flex items-center gap-2 rounded-md border border-slate-300 bg-white px-3 py-2 text-xs font-semibold text-slate-700 shadow-sm transition-colors hover:bg-slate-100"
        >
          + Termin
        </button>
        <button
          type="button"
          onClick={openTemplateCreate}
          className="inline-flex items-center gap-2 rounded-md border border-[#5080D8] bg-[#5080D8] px-3 py-2 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-[#4070c8]"
        >
          + Vorlage
        </button>
      </div>

      {shouldLoadInstances && (
        <ConflictWarningsBanner conflictCount={conflictCount} />
      )}

      {view === "month" && (
        <MonthPlannerGrid
          days={monthDays}
          monthDate={monthDate}
          instances={instances}
          todayISO={todayISO}
          onDayClick={openWeekForDay}
          onPlanWeek={(dateISO) => {
            openWeekForDay(dateISO);
          }}
        />
      )}

      {view === "week" && (
        <>
          <WeeklyPlannerGrid
            weekDays={weekDays}
            instances={instances}
            selectedId={selectedInstanceId}
            onInstanceClick={handleSelectInstance}
            todayISO={todayISO}
          />

          {instances.length === 0 && !error && (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 p-6 text-center">
              <h3 className="text-base font-bold text-slate-900">
                Diese Woche hat noch keine Termine
              </h3>
              <p className="mt-1 text-sm text-slate-500">
                Plane die Woche aus den Vorlagen oder lege zuerst eine Vorlage
                für Mensa, Lernzeit, AGs oder externe Angebote an.
              </p>
              <div className="mt-4 flex flex-wrap justify-center gap-3">
                <MaterializeButton
                  onMaterialize={handleMaterialize}
                  weekLabel={weekLabel}
                  variant="primary"
                />
                <button
                  type="button"
                  onClick={openTemplateCreate}
                  className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                >
                  Vorlage anlegen
                </button>
                <button
                  type="button"
                  onClick={openInstanceCreate}
                  className="rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                >
                  Termin hinzufügen
                </button>
              </div>
            </div>
          )}

          {instances.length > 0 && (
            <PlanQualityPanel
              instances={instances}
              gaps={gaps}
              conflicts={exceptionConflicts}
              staff={staff}
              loading={gapsLoading || conflictsLoading}
              onSelectInstance={(instanceId) =>
                updateUrlParams({ instance: instanceId })
              }
              onSubstitute={handleSubstitute}
              onReplanWeek={handleReplanWeek}
            />
          )}
        </>
      )}

      {view === "templates" && (
        <>
          {templatesLoading ? (
            <Loading />
          ) : (
            <TemplateList
              templates={templates}
              selectedId={selectedTemplate?.id ?? templates[0]?.id ?? null}
              onSelect={setSelectedTemplate}
              onCreate={openTemplateCreate}
              onEdit={(template) => {
                setEditingTemplate(template);
                setCreateModalOpen(true);
              }}
              onArchive={(template) => void handleArchiveTemplate(template)}
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
          setInstanceModalOpen(true);
        }}
        staffNames={staffNames}
        studentNames={studentNames}
        onAttendancePatch={handleAttendancePatch}
        editDeferred={false}
      />

      <TimetableInstanceModal
        isOpen={instanceModalOpen}
        onClose={() => setInstanceModalOpen(false)}
        defaultDate={selectedDay ?? fromISO}
        initialInstance={editingInstance}
        onSaved={(instance) => {
          void tenantMutate(swrKey);
          if (instance.id) {
            updateUrlParams({ instance: instance.id });
          }
        }}
      />

      <RecurringActivityModal
        isOpen={createModalOpen}
        onClose={() => {
          setCreateModalOpen(false);
          setEditingTemplate(null);
        }}
        weekFrom={fromISO}
        weekTo={toISO}
        calendarPeriods={assignedPeriods}
        defaultCalendarPeriodId={defaultTemplatePeriod?.id ?? null}
        showPeriodField={showTemplatePeriodField}
        initialTemplate={editingTemplate}
        onCreated={() => {
          // The backend already materialised the visible week as part of
          // the create call (we passed weekFrom/weekTo). Refetch so the
          // fresh instances appear immediately.
          void tenantMutate(swrKey);
          if (templatePeriodID) {
            void tenantMutate(`timetable-templates-${templatePeriodID}`);
          }
          setEditingTemplate(null);
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

function ViewToggle({
  value,
  onChange,
}: {
  value: TimetableView;
  onChange: (view: TimetableView) => void;
}) {
  const items: Array<{ value: TimetableView; label: string }> = [
    { value: "month", label: "Monat" },
    { value: "week", label: "Woche" },
    { value: "templates", label: "Vorlagen" },
  ];
  return (
    <div className="inline-flex rounded-lg border border-slate-200 bg-white p-1">
      {items.map((item) => (
        <button
          key={item.value}
          type="button"
          onClick={() => onChange(item.value)}
          className={`rounded-md px-3 py-1.5 text-sm font-semibold transition-colors ${
            value === item.value
              ? "bg-[#5080D8] text-white"
              : "text-slate-600 hover:bg-slate-100"
          }`}
        >
          {item.label}
        </button>
      ))}
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
