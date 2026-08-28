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
 * "serien") und `block` (Instanz-ID des geöffneten InstanceDetailModal).
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
import { Printer } from "lucide-react";

import { CalendarPeriodModal } from "~/components/timetable/calendar-period-modal";
import { PlanExportModal } from "~/components/planning/plan-export-modal";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ConfirmationModal } from "~/components/ui/modal";
import { OriginChip } from "~/components/ui/origin-chip";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { StatusBadge } from "~/components/ui/status-badge";
import { PlanLegend, type PlanLegendEntry } from "~/components/ui/plan-legend";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { useToast } from "~/contexts/ToastContext";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import {
  ConflictWarningsBanner,
  type ConflictBannerEntry,
} from "~/components/timetable/conflict-warnings-banner";
import { GapJumpList } from "~/components/timetable/gap-jump-list";
import {
  InstanceDetailModal,
  type LifecycleAction,
  type LifecycleActionOptions,
} from "~/components/timetable/instance-detail-modal";
import { cancelledToast } from "~/components/timetable/guardian-notice-toast";
import { StaffPoolSlideOver } from "~/components/timetable/staff-pool-slide-over";
import { timetableSeriesErrorMessage } from "~/components/timetable/event-form/scope-error-message";
import { TimetableAddMenu } from "~/components/timetable/timetable-add-menu";
import { MonthPlannerGrid } from "~/components/timetable/month-planner-grid";
import { PeriodSwitcherDropdown } from "~/components/timetable/period-switcher-dropdown";
import { TemplateList } from "~/components/timetable/template-list";
import { TimetableEventModal } from "~/components/timetable/timetable-event-modal";
import { resolveDemandOrigin } from "~/components/timetable/demand-origin";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { hasPermission } from "~/lib/auth-utils";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useUrlParams } from "~/lib/hooks/use-url-params";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { getSettingValue } from "~/lib/settings-api";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import { fetchStudents } from "~/lib/student-api";
import { staffService } from "~/lib/staff-api";
import { listPhases } from "~/lib/enrollment-phase-api";
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
  DENSITY_TO_HOUR_HEIGHT_PX,
  chunkDateRange,
  firstSchoolDayInPeriod,
  formatFullDayLabel,
  formatWeekLabel,
  formatMonthLabel,
  getMonthDays,
  getMonthRange,
  getWeekRange,
  getWeekdays,
  nextWorkdayISO,
  previousWorkdayISO,
  resolveTemplateCalendarPeriodId,
  toISODate,
  type TimetableView,
  type WeekDensity,
} from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  MaterializeWarning,
  TimetableTemplate,
} from "~/lib/timetable-types";
import {
  TimetableContentSkeleton,
  TimetablePageSkeleton,
} from "./betreuungsplan-skeleton";

const logger = createLogger({ component: "TimetablesPage" });
const PERIODS_SWR_KEY = "database-calendar-periods-list";
const PHASES_SWR_KEY = "timetable-enrollment-phases";
const CONFLICT_ACKS_SWR_KEY = "timetable-conflict-acks";

// Das verbindliche Drei-Parameter-Vokabular (06 §2.1). updateUrlParams baut die
// URL aus dieser Allowlist neu auf, damit fremde Params (?utm_source=…) nicht
// jeden Wochen-/Ansichts-/Block-Wechsel überleben.
const ALLOWED_URL_PARAMS = ["d", "view", "block"] as const;

type ViewParam = "tag" | "woche" | "monat" | "serien";
type PeriodCoverage = "none" | "partial" | "full";

/**
 * Übersetzt den `view`-URL-Wert in den internen Ansichts-Typ. Ungültige Werte
 * fallen still auf die Woche zurück; die Jahresansicht ist nicht mehr
 * verlinkbar (kein `view`-Wert erzeugt sie).
 */
function parseViewParam(raw: string | null): TimetableView {
  if (raw === "tag") return "day";
  if (raw === "monat") return "month";
  if (raw === "serien") return "series";
  return "week";
}

/** Segmentschalter-Wert (deutsches Vokabular) aus dem internen Ansichts-Typ. */
function viewToTab(view: TimetableView): ViewParam {
  if (view === "day") return "tag";
  if (view === "month") return "monat";
  if (view === "series") return "serien";
  return "woche";
}

// Dichte-Umschalter des Wochenrasters (reiner Component-State, nie in der URL).
// Der Bedienknopf sitzt seit dem Chrome-Abbau als kleines Kebab-Menü in der
// PlanningContextBar-Aktionszeile (nur in der Wochenansicht).
const DENSITY_OPTIONS: Array<{ value: WeekDensity; label: string }> = [
  { value: "compact", label: "Kompakt" },
  { value: "normal", label: "Normal" },
  { value: "comfortable", label: "Komfortabel" },
];

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

export function buildPlanningTrackLegend(
  instances: readonly EnrichedInstance[],
): PlanLegendEntry[] {
  const used = new Map<string, PlanLegendEntry & { sortOrder: number }>();
  let hasUnassigned = false;
  for (const instance of instances) {
    if (!instance.planningTrackId || !instance.planningTrackName) {
      hasUnassigned = true;
      continue;
    }
    used.set(instance.planningTrackId, {
      key: instance.planningTrackId,
      label: instance.planningTrackName,
      color: instance.planningTrackColor,
      sortOrder: instance.planningTrackSortOrder ?? Number.MAX_SAFE_INTEGER,
    });
  }
  const entries = [...used.values()]
    .sort(
      (left, right) =>
        left.sortOrder - right.sortOrder ||
        left.label.localeCompare(right.label, "de"),
    )
    .map(({ sortOrder: _sortOrder, ...entry }) => entry);
  if (hasUnassigned) {
    entries.push({ key: "unassigned", label: "Ohne Planungsspur" });
  }
  return entries;
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
  const canExportBetreuungsplan =
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "users:read");
  const canManageSchedules = hasPermission(session, "schedules:manage");
  // Leseansicht (#2283): ohne users:read dürfen die tenantweiten Kinder- und
  // Personallisten nicht geladen werden (403) — Namen kommen dann pro Termin
  // über den Teilnehmer-Endpunkt ins Detail-Modal.
  const canReadTenantRosters = hasPermission(session, "users:read");
  // Anmeldephasen (Bedarf-Chip) hängen am echten Endpunkt-Gate config:read,
  // Perioden-Bootstrap an schedules:create — keine Proxy-Permissions.
  const canReadEnrollmentPhases = hasPermission(session, "config:read");
  const canBootstrapPeriods = hasPermission(session, "schedules:create");
  const canManageCategories = hasPermission(
    session,
    "activities:manage_categories",
  );
  const toast = useToast();
  const tenantMutate = useTenantMutate();
  const tenantPath = useTenantAwarePath();

  // Sichtbares Datum, Ansicht und geöffneter Block werden bei jedem Render aus
  // den Search-Params abgeleitet (keine Offset-Arithmetik, kein weekOffset-State
  // mehr). Ein ungültiges `d` fällt still auf heute zurück, ein unbekanntes
  // `view` auf die Woche.
  const rawDay = params.d;
  const requestedDayISO =
    rawDay !== null && isValidISODate(rawDay) ? rawDay : berlinTodayISO();
  // Leseansicht (#2283, #2621): Nicht-Planende sehen Tag und Woche — ein
  // ?view=monat/serien-Deeplink fällt still auf die Woche zurück.
  const requestedView = parseViewParam(params.view);
  const view: TimetableView =
    canManageSchedules || requestedView === "day" ? requestedView : "week";
  // The monthly calendar and series period lookup retain their requested
  // calendar date. Only the workweek and the single school day snap weekend
  // anchors to Monday.
  const dayISO =
    view === "week" || view === "day"
      ? nextWorkdayISO(requestedDayISO)
      : requestedDayISO;
  const selectedInstanceId = params.block;

  const visibleDate = useMemo(() => parseISODate(dayISO), [dayISO]);
  const todayISO = useMemo(() => berlinTodayISO(), []);
  const todayTargetISO = useMemo(() => nextWorkdayISO(todayISO), [todayISO]);

  const [eventModalOpen, setEventModalOpen] = useState(false);
  // Drucken/Exportieren der angezeigten Woche (#2079).
  const [exportOpen, setExportOpen] = useState(false);
  // Personalpool (#1884): Zielblock, für den der Pool-SlideOver offen ist.
  // Solange er offen ist, wird das Detail-Modal suspendiert (Kit-Modal
  // z-9999 würde den SlideOver sonst verdecken; Muster aus PR #1962).
  const [poolInstance, setPoolInstance] = useState<EnrichedInstance | null>(
    null,
  );
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
  // 60/90/120 px per hour). Local-only, never synced to URL (06 §2.1). Der
  // Bedienknopf sitzt als Kebab-Menü in der PlanningContextBar-Aktionszeile.
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
  // Tagesnavigation springt von Schultag zu Schultag: Freitag führt vorwärts
  // auf Montag, Montag rückwärts auf Freitag. Das Wochenende hat im
  // Betreuungsplan keine Tage, also darf es auch keinen leeren Halt geben.
  const goToDay = useCallback(
    (direction: 1 | -1) => {
      const next = parseISODate(dayISO);
      next.setDate(next.getDate() + 1);
      const target =
        direction === 1
          ? nextWorkdayISO(toISODate(next))
          : previousWorkdayISO(dayISO);
      updateUrlParams({ d: target, block: null });
    },
    [dayISO, updateUrlParams],
  );
  const goToToday = useCallback(
    () =>
      updateUrlParams({
        d: view === "week" || view === "day" ? todayTargetISO : todayISO,
        block: null,
      }),
    [todayISO, todayTargetISO, updateUrlParams, view],
  );

  const handlePrev = useCallback(() => {
    if (view === "day") goToDay(-1);
    else if (view === "week") goToWeek(-7);
    else if (view === "month") goToMonth(-1);
  }, [view, goToDay, goToWeek, goToMonth]);
  const handleNext = useCallback(() => {
    if (view === "day") goToDay(1);
    else if (view === "week") goToWeek(7);
    else if (view === "month") goToMonth(1);
  }, [view, goToDay, goToWeek, goToMonth]);

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
      updateUrlParams({ view: null, d: nextWorkdayISO(dateISO), block: null });
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
  // lädt keine Instanzen. Die Jahresansicht ist ersatzlos entfallen.
  const fromISO = toISODate(weekRange.from);
  const monthRange = useMemo(() => getMonthRange(visibleDate), [visibleDate]);
  const monthDays = useMemo(() => getMonthDays(visibleDate), [visibleDate]);
  // Der Betreuungsplan folgt derselben Betriebswoche wie Dienstplan und
  // Vertretung: geplant und angezeigt wird nur Mo–Fr. Die Monatsansicht
  // bleibt ein vollständiger Kalender, daher behält sie ihr eigenes Fenster.
  const weekDays = useMemo(
    () => getWeekdays(weekRange.from).slice(0, 5),
    [weekRange.from],
  );
  const weekDayISOs = useMemo(() => weekDays.map(toISODate), [weekDays]);
  const workweekToISO = weekDayISOs[4]!;
  // Die Tagesansicht lädt genau ihren Tag — ein Wochenfenster würde Blöcke
  // holen, die sie gar nicht zeigt, und den Konfliktbanner mit fremden Tagen
  // füllen.
  const fetchFromISO =
    view === "month"
      ? toISODate(monthRange.from)
      : view === "day"
        ? dayISO
        : fromISO;
  const fetchToISO =
    view === "month"
      ? toISODate(monthRange.to)
      : view === "day"
        ? dayISO
        : workweekToISO;
  const dayOnly = useMemo(() => [visibleDate], [visibleDate]);
  const periodContextDays =
    view === "month" ? monthDays : view === "day" ? dayOnly : weekDays;
  const periodContextDayISOs = useMemo(
    () => periodContextDays.map(toISODate),
    [periodContextDays],
  );
  const weekLabel = useMemo(
    () => formatWeekLabel(weekRange.from, weekDays[4]!),
    [weekRange.from, weekDays],
  );
  const monthLabel = useMemo(
    () => formatMonthLabel(visibleDate),
    [visibleDate],
  );
  const dayLabel = useMemo(
    () => formatFullDayLabel(visibleDate),
    [visibleDate],
  );

  // OGS-Schließtage (#2032). Ein SWR-Zustand liefert sichtbares Fenster,
  // vollständige Zeiträume und den Ladestatus: Raster und Termin-Dialog sehen
  // dadurch immer denselben Datenstand.
  const {
    closingDays,
    closingDayRanges,
    isLoading: closingDaysLoading,
  } = useClosingDaysState(fetchFromISO, fetchToISO);

  const swrKey = `timetable-${view}-${fetchFromISO}-${fetchToISO}`;
  const shouldLoadInstances = view !== "series";
  // Lücken sind vorwärtsgerichtet: der Endpunkt lehnt ein vergangenes `date`
  // ab, also den Fensterstart auf heute klemmen und vollständig vergangene
  // Fenster überspringen. Nur die Wochenansicht lädt Lücken ("wie heute",
  // Spec 06 §5.2): GET /gaps begrenzt das Fenster hart auf 14 Tage und lehnt
  // das Monatsfenster mit 400 ab — ein leerer Zähler wäre in der
  // Monatsansicht eine falsche "Keine Lücken"-Aussage.
  // The workweek has no Saturday/Sunday destination. On weekends, start at
  // Monday so the gap list cannot offer a retained legacy weekend block that
  // the week view deliberately does not expose.
  const gapsFromISO =
    fetchFromISO < todayTargetISO ? todayTargetISO : fetchFromISO;
  // Lücken sind Planungswerkzeug — in der Leseansicht weder Chip noch Fetch.
  const shouldLoadGaps =
    canManageSchedules && view === "week" && fetchToISO >= todayTargetISO;
  const gapsSWRKey = `timetable-gaps-${gapsFromISO}-${fetchToISO}`;
  const { data, error, isLoading } = useSWRAuth(
    status === "authenticated" && shouldLoadInstances ? swrKey : null,
    () => timetableService.getWeek(fetchFromISO, fetchToISO),
  );
  const { data: gapsData, error: gapsError } = useSWRAuth(
    status === "authenticated" && shouldLoadGaps ? gapsSWRKey : null,
    () => timetableService.getGaps(gapsFromISO, fetchToISO),
  );
  const { data: staffData } = useSWRAuth(
    status === "authenticated" && canReadTenantRosters
      ? "timetable-staff-list"
      : null,
    () => staffService.getAllStaff(),
  );
  const { data: studentData } = useSWRAuth(
    status === "authenticated" && canReadTenantRosters
      ? "timetable-student-list"
      : null,
    () => fetchStudents({ page_size: 500 }),
  );
  // Bedarfsquellen-Chip (06 §3.2): die Anmeldephasen für die clientseitige
  // Zuordnung.
  const { data: phasesData, error: phasesError } = useSWRAuth(
    // Leseansicht (#2283): Anmeldephasen sind admin-gegated; ohne
    // schedules:manage würde der Abruf nur 403-Rauschen erzeugen.
    status === "authenticated" && canReadEnrollmentPhases
      ? PHASES_SWR_KEY
      : null,
    () => listPhases(),
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
  const { data: settingsSchema, isLoading: settingsSchemaLoading } =
    useSettingsSchema(status === "authenticated", {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    });
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;

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
  const planQualityErrorMessage = gapsErrorMessage
    ? `Personal-Lücken konnten nicht geprüft werden: ${gapsErrorMessage}`
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
    // Leseansicht (#2283): ohne schedules:create würde der Bootstrap ohnehin
    // mit 403 abgewiesen — gar nicht erst versuchen.
    if (!canBootstrapPeriods) return;
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
    canBootstrapPeriods,
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
  const planningTrackLegend = useMemo(
    () => buildPlanningTrackLegend(instances),
    [instances],
  );
  const gaps = useMemo(() => gapsData?.gaps ?? [], [gapsData?.gaps]);
  const gapInstanceIds = useMemo(
    () => new Set(gaps.map((gap) => gap.instanceId)),
    [gaps],
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
  const planningDisabledDateISOs = useMemo(
    () =>
      new Set(
        periodAssignments
          .filter((assignment) => assignment.period === null)
          .map((assignment) => assignment.date),
      ),
    [periodAssignments],
  );
  const weekPeriodAssignments = useMemo(
    () => mapPeriodsForDates(calendarPeriods, weekDayISOs),
    [calendarPeriods, weekDayISOs],
  );
  const weekPeriodCoverage = useMemo<PeriodCoverage>(() => {
    const coveredDayCount = weekPeriodAssignments.filter(
      (assignment) => assignment.period !== null,
    ).length;
    if (coveredDayCount === 0) return "none";
    if (coveredDayCount === weekPeriodAssignments.length) return "full";
    return "partial";
  }, [weekPeriodAssignments]);
  // Tagesansicht (#2621): der sichtbare Tag ist genau ein Datum, deshalb
  // reicht ein direkter Blick in die Zeitraum-Zuordnung — und der Schließtag
  // wird zum benennbaren Leerzustand statt zu einem stillen leeren Raster.
  const dayHasPeriodCoverage = useMemo(
    () => findPeriodForDate(calendarPeriods, dayISO) !== null,
    [calendarPeriods, dayISO],
  );
  const dayClosingReason = closingDays.get(dayISO) ?? null;
  // Der sichtbare Planungszeitraum: der Zeitraum, in den das sichtbare Datum
  // fällt. Speist Zeitraum-Chip, Bedarfsquellen-Chip und die Serienliste.
  const visiblePeriod = useMemo(
    () => findPeriodForDate(calendarPeriods, dayISO),
    [calendarPeriods, dayISO],
  );
  const templatePeriodID = visiblePeriod?.id ?? assignedPeriods[0]?.id;
  const focusedPeriodID = templatePeriodID ?? null;
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
  // Konflikte (#2139): fensterweite Personen-Doppelbelegungen, aggregiert
  // nach Fingerprint (beide beteiligten Termine tragen denselben). Die
  // Quittierung ist Nutzerdatenzustand und kommt pro Konto vom Backend.
  const { data: ackData } = useSWRAuth(
    status === "authenticated" && canManageSchedules
      ? CONFLICT_ACKS_SWR_KEY
      : null,
    () => timetableService.getConflictAcks(),
  );
  const ackedFingerprints = useMemo(() => new Set(ackData ?? []), [ackData]);

  const conflictEntries = useMemo(() => {
    const byFingerprint = new Map<string, ConflictBannerEntry>();
    for (const inst of instances) {
      for (const warning of inst.conflictWarnings) {
        if (!warning.fingerprint) continue;
        const existing = byFingerprint.get(warning.fingerprint);
        if (existing) {
          if (!existing.instances.some((item) => item.id === inst.id)) {
            existing.instances.push({ id: inst.id, title: inst.title });
          }
          continue;
        }
        const personName =
          (warning.kind === "student"
            ? studentNames.get(warning.resourceId)
            : staffNames.get(warning.resourceId)) ??
          (warning.kind === "student" ? "Kind" : "Personal");
        byFingerprint.set(warning.fingerprint, {
          fingerprint: warning.fingerprint,
          kind: warning.kind,
          personName,
          dateISO: inst.date,
          overlapStart: warning.overlapStart,
          overlapEnd: warning.overlapEnd,
          instances: [{ id: inst.id, title: inst.title }],
        });
      }
    }
    return [...byFingerprint.values()];
  }, [instances, staffNames, studentNames]);

  const openConflicts = useMemo(
    () =>
      conflictEntries.filter(
        (entry) => !ackedFingerprints.has(entry.fingerprint),
      ),
    [conflictEntries, ackedFingerprints],
  );
  const hiddenConflicts = useMemo(
    () =>
      conflictEntries.filter((entry) =>
        ackedFingerprints.has(entry.fingerprint),
      ),
    [conflictEntries, ackedFingerprints],
  );

  // Die Leseansicht zeigt keine Konflikthinweise. Für Planende verschwinden
  // quittierte Konflikte auch von den Terminkarten und aus dem Detail-Modal —
  // sonst zählt der Banner anders als das Raster markiert.
  const visibleInstances = useMemo(
    () =>
      instances.map((inst) => {
        if (!canManageSchedules && inst.conflictWarnings.length > 0) {
          return { ...inst, conflictWarnings: [] };
        }
        return inst.conflictWarnings.some(
          (warning) =>
            warning.fingerprint && ackedFingerprints.has(warning.fingerprint),
        )
          ? {
              ...inst,
              conflictWarnings: inst.conflictWarnings.filter(
                (warning) =>
                  !warning.fingerprint ||
                  !ackedFingerprints.has(warning.fingerprint),
              ),
            }
          : inst;
      }),
    [instances, ackedFingerprints, canManageSchedules],
  );

  const handleHideConflict = useCallback(
    async (entry: ConflictBannerEntry) => {
      try {
        await timetableService.acknowledgeConflict(entry.fingerprint);
        await tenantMutate(CONFLICT_ACKS_SWR_KEY);
      } catch (err) {
        logger.error("conflict_ack_failed", {
          fingerprint: entry.fingerprint,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error("Konflikt konnte nicht ausgeblendet werden");
      }
    },
    [tenantMutate, toast],
  );

  // Sammel-Quittierung: jeder offene Konflikt wird einzeln über seinen
  // Fingerprint quittiert — kein Kategorien-Schalter, Neues taucht wieder auf.
  const handleHideAllConflicts = useCallback(async () => {
    try {
      await Promise.all(
        openConflicts.map((entry) =>
          timetableService.acknowledgeConflict(entry.fingerprint),
        ),
      );
      await tenantMutate(CONFLICT_ACKS_SWR_KEY);
    } catch (err) {
      logger.error("conflict_ack_all_failed", {
        count: openConflicts.length,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Konflikte konnten nicht ausgeblendet werden");
      // Teil-Erfolge sichtbar machen: was durchging, ist quittiert.
      await tenantMutate(CONFLICT_ACKS_SWR_KEY);
    }
  }, [openConflicts, tenantMutate, toast]);

  const handleUnhideConflict = useCallback(
    async (entry: ConflictBannerEntry) => {
      try {
        await timetableService.unacknowledgeConflict(entry.fingerprint);
        await tenantMutate(CONFLICT_ACKS_SWR_KEY);
      } catch (err) {
        logger.error("conflict_unack_failed", {
          fingerprint: entry.fingerprint,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error("Konflikt konnte nicht wieder angezeigt werden");
      }
    },
    [tenantMutate, toast],
  );

  const handleJumpToConflict = useCallback(
    (entry: ConflictBannerEntry, instanceId: string) => {
      updateUrlParams({ d: entry.dateISO, block: instanceId });
    },
    [updateUrlParams],
  );

  // Sind alle Konflikte quittiert, verschwindet der Banner komplett ("nicht
  // bei jedem Aufruf erneut erscheinen", #2139). Der Wiedereinstieg zu den
  // Ausgeblendeten liegt im ⋮-Menü der Kopfzeile und öffnet dieses Panel.
  const [hiddenConflictsPanelOpen, setHiddenConflictsPanelOpen] =
    useState(false);

  const selectedInstance = useMemo(
    () =>
      visibleInstances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [visibleInstances, selectedInstanceId],
  );
  const isInstanceDataLoading = shouldLoadInstances && isLoading && !data;
  const weekEmptyState = useMemo(() => {
    if (instances.length > 0 || error) return undefined;
    if (weekPeriodCoverage === "partial") {
      return {
        title: "Einige Tage liegen außerhalb des Planungszeitraums",
        description: canManageSchedules
          ? "An diesen Tagen können Sie keine Termine planen."
          : "An diesen Tagen können Sie keine Termine planen. Geplant wird von den Admins Ihrer Schule.",
      };
    }
    if (weekPeriodCoverage === "full") {
      return {
        title: "Diese Woche hat noch keine Termine",
        description: canManageSchedules
          ? "Planen Sie Angebote als Regeltermin oder legen Sie einen einzelnen Termin an."
          : "Für diese Woche ist noch nichts geplant.",
      };
    }
    return {
      title: "Diese Woche hat keinen Planungszeitraum",
      description: canManageSchedules
        ? "Legen Sie zuerst einen aktiven Planungszeitraum an."
        : "Für diese Woche ist noch nichts geplant.",
    };
  }, [canManageSchedules, error, instances.length, weekPeriodCoverage]);

  // Bedarfsquellen-Zuordnung (06 §3.2), reine Funktion.
  const demandOrigin = useMemo(
    () => resolveDemandOrigin(phases, visiblePeriod?.id ?? null, dayISO),
    [phases, visiblePeriod, dayISO],
  );

  const handleLifecycle = useCallback(
    async (action: LifecycleAction, options?: LifecycleActionOptions) => {
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
          await timetableService.complete(selectedInstance.id, []);
          toast.success("Aktivität beendet");
        } else if (action === "reopen") {
          await timetableService.reopen(selectedInstance.id);
          toast.success("Aktivität wieder geöffnet");
        } else {
          // Plain cancels keep the single-argument call; the notice rides
          // along only when the dialog produced one (#2601).
          const res = options?.guardianNotice
            ? await timetableService.cancel(
                selectedInstance.id,
                undefined,
                options.guardianNotice,
              )
            : await timetableService.cancel(selectedInstance.id);
          toast.success(
            cancelledToast("Aktivität abgesagt", res.guardianNotice),
          );
        }
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
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
    [selectedInstance, swrKey, gapsSWRKey, tenantMutate, toast],
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
    [swrKey, tenantMutate, toast],
  );

  const handleDeleteCancelledInstance = useCallback(
    async (instance: EnrichedInstance) => {
      try {
        await timetableService.deleteCancelled(instance.id);
        toast.success("Termin gelöscht");
        updateUrlParams({ block: null });
        await tenantMutate(swrKey);
        await tenantMutate(gapsSWRKey);
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
    [gapsSWRKey, swrKey, tenantMutate, toast, updateUrlParams],
  );

  const handleDeleteFollowingInstances = useCallback(
    async (instance: EnrichedInstance) => {
      if (!instance.activityGroupId) return;
      // Das Modal blendet die Option für vergangene Termine aus; bleibt es
      // über Mitternacht offen, würde das Backend mit einer englischen
      // Fehlermeldung antworten (effective_date must not be in the past).
      if (instance.date < berlinTodayISO()) {
        toast.error(
          "Ein Regeltermin kann nur ab heute beendet werden. Vergangene Termine lassen sich nur einzeln löschen.",
        );
        throw new Error("effective_date in the past");
      }
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
      } catch (err) {
        logger.error("template_end_failed", {
          template_id: instance.activityGroupId,
          instance_id: instance.id,
          effective_date: instance.date,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          timetableSeriesErrorMessage(
            err,
            "Folgetermine konnten nicht gelöscht werden",
          ),
        );
        throw err;
      }
    },
    [
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
    [gapsSWRKey, swrKey, templatePeriodID, tenantMutate, toast],
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
  // Start; Wochenendtage werden auf den nächsten Schultag im Zeitraum
  // gehoben). In der Serienansicht wechselt so die angezeigte
  // Regeltermin-Liste, in Woche/Monat springt der Kalender.
  const jumpToPeriod = useCallback(
    (period: CalendarPeriod) => {
      const target =
        todayISO >= period.startDate && todayISO <= period.endDate
          ? todayISO
          : period.startDate;
      updateUrlParams({
        d: firstSchoolDayInPeriod(period.startDate, period.endDate, target),
        block: null,
      });
    },
    [todayISO, updateUrlParams],
  );

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
    gapsSWRKey,
    swrKey,
    templatePeriodID,
    tenantMutate,
    toast,
  ]);

  // While the session or the settings schema loads we cannot tell yet
  // whether the feature is enabled or what the caller may do. The
  // PlanningContextBar (title, navigation) renders immediately regardless —
  // only the calendar-grid content below falls back to its skeleton, and
  // permission-dependent toolbar bits (view switcher, export, "Neu") stay
  // hidden until they resolve (hasPermission(undefined, …) is false while
  // loading).
  const showSkeleton = status === "loading" || settingsSchemaLoading;

  if (!showSkeleton && timetableDisabled) {
    return (
      <PlanningDisabledState
        pageTitle="Betreuungsplan"
        heading="Betreuungsplan ist deaktiviert"
        description="Der Betreuungsplan ist für diese Schule ausgeschaltet. Er kann in den Einstellungen unter „Betrieb“ wieder aktiviert werden."
        testId="timetable-disabled-state"
      />
    );
  }

  const todayDate = parseISODate(view === "month" ? todayISO : todayTargetISO);
  const isOnToday =
    view === "series"
      ? true
      : view === "day"
        ? dayISO === todayTargetISO
        : view === "week"
          ? todayTargetISO >= fromISO && todayTargetISO <= workweekToISO
          : visibleDate.getFullYear() === todayDate.getFullYear() &&
            visibleDate.getMonth() === todayDate.getMonth();
  const showTodayButton = view !== "series" && !isOnToday;

  // Solange die Phasen laden, keine bestätigte Negativ-Aussage ("keine
  // Anmeldung verknüpft") aus dem leeren Zwischenstand ableiten — neutraler
  // Platzhalter. Schlägt der Fetch fehl, entfällt der Chip (keine Aussage
  // ist ehrlicher als eine falsche).
  // Rahmenlos in der Kontextzeile (#2031): dort stehen alle Angaben als ruhiger
  // Text, eine gerahmte Pille daneben wäre das dritte Formvokabular in einer
  // Zeile. Als Link bleibt sie unterstrichen bei Hover, sonst reiner Text.
  const originChipClassName = "border-transparent bg-transparent px-0";
  const demandOriginChip =
    !canReadEnrollmentPhases || phasesError ? null : phasesData ===
      undefined ? (
      <OriginChip
        label="Bedarf wird ermittelt …"
        className={originChipClassName}
      />
    ) : demandOrigin.href ? (
      <Link
        href={tenantPath(demandOrigin.href)}
        className="inline-flex hover:underline"
      >
        <OriginChip
          label={demandOrigin.label}
          className={originChipClassName}
        />
      </Link>
    ) : (
      <OriginChip label={demandOrigin.label} className={originChipClassName} />
    );

  // Dichte-Umschalter als kleines Kebab-Menü in der Aktionszeile (nur in der
  // Wochenansicht, weil nur das Wochenraster Zeilenhöhen kennt). Reiner
  // Component-State, kein URL-Parameter (06 §2.1).
  const densityMenuItems: OverflowMenuEntry[] = [
    { kind: "header", label: "Zeilenhöhe" },
    ...DENSITY_OPTIONS.map((opt) => ({
      kind: "radio" as const,
      label: opt.label,
      checked: density === opt.value,
      onClick: () => setDensity(opt.value),
    })),
  ];

  // Wiedereinstieg zu quittierten Konflikten (#2139): der Banner ist bei
  // "alles quittiert" bewusst weg, also führt dieser Menü-Eintrag zurück zum
  // ausgeblendeten Bestand. Nur angeboten, wenn es welche gibt.
  const hiddenConflictsMenuItems: OverflowMenuEntry[] =
    shouldLoadInstances && hiddenConflicts.length > 0
      ? [
          { kind: "header", label: "Konflikte" },
          {
            label: `Ausgeblendete Konflikte anzeigen`,
            badge: hiddenConflicts.length,
            onClick: () => setHiddenConflictsPanelOpen(true),
          },
        ]
      : [];

  // Leerzustand (Kriterium 5): solange kein Planungszeitraum existiert, zeigt
  // die Kalenderfläche eine Hinweiskarte statt des Rasters. Der stille
  // bootstrap() legt in der Regel einen Default-Zeitraum an; bei fehlender
  // Berechtigung (403) bleibt es leer und der Hinweis führt zum Anlegen-Dialog.
  const showEmptyPeriodState =
    !periodsLoading && periods !== undefined && calendarPeriods.length === 0;

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
              : view === "day"
                ? dayLabel
                : weekLabel
        }
        onToday={showTodayButton ? goToToday : undefined}
        viewSwitcher={
          // Tag und Woche stehen allen offen (#2621); Monat und Serien sind
          // Planungsansichten und bleiben Planenden vorbehalten.
          <Tabs
            value={viewToTab(view)}
            onValueChange={(v) => setViewParam(v as ViewParam)}
          >
            <TabsList variant="default">
              <TabsTrigger value="tag">Tag</TabsTrigger>
              <TabsTrigger value="woche">Woche</TabsTrigger>
              {canManageSchedules && (
                <>
                  <TabsTrigger value="monat">Monat</TabsTrigger>
                  <TabsTrigger value="serien">Serien</TabsTrigger>
                </>
              )}
            </TabsList>
          </Tabs>
        }
        actions={
          <>
            {canManageSchedules && hiddenConflictsMenuItems.length > 0 ? (
              // Mit ausgeblendeten Konflikten trägt das Menü den Wieder-
              // einstieg und muss überall erreichbar sein (auch mobil und in
              // der Monatsansicht); die Zeilenhöhe bleibt an die Wochenansicht
              // gebunden.
              <OverflowMenu
                ariaLabel="Weitere Optionen"
                items={[
                  ...(view === "week" || view === "day"
                    ? densityMenuItems
                    : []),
                  ...hiddenConflictsMenuItems,
                ]}
              />
            ) : (
              canManageSchedules &&
              (view === "week" || view === "day") && (
                // Die Zeilenhöhe des Wochenrasters ist eine Desktop-Feinjustage:
                // mobil wird das Raster ohnehin tageweise gezeigt, und der Knopf
                // war mit den drei Ansichts-Tabs und "Neu" zusammen breiter als
                // die Kopfzeile — das Datum wurde dadurch auf null gequetscht.
                <span className="hidden sm:contents">
                  <OverflowMenu
                    ariaLabel="Zeilenhöhe"
                    items={densityMenuItems}
                  />
                </span>
              )
            )}
            {/* Drucken/Exportieren (#2079): gehört auf die Fläche, weil der
                Export die Woche meint, die gerade zu sehen ist. Unter sm nur
                das Symbol — die Kopfzeile trägt hier schon drei Ansichts-Tabs
                und "Neu". */}
            {canManageSchedules && canExportBetreuungsplan && (
              <Button
                type="button"
                variant="outline"
                size="md"
                aria-label="Betreuungsplan drucken oder exportieren"
                className="max-sm:h-8 max-sm:w-8 max-sm:justify-center max-sm:p-0"
                onClick={() => setExportOpen(true)}
              >
                <Printer className="h-4 w-4 shrink-0 sm:mr-1.5" aria-hidden />
                <span className="hidden whitespace-nowrap sm:inline">
                  Drucken
                </span>
              </Button>
            )}
            {/* Leseansicht (#2283): ohne schedules:manage gibt es kein "Neu";
                das Badge erklärt den Unterschied zum Admin-Bildschirm. */}
            {canManageSchedules ? (
              <TimetableAddMenu
                onAddInstance={openEventCreate}
                onAddSeries={openSeriesCreate}
              />
            ) : (
              <StatusBadge
                label="Nur ansehen"
                tone="gray"
                title="Du kannst den Betreuungsplan ansehen. Ändern können ihn nur Admins."
              />
            )}
          </>
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
            canManage={canManageSchedules}
          />
        )}
        {demandOriginChip}
        {shouldLoadGaps && !gapsError && (
          <GapJumpList
            gaps={gaps}
            state={gapsData === undefined ? "loading" : "ready"}
            onJump={(gap) =>
              updateUrlParams({ d: gap.date, block: gap.instanceId })
            }
          />
        )}
      </PlanningContextBar>

      {showEmptyPeriodState ? (
        <div className={`${timetableSurface} p-10 text-center`}>
          <MotoConceptIcon
            concept="closingDays"
            size={42}
            className="mx-auto"
          />
          <h2 className="mt-4 text-base font-semibold text-gray-900">
            Noch kein Planungszeitraum
          </h2>
          <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-600">
            {canManageSchedules
              ? "Lege einen Planungszeitraum an, um Angebote und Termine im Betreuungsplan zu planen."
              : "Es gibt noch keinen Planungszeitraum. Ein Admin muss ihn zuerst anlegen."}
          </p>
          {canManageSchedules && (
            <div className="mt-4 flex justify-center">
              <Button
                type="button"
                variant="primary"
                onClick={openPeriodCreate}
              >
                Planungszeitraum anlegen
              </Button>
            </div>
          )}
        </div>
      ) : (
        <>
          {canManageSchedules && shouldLoadInstances && (
            <ConflictWarningsBanner
              openConflicts={openConflicts}
              hiddenConflicts={hiddenConflicts}
              periodLabel={
                view === "month"
                  ? "in diesem Monat"
                  : view === "day"
                    ? "an diesem Tag"
                    : "diese Woche"
              }
              onHide={handleHideConflict}
              onHideAll={handleHideAllConflicts}
              onUnhide={handleUnhideConflict}
              onJump={handleJumpToConflict}
              revealHidden={hiddenConflictsPanelOpen}
              onDismissHiddenPanel={() => setHiddenConflictsPanelOpen(false)}
            />
          )}

          {shouldLoadInstances &&
            !isInstanceDataLoading &&
            planningTrackLegend.length > 0 && (
              <PlanLegend
                entries={planningTrackLegend}
                aria-label="Planungsspuren im sichtbaren Betreuungsplan"
              />
            )}

          {view === "month" &&
            (showSkeleton || isInstanceDataLoading ? (
              <TimetableContentSkeleton view="month" />
            ) : (
              <MonthPlannerGrid
                days={monthDays}
                monthDate={visibleDate}
                instances={visibleInstances}
                todayISO={todayISO}
                closingDays={closingDays}
                onDayClick={openWeekForDay}
                onInstanceClick={handleSelectInstance}
              />
            ))}

          {view === "day" && (
            <>
              {showSkeleton || isInstanceDataLoading ? (
                <TimetableContentSkeleton view="day" />
              ) : (
                <WeeklyCalendarGrid
                  weekDays={dayOnly}
                  instances={visibleInstances}
                  selectedId={selectedInstanceId}
                  onInstanceClick={handleSelectInstance}
                  onSlotClick={canManageSchedules ? openQuickCreate : undefined}
                  gapInstanceIds={gapInstanceIds}
                  planningDisabledDateISOs={planningDisabledDateISOs}
                  closingDays={closingDays}
                  todayISO={todayISO}
                  dayStartHour={dayStartHour}
                  dayEndHour={dayEndHour}
                  hourHeightPx={hourHeightPx}
                  showDayHeader
                  emptyState={
                    instances.length === 0 && !error
                      ? {
                          // Der Schließtag ist der stärkste Grund und steht
                          // deshalb vorn. Ohne Planungsrecht sagt der Text
                          // nicht "Planungszeitraum": das ist ein Begriff aus
                          // der Planung und für Lesende keine Auskunft.
                          title: dayClosingReason
                            ? "Schließtag"
                            : dayHasPeriodCoverage || !canManageSchedules
                              ? "An diesem Tag ist nichts geplant"
                              : "Dieser Tag hat keinen Planungszeitraum",
                          description: dayClosingReason
                            ? `Die Betreuung entfällt: ${dayClosingReason}`
                            : // Leseansicht (#2283): keine Handlungs-
                              // aufforderung an Leute, die nicht planen dürfen
                              // — stattdessen, wer den Tag plant.
                              canManageSchedules
                              ? dayHasPeriodCoverage
                                ? "Planen Sie Angebote als Regeltermin oder legen Sie einen einzelnen Termin an."
                                : "Legen Sie zuerst einen aktiven Planungszeitraum an."
                              : "Geplant wird von den Admins Ihrer Schule.",
                        }
                      : undefined
                  }
                />
              )}
            </>
          )}

          {view === "week" && (
            <>
              {showSkeleton || isInstanceDataLoading ? (
                <TimetableContentSkeleton view="week" />
              ) : (
                <WeeklyCalendarGrid
                  weekDays={weekDays}
                  instances={visibleInstances}
                  selectedId={selectedInstanceId}
                  onInstanceClick={handleSelectInstance}
                  onSlotClick={canManageSchedules ? openQuickCreate : undefined}
                  gapInstanceIds={gapInstanceIds}
                  planningDisabledDateISOs={planningDisabledDateISOs}
                  closingDays={closingDays}
                  todayISO={todayISO}
                  dayStartHour={dayStartHour}
                  dayEndHour={dayEndHour}
                  hourHeightPx={hourHeightPx}
                  showDayHeader
                  emptyState={weekEmptyState}
                />
              )}
            </>
          )}

          {view === "series" && (
            <>
              {showSkeleton || templatesLoading ? (
                <TimetableContentSkeleton view="series" />
              ) : (
                <TemplateList
                  templates={templates}
                  canManage={canManageSchedules}
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
        </>
      )}

      <InstanceDetailModal
        instance={selectedInstance}
        onClose={() => handleSelectInstance(null)}
        onLifecycleAction={handleLifecycle}
        canManage={canManageSchedules}
        fetchParticipantNames={!canReadTenantRosters}
        onDeleteCancelled={
          canManageSchedules ? handleDeleteCancelledInstance : undefined
        }
        onDeleteFollowing={
          canManageSchedules ? handleDeleteFollowingInstances : undefined
        }
        onEdit={
          canManageSchedules
            ? (instance) => {
                setEditingInstance(instance);
                setEditingTemplate(null);
                setConvertingInstance(null);
                setEventDefaultRepeat("none");
                setEventModalOpen(true);
              }
            : undefined
        }
        onRepeat={
          canManageSchedules
            ? (instance) => {
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
              }
            : undefined
        }
        staffNames={staffNames}
        studentNames={studentNames}
        onAttendancePatch={
          canManageSchedules ? handleAttendancePatch : undefined
        }
        editDeferred={false}
        suspended={eventModalOpen || poolInstance !== null}
        onOpenPool={setPoolInstance}
        canManageStaffPool={canManageSchedules}
      />

      <StaffPoolSlideOver
        open={poolInstance !== null}
        instance={poolInstance}
        canManage={canManageSchedules}
        onClose={() => setPoolInstance(null)}
        onMoved={() => {
          void tenantMutate(swrKey);
          void tenantMutate(gapsSWRKey);
        }}
      />

      <TimetableEventModal
        canCheckShiftCoverage={canCheckShiftCoverage}
        canManageCategories={canManageCategories}
        canManagePlanningTracks={canManageSchedules}
        isOpen={eventModalOpen}
        onClose={() => {
          setEventModalOpen(false);
          setEditingInstance(null);
          setEditingTemplate(null);
          setConvertingInstance(null);
          setEventDefaultRepeat("none");
          setQuickPrefill(null);
        }}
        defaultDate={nextWorkdayISO(quickPrefill?.date ?? dayISO)}
        closingDayRanges={closingDayRanges}
        closingDaysLoading={closingDaysLoading}
        weekFrom={fromISO}
        weekTo={workweekToISO}
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

      {/* Erst bei Bedarf gemountet, wie die übrigen Dialoge dieser Fläche:
          ein dauerhaft eingehängter Dialog zieht seinen Kontext (Toasts) auch
          dann in jeden Test dieser Seite, wenn ihn niemand öffnet. */}
      {canExportBetreuungsplan && exportOpen && (
        <PlanExportModal
          isOpen
          plan="betreuungsplan"
          weekDay={dayISO}
          isWeekOnScreen={view === "week"}
          canExportInternal={canManageSchedules}
          onClose={() => setExportOpen(false)}
        />
      )}

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
