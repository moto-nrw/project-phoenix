"use client";

/**
 * Betreuungsplan view (ehemals /timetables): admin weekly planner. Hosted as
 * a tab of /planung (#1886).
 *
 * Planner surface for calendar periods, series, materialized instances,
 * one-off appointments, lifecycle actions, and plan-quality checks.
 *
 * URL-Vokabular (Planung-Redesign, docs/planung-redesign/docs/06-betreuungsplan.md
 * Abschnitt 2.1): genau `d` (Berlin-Kalendertag; die angezeigte Woche/der Monat
 * ist die Woche/der Monat, die/den `d` enthält), `view` ("woche" | "monat" |
 * "serien") und `block` (Instanz-ID des geöffneten InstanceDetailSlideOver).
 * Ungültige Werte fallen still auf die Defaults zurück (heute, "woche"). Die
 * sieben Alt-Parameter (week/month/year/instance/day/period/Alt-view) entfallen
 * ersatzlos; die Jahresansicht ist nicht mehr verlinkbar. Der Dichte-Umschalter
 * bleibt reiner Component-State.
 */

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { CalendarOff } from "lucide-react";

import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { OriginChip } from "~/components/ui/origin-chip";
import { PageHeader } from "~/components/ui/page-header/PageHeader";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { ConflictWarningsBanner } from "~/components/timetable/conflict-warnings-banner";
import { GapJumpList } from "~/components/timetable/gap-jump-list";
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
import { resolveDemandOrigin } from "~/components/timetable/demand-origin";
import { hasPermission } from "~/lib/auth-utils";
import {
  DENSITY_TO_HOUR_HEIGHT_PX,
  type TimetableView,
  type WeekDensity,
} from "~/components/timetable/timetable-toolbar";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { YearPlannerGrid } from "~/components/timetable/year-planner-grid";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { useUrlParams } from "~/lib/hooks/use-url-params";
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
import {
  berlinTodayISO,
  formatDate,
  isValidISODate,
  parseISODate,
} from "~/lib/date-helpers";
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
const PHASES_SWR_KEY = "timetable-enrollment-phases";

// Das verbindliche Drei-Parameter-Vokabular (06 §2.1). updateUrlParams baut die
// URL aus dieser Allowlist neu auf, damit fremde Params (?utm_source=…) nicht
// jeden Wochen-/Ansichts-/Block-Wechsel überleben.
const ALLOWED_URL_PARAMS = ["d", "view", "block"] as const;

type ViewParam = "woche" | "monat" | "serien";

/**
 * Übersetzt den `view`-URL-Wert in den internen Ansichts-Typ. Ungültige Werte
 * fallen still auf die Woche zurück; die Jahresansicht ist nicht mehr
 * verlinkbar (kein `view`-Wert erzeugt sie).
 */
function parseViewParam(raw: string | null): TimetableView {
  if (raw === "monat") return "month";
  if (raw === "serien") return "series";
  return "week";
}

/** Segmentschalter-Wert (deutsches Vokabular) aus dem internen Ansichts-Typ. */
function viewToTab(view: TimetableView): ViewParam {
  if (view === "month") return "monat";
  if (view === "series") return "serien";
  return "woche";
}

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
  const { params, updateParams: updateUrlParams } = useUrlParams(
    ALLOWED_URL_PARAMS,
    { syncPopstate: true },
  );
  const { data: session, status } = useSession({ required: true });
  const canCheckShiftCoverage =
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "users:read");
  const toast = useToast();
  const tenantMutate = useTenantMutate();

  // Sichtbares Datum, Ansicht und geöffneter Block werden bei jedem Render aus
  // den Search-Params abgeleitet (keine Offset-Arithmetik, kein weekOffset-State
  // mehr). Ein ungültiges `d` fällt still auf heute zurück, ein unbekanntes
  // `view` auf die Woche.
  const rawDay = params.d;
  const dayISO =
    rawDay !== null && isValidISODate(rawDay) ? rawDay : berlinTodayISO();
  const view: TimetableView = parseViewParam(params.view);
  const selectedInstanceId = params.block;

  const visibleDate = useMemo(() => parseISODate(dayISO), [dayISO]);
  const todayISO = useMemo(() => berlinTodayISO(), []);

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
  // StrictMode double-invokes effects; the ref keeps the (idempotent)
  // bootstrap POST from firing twice per mount.
  const bootstrapAttemptedRef = useRef(false);
  // Density picker for the week grid (Kompakt/Normal/Komfortabel maps to
  // 60/90/120 px per hour). Local-only, never synced to URL. Der Umschalter
  // selbst wandert mit dem Chrome-Abbau (Chunk 5/6); der Zustand bleibt hier
  // als Component-State erhalten (06 §2.1).
  const [density] = useState<WeekDensity>("normal");
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

  // Wochen-Navigation ankert am Montag der sichtbaren Woche und schreibt den
  // Montag der Zielwoche nach `d` (konsistent mit dienstplan goToWeek).
  const weekRange = useMemo(() => getWeekRange(visibleDate, 0), [visibleDate]);
  const goToWeek = useCallback(
    (deltaDays: number) => {
      const target = new Date(weekRange.from);
      target.setDate(target.getDate() + deltaDays);
      updateUrlParams({ d: toISODate(target), block: null });
    },
    [weekRange.from, updateUrlParams],
  );
  const goToMonth = useCallback(
    (delta: number) => {
      const target = new Date(
        visibleDate.getFullYear(),
        visibleDate.getMonth() + delta,
        1,
      );
      updateUrlParams({ d: toISODate(target), block: null });
    },
    [visibleDate, updateUrlParams],
  );
  const goToYear = useCallback(
    (delta: number) => {
      const target = new Date(visibleDate.getFullYear() + delta, 0, 1);
      updateUrlParams({ d: toISODate(target), block: null });
    },
    [visibleDate, updateUrlParams],
  );
  const goToToday = useCallback(
    () => updateUrlParams({ d: todayISO, block: null }),
    [todayISO, updateUrlParams],
  );

  const handlePrev = useCallback(() => {
    if (view === "week") goToWeek(-7);
    else if (view === "month") goToMonth(-1);
    else if (view === "year") goToYear(-1);
  }, [view, goToWeek, goToMonth, goToYear]);
  const handleNext = useCallback(() => {
    if (view === "week") goToWeek(7);
    else if (view === "month") goToMonth(1);
    else if (view === "year") goToYear(1);
  }, [view, goToWeek, goToMonth, goToYear]);

  // Ansichtswechsel schreibt `view` (Woche als Default = Param-Entfernung) und
  // schließt den Slide-Over (Block-Param abräumen).
  const setViewParam = useCallback(
    (tab: ViewParam) => {
      updateUrlParams({ view: tab === "woche" ? null : tab, block: null });
    },
    [updateUrlParams],
  );

  // Monatsklick auf einen Tag: in die Woche wechseln und `d` auf den Tag setzen.
  const openWeekForDay = useCallback(
    (dateISO: string) => {
      updateUrlParams({ view: null, d: dateISO, block: null });
    },
    [updateUrlParams],
  );

  // Jahresraster (nicht mehr verlinkbar, toter Renderpfad bis Chunk 5): Klick
  // auf einen Monat wechselt in die Monatsansicht.
  const openMonthFromYear = useCallback(
    (month: Date) => {
      updateUrlParams({ view: "monat", d: toISODate(month), block: null });
    },
    [updateUrlParams],
  );

  const handleSelectInstance = useCallback(
    (instance: EnrichedInstance | null) => {
      updateUrlParams({ block: instance?.id ?? null });
    },
    [updateUrlParams],
  );

  // Fetch-Fenster. Woche/Monat leiten ihr Fenster aus `d` ab; die Serienansicht
  // lädt keine Instanzen. Der Jahres-Renderpfad bleibt bis Chunk 5 bestehen,
  // ist aber über die URL nicht mehr erreichbar.
  const fromISO = toISODate(weekRange.from);
  const toISO = toISODate(weekRange.to);
  const monthRange = useMemo(() => getMonthRange(visibleDate), [visibleDate]);
  const monthDays = useMemo(() => getMonthDays(visibleDate), [visibleDate]);
  const yearDate = useMemo(
    () => new Date(visibleDate.getFullYear(), 0, 1),
    [visibleDate],
  );
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
  const weekLabel = useMemo(
    () => formatWeekLabel(weekRange.from, weekRange.to),
    [weekRange.from, weekRange.to],
  );
  const monthLabel = useMemo(
    () => formatMonthLabel(visibleDate),
    [visibleDate],
  );
  const yearLabel = useMemo(() => formatYearLabel(yearDate), [yearDate]);

  const swrKey = `timetable-${view}-${fetchFromISO}-${fetchToISO}`;
  const shouldLoadInstances = view !== "series";
  // Lücken sind vorwärtsgerichtet: der Endpunkt lehnt ein vergangenes `date`
  // ab, also den Fensterstart auf heute klemmen und vollständig vergangene
  // Fenster überspringen. Der Lückenzähler braucht sie in Woche UND Monat.
  const gapsFromISO = fetchFromISO < todayISO ? todayISO : fetchFromISO;
  const shouldLoadGaps =
    (view === "week" || view === "month") && fetchToISO >= todayISO;
  // Ausnahmekonflikte nur für die Wochenansicht (einziger Abnehmer:
  // PlanQualityPanel).
  const shouldLoadConflicts = view === "week" && toISO >= todayISO;
  const gapsSWRKey = `timetable-gaps-${gapsFromISO}-${fetchToISO}`;
  const exceptionConflictsSWRKey = `timetable-exception-conflicts-${gapsFromISO}-${toISO}`;
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
    status === "authenticated" && shouldLoadGaps ? gapsSWRKey : null,
    () => timetableService.getGaps(gapsFromISO, fetchToISO),
  );
  const {
    data: exceptionConflictData,
    error: conflictsError,
    isLoading: conflictsLoading,
  } = useSWRAuth(
    status === "authenticated" && shouldLoadConflicts
      ? exceptionConflictsSWRKey
      : null,
    () => timetableService.getExceptionConflicts(gapsFromISO, toISO),
  );
  const { data: staffData } = useSWRAuth(
    status === "authenticated" ? "timetable-staff-list" : null,
    () => staffService.getAllStaff(),
  );
  const { data: studentData } = useSWRAuth(
    status === "authenticated" ? "timetable-student-list" : null,
    () => fetchStudents({ page_size: 500 }),
  );
  // Bedarfsquellen-Chip (06 §3.2): die Anmeldephasen für die clientseitige
  // Zuordnung. Eigener SWR-Key, damit der Chip unabhängig vom (übergangsweise
  // noch vorhandenen) Setup-Guide lädt.
  const { data: phasesData } = useSWRAuth(
    status === "authenticated" ? PHASES_SWR_KEY : null,
    () => listPhases(),
    { revalidateOnFocus: false },
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
  const gapInstanceIds = useMemo(
    () => new Set(gaps.map((gap) => gap.instanceId)),
    [gaps],
  );
  const exceptionConflicts = useMemo(
    () => exceptionConflictData?.conflicts ?? [],
    [exceptionConflictData?.conflicts],
  );
  const staff = useMemo(() => staffData ?? [], [staffData]);
  const students = useMemo(
    () => studentData?.students ?? [],
    [studentData?.students],
  );
  const phases = useMemo(() => phasesData ?? [], [phasesData]);
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
  // Der sichtbare Planungszeitraum: der Zeitraum, in den das sichtbare Datum
  // fällt. Speist Zeitraum-Chip, Bedarfsquellen-Chip und die Serienliste.
  const visiblePeriod = useMemo(
    () => findPeriodForDate(calendarPeriods, dayISO),
    [calendarPeriods, dayISO],
  );
  const templatePeriodID = visiblePeriod?.id ?? assignedPeriods[0]?.id;
  const focusedPeriodID = visiblePeriod?.id ?? assignedPeriods[0]?.id ?? null;
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

  // Bedarfsquellen-Zuordnung (06 §3.2), reine Funktion.
  const demandOrigin = useMemo(
    () => resolveDemandOrigin(phases, visiblePeriod?.id ?? null, dayISO),
    [phases, visiblePeriod, dayISO],
  );

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
        updateUrlParams({ block: null });
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
        updateUrlParams({ block: null });
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

  // Zeitraum-Chip: Sprung in einen anderen Planungszeitraum setzt `d` auf ein
  // Datum innerhalb des Zeitraums (heute, falls es hineinfällt, sonst der
  // Start). In der Serienansicht wechselt so die angezeigte Regeltermin-Liste,
  // in Woche/Monat springt der Kalender.
  const jumpToPeriod = useCallback(
    (period: CalendarPeriod) => {
      const target =
        todayISO >= period.startDate && todayISO <= period.endDate
          ? todayISO
          : period.startDate;
      updateUrlParams({ d: target, block: null });
    },
    [todayISO, updateUrlParams],
  );

  // Der Zeitraum-Chip ist immer sichtbar, sobald mindestens ein Zeitraum
  // existiert (Kriterium 8). Das "Schuljahre & Ferien" der noch verbliebenen
  // Overview/SetupGuide-Karten öffnet weiterhin den Anlegen-Dialog, wenn es
  // noch keinen Zeitraum gibt.
  const handleManagePeriods = useCallback(() => {
    if (calendarPeriods.length === 0) {
      openPeriodCreate();
    }
  }, [calendarPeriods.length, openPeriodCreate]);

  const handleApplyTemplate = useCallback(
    async (template: TimetableTemplate) => {
      // Resolve which calendar period the template's schedules belong to.
      // Falls back to the period currently in scope (visiblePeriod).
      // If none is active, ask the admin to create one before continuing.
      const periodId =
        resolveTemplateCalendarPeriodId(template) ?? visiblePeriod?.id;
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
      visiblePeriod,
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

  const todayDate = parseISODate(todayISO);
  const isOnToday =
    view === "series"
      ? true
      : view === "week"
        ? todayISO >= fromISO && todayISO <= toISO
        : view === "month"
          ? visibleDate.getFullYear() === todayDate.getFullYear() &&
            visibleDate.getMonth() === todayDate.getMonth()
          : visibleDate.getFullYear() === todayDate.getFullYear();
  const showTodayButton = view !== "series" && !isOnToday;

  const demandOriginChip = demandOrigin.href ? (
    <Link href={demandOrigin.href} className="inline-flex">
      <OriginChip label={demandOrigin.label} />
    </Link>
  ) : (
    <OriginChip label={demandOrigin.label} />
  );

  return (
    <div className="flex flex-col gap-4">
      <PlanningContextBar
        title="Betreuungsplan"
        onPrevious={view === "series" ? undefined : handlePrev}
        onNext={view === "series" ? undefined : handleNext}
        previousLabel="Zurück"
        nextLabel="Weiter"
        dateLabel={
          view === "series"
            ? (visiblePeriod?.name ?? undefined)
            : view === "month"
              ? monthLabel
              : view === "year"
                ? yearLabel
                : weekLabel
        }
        onToday={showTodayButton ? goToToday : undefined}
        viewSwitcher={
          <Tabs
            value={viewToTab(view)}
            onValueChange={(v) => setViewParam(v as ViewParam)}
          >
            <TabsList variant="default">
              <TabsTrigger value="woche">Woche</TabsTrigger>
              <TabsTrigger value="monat">Monat</TabsTrigger>
              <TabsTrigger value="serien">Serien</TabsTrigger>
            </TabsList>
          </Tabs>
        }
        actions={
          <TimetableAddMenu
            onAddInstance={openEventCreate}
            onAddSeries={openSeriesCreate}
          />
        }
      >
        {calendarPeriods.length > 0 && (
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
        )}
        {demandOriginChip}
        {shouldLoadGaps && (
          <GapJumpList
            gaps={gaps}
            onJump={(gap) =>
              updateUrlParams({ d: gap.date, block: gap.instanceId })
            }
          />
        )}
      </PlanningContextBar>

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
            monthDate={visibleDate}
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
              gapInstanceIds={gapInstanceIds}
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
                updateUrlParams({ block: instanceId })
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
        defaultDate={quickPrefill?.date ?? dayISO}
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
            updateUrlParams({ block: result.instance.id });
          } else if (result.kind === "series" && result.linkedInstanceId) {
            updateUrlParams({ block: result.linkedInstanceId });
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
