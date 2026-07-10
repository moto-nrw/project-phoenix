"use client";

/**
 * Vertretungsplan (#1840), the weekly substitution view.
 *
 * Deliberately separate from the Betreuungsplan (/timetables): this surface
 * shows the materialized week as a deviation over the base plan and lets an
 * admin record absences, substitutes, cancellations, and deliberately-open
 * positions for a week or a single day, without touching the half-year
 * template. It reuses the WeeklyCalendarGrid and the timetable API; the
 * per-block actions live in the SubstitutionSlideOver.
 */

import {
  CalendarOff,
  ChevronLeft,
  ChevronRight,
  TriangleAlert,
  UserX,
} from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { Suspense, useCallback, useEffect, useMemo } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { PageHeader } from "~/components/ui/page-header/PageHeader";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { SubstitutionSlideOver } from "~/components/timetable/substitution-slide-over";
import { TimetableStatCard } from "~/components/timetable/timetable-stat-card";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { useToast } from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { createLogger } from "~/lib/logger";
import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
} from "~/lib/settings-api";
import { staffService } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import {
  formatWeekLabel,
  getWeekRange,
  getWeekdays,
} from "~/lib/timetable-helpers";
import type {
  ApplyDeviationsInput,
  EnrichedInstance,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "Vertretungsplan" });
const HOUR_HEIGHT_PX = 90;

function parseIntParam(value: string | null, fallback: number): number {
  if (value === null) return fallback;
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) ? n : fallback;
}

function shiftDayISO(iso: string, delta: number): string {
  const d = parseISODate(iso);
  d.setDate(d.getDate() + delta);
  return toISODate(d);
}

function VertretungsplanContent() {
  const { status } = useSession();
  const searchParams = useSearchParams();
  const toast = useToast();
  const tenantMutate = useTenantMutate();
  const { dayStartHour, dayEndHour } = useTimetableDayHours();

  const weekOffset = parseIntParam(searchParams.get("week"), 0);
  const selectedInstanceId = searchParams.get("instance");
  // "Woche oder einen Tag" (issue #1840): the day view narrows the grid to a
  // single day; both still fetch the surrounding week.
  const view = searchParams.get("view") === "day" ? "day" : "week";
  const dayISO = searchParams.get("day") ?? berlinTodayISO();

  const range = useMemo(
    () =>
      view === "day"
        ? getWeekRange(parseISODate(dayISO), 0)
        : getWeekRange(new Date(), weekOffset),
    [view, dayISO, weekOffset],
  );
  const fromISO = toISODate(range.from);
  const toISO = toISODate(range.to);
  const weekDays = useMemo(() => getWeekdays(range.from), [range.from]);
  const gridDays = useMemo(
    () => (view === "day" ? [parseISODate(dayISO)] : weekDays),
    [view, dayISO, weekDays],
  );

  const updateUrlParams = useCallback(
    (updates: Record<string, string | null>) => {
      const params = new URLSearchParams(window.location.search);
      for (const [key, value] of Object.entries(updates)) {
        if (value === null) params.delete(key);
        else params.set(key, value);
      }
      const qs = params.toString();
      window.history.replaceState(
        null,
        "",
        qs ? `?${qs}` : window.location.pathname,
      );
    },
    [],
  );

  const swrKey = `vertretungsplan-week-${fromISO}-${toISO}`;
  // Gap detection is forward-looking: the endpoint rejects a past `date`, so
  // clamp the window start to today and skip entirely for fully-past weeks.
  const today = berlinTodayISO();
  const gapsFromISO = fromISO < today ? today : fromISO;
  const loadGaps = toISO >= today;
  const gapsSWRKey = `vertretungsplan-gaps-${gapsFromISO}-${toISO}`;

  const { data, isLoading, error } = useSWRAuth(
    status === "authenticated" ? swrKey : null,
    () => timetableService.getWeek(fromISO, toISO),
  );
  const { data: gapsData, error: gapsError } = useSWRAuth(
    status === "authenticated" && loadGaps ? gapsSWRKey : null,
    () => timetableService.getGaps(gapsFromISO, toISO),
  );
  const { data: staffData, error: staffError } = useSWRAuth(
    status === "authenticated" ? "vertretungsplan-staff-list" : null,
    () => staffService.getAllStaff(),
  );

  // Route guard: same source the sidebar uses to hide the nav entry
  // (settings schema -> timetable.enabled). Without it, a tenant with the
  // Betreuungsplan/Vertretungsplan disabled could still open this URL directly
  // and mutate retained materialized data. fetchSettingsSchema returns null
  // when the user cannot read settings; the page then renders normally
  // (graceful default, mirroring /timetables).
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

  // Preserve fetch failures so we never render "Keine Termine" / zero gaps as
  // if a failed request had succeeded (an admin must not read a 403/500 as a
  // covered, empty plan). SWR retries produce a fresh Error per attempt, so we
  // key the toasts on the message string to avoid one toast per retry.
  const weekErrorMessage = error
    ? error instanceof Error
      ? error.message
      : String(error)
    : null;
  const gapsErrorMessage = gapsError
    ? gapsError instanceof Error
      ? gapsError.message
      : String(gapsError)
    : null;
  // A failed staff fetch must never masquerade as an empty replacement list —
  // every picker would silently disable and names would fall back to IDs, which
  // is indistinguishable from a tenant with no staff (#1840).
  const staffErrorMessage = staffError
    ? staffError instanceof Error
      ? staffError.message
      : String(staffError)
    : null;

  useEffect(() => {
    if (!weekErrorMessage) return;
    logger.error("week_load_failed", { error: weekErrorMessage });
    toast.error(
      `Vertretungsplan konnte nicht geladen werden: ${weekErrorMessage}`,
    );
  }, [weekErrorMessage, toast]);

  useEffect(() => {
    if (!gapsErrorMessage) return;
    logger.error("gaps_load_failed", { error: gapsErrorMessage });
    toast.error(
      `Offene Lücken konnten nicht geprüft werden: ${gapsErrorMessage}`,
    );
  }, [gapsErrorMessage, toast]);

  useEffect(() => {
    if (!staffErrorMessage) return;
    logger.error("staff_load_failed", { error: staffErrorMessage });
    toast.error(
      `Personalliste konnte nicht geladen werden: ${staffErrorMessage}`,
    );
  }, [staffErrorMessage, toast]);

  const instances = useMemo(() => data?.instances ?? [], [data?.instances]);
  // The day view renders a single column, so hand the grid only that day's
  // instances (otherwise its auto hour-window would stretch to fit other days).
  const gridInstances = useMemo(
    () =>
      view === "day"
        ? instances.filter((inst) => inst.date === dayISO)
        : instances,
    [view, dayISO, instances],
  );
  const staffOptions = useMemo(
    () => (staffData ?? []).map((s) => ({ id: s.id, name: s.name })),
    [staffData],
  );
  const staffNames = useMemo(
    () => new Map((staffData ?? []).map((s) => [s.id, s.name])),
    [staffData],
  );
  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );
  // Staff marked absent anywhere on the selected block's date. Absence is
  // day-wide, so these people are out of action and must not be offered as
  // substitutes in the slide-over picker (#1840).
  const dayAbsentStaffIds = useMemo(() => {
    const ids = new Set<string>();
    const date = selectedInstance?.date;
    if (!date) return ids;
    for (const inst of instances) {
      if (inst.date !== date) continue;
      for (const row of inst.staff) {
        if (row.isAbsent) ids.add(row.staffId);
      }
    }
    return ids;
  }, [instances, selectedInstance?.date]);

  // /gaps always returns the surrounding week, but the day view shows a single
  // day, so scope the KPI counters to that day — otherwise viewing one day
  // surfaces shortfalls from other days of that week (#1840).
  const openGaps = useMemo(() => {
    const all = gapsData?.gaps ?? [];
    return view === "day" ? all.filter((g) => g.date === dayISO) : all;
  }, [gapsData?.gaps, view, dayISO]);
  const acknowledgedGaps = useMemo(() => {
    const all = gapsData?.acknowledged ?? [];
    return view === "day" ? all.filter((g) => g.date === dayISO) : all;
  }, [gapsData?.acknowledged, view, dayISO]);

  // Cache refresh is deliberately SEPARATE from the committed mutation: SWR's
  // mutate rejects by default when a revalidation fetch fails, and we must never
  // let that turn an already-committed save into a reported failure (or abort a
  // multi-edit save). So revalidate swallows its own errors and surfaces a
  // distinct "please reload" toast instead of throwing back into the save (#1840).
  const revalidate = useCallback(async () => {
    try {
      await Promise.all([tenantMutate(swrKey), tenantMutate(gapsSWRKey)]);
    } catch (err) {
      logger.error("revalidate_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        "Ansicht konnte nicht aktualisiert werden. Bitte die Seite neu laden.",
      );
    }
  }, [gapsSWRKey, swrKey, tenantMutate, toast]);

  const handleSelectInstance = useCallback(
    (instance: EnrichedInstance | null) => {
      updateUrlParams({ instance: instance?.id ?? null });
    },
    [updateUrlParams],
  );

  // One atomic save for the whole slide-over form. The backend applies every
  // edit in a single transaction, so the frontend never sequences independent
  // mutations that could half-commit. Cache refresh runs AFTER the committed
  // mutation and cannot revert its success.
  const handleApply = useCallback(
    async (input: ApplyDeviationsInput) => {
      const instance = selectedInstance;
      if (!instance) return;
      try {
        const result = await timetableService.applyDeviations(
          instance.id,
          input,
        );
        if (result.cancelled) {
          toast.success("Block abgesagt");
          updateUrlParams({ instance: null });
        } else {
          const count = result.affectedInstances.length;
          toast.success(
            count > 0
              ? `Vertretungsplan aktualisiert: ${count} Termin(e)`
              : "Vertretungsplan aktualisiert",
          );
          if (result.warnings.length > 0) {
            toast.error(
              `${result.warnings.length} mögliche Zeitüberschneidung(en) prüfen.`,
            );
          }
        }
      } catch (err) {
        logger.error("apply_deviations_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error ? err.message : "Speichern fehlgeschlagen",
        );
        throw err; // keep the slide-over open so the admin can retry
      } finally {
        // Runs on both success and failure; non-throwing (see revalidate).
        await revalidate();
      }
    },
    [selectedInstance, revalidate, toast, updateUrlParams],
  );

  // While the settings schema loads we cannot tell yet whether the feature is
  // enabled — render nothing (Suspense-style) instead of flashing the planner.
  if (status === "loading" || settingsSchemaLoading) {
    return null;
  }

  if (timetableDisabled) {
    return <VertretungsplanDisabledState />;
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title="Vertretungsplan" />

      {weekErrorMessage && (
        <Alert
          type="error"
          message={`Vertretungsplan konnte nicht geladen werden: ${weekErrorMessage}`}
        />
      )}

      {staffErrorMessage && (
        <Alert
          type="error"
          message={`Personalliste konnte nicht geladen werden: ${staffErrorMessage}. Ersatz kann nicht ausgewählt werden, bis die Seite neu geladen wurde.`}
        />
      )}

      <div
        className={`${timetableSurface} flex flex-col gap-2 p-3 sm:flex-row sm:items-center sm:justify-between`}
      >
        <Tabs
          value={view}
          onValueChange={(v) =>
            updateUrlParams(
              v === "day"
                ? { view: "day", day: dayISO }
                : { view: null, day: null },
            )
          }
        >
          <TabsList>
            <TabsTrigger value="week">Woche</TabsTrigger>
            <TabsTrigger value="day">Tag</TabsTrigger>
          </TabsList>
        </Tabs>

        <div className="flex items-center justify-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={view === "day" ? "Vorheriger Tag" : "Vorherige Woche"}
            onClick={() =>
              view === "day"
                ? updateUrlParams({ day: shiftDayISO(dayISO, -1) })
                : updateUrlParams({ week: String(weekOffset - 1) })
            }
          >
            <ChevronLeft className="h-5 w-5" />
          </Button>
          <div className="min-w-[200px] text-center">
            <div className="text-sm font-semibold text-gray-900 tabular-nums">
              {view === "day"
                ? formatDate(dayISO, true)
                : formatWeekLabel(range.from, range.to)}
            </div>
            {(view === "day"
              ? dayISO !== berlinTodayISO()
              : weekOffset !== 0) && (
              <button
                type="button"
                className="text-[11px] font-medium text-[#5A8E1F] hover:underline"
                onClick={() =>
                  view === "day"
                    ? updateUrlParams({ day: berlinTodayISO() })
                    : updateUrlParams({ week: "0" })
                }
              >
                Heute
              </button>
            )}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={view === "day" ? "Nächster Tag" : "Nächste Woche"}
            onClick={() =>
              view === "day"
                ? updateUrlParams({ day: shiftDayISO(dayISO, 1) })
                : updateUrlParams({ week: String(weekOffset + 1) })
            }
          >
            <ChevronRight className="h-5 w-5" />
          </Button>
        </div>
      </div>

      <VertretungsplanOverview
        openCount={openGaps.length}
        ackCount={acknowledgedGaps.length}
        gapsUnavailable={gapsErrorMessage !== null}
      />

      <WeeklyCalendarGrid
        weekDays={gridDays}
        instances={gridInstances}
        selectedId={selectedInstanceId}
        onInstanceClick={handleSelectInstance}
        todayISO={berlinTodayISO()}
        dayStartHour={dayStartHour}
        dayEndHour={dayEndHour}
        hourHeightPx={HOUR_HEIGHT_PX}
        emptyState={
          gridInstances.length > 0
            ? undefined
            : weekErrorMessage
              ? {
                  title: "Termine konnten nicht geladen werden",
                  description:
                    "Bitte die Seite neu laden. Dies ist kein leerer Plan.",
                }
              : isLoading
                ? { title: "Lädt…", description: "Termine werden geladen." }
                : {
                    title: "Keine Termine",
                    description:
                      view === "day"
                        ? "Für diesen Tag sind keine Termine geplant."
                        : "Für diese Woche sind keine Termine geplant.",
                  }
        }
      />

      <SubstitutionSlideOver
        instance={selectedInstance}
        staffOptions={staffOptions}
        staffNames={staffNames}
        dayAbsentStaffIds={dayAbsentStaffIds}
        staffLoadError={staffErrorMessage !== null}
        onClose={() => handleSelectInstance(null)}
        onApply={handleApply}
      />
    </div>
  );
}

// VertretungsplanOverview mirrors the Betreuungsplan's TimetableOverview so the
// two planning pages share the same top-of-page shell (kicker + title +
// description + two KPI stat cards). The purpose differs, this one tracks
// staffing shortfalls, not planned counts.
function VertretungsplanOverview({
  openCount,
  ackCount,
  gapsUnavailable,
}: {
  openCount: number;
  ackCount: number;
  /** True when the /gaps request failed — show "—" instead of a false zero. */
  gapsUnavailable: boolean;
}) {
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-5">
      <div>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Planung
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Vertretungsplan im Blick
        </h2>
        <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
          Kurzfristige Abweichungen vom Betreuungsplan für eine Woche oder einen
          Tag: Abwesenheiten, Ersatz und ausfallende oder bewusst unbesetzte
          Blöcke, ohne die Halbjahresvorlage zu ändern.
        </p>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <TimetableStatCard
          size="lg"
          icon={<UserX className="h-4 w-4" />}
          label="Offene Lücken"
          value={gapsUnavailable ? "—" : String(openCount)}
          sublabel={gapsUnavailable ? "nicht verfügbar" : "brauchen Personal"}
          tone={!gapsUnavailable && openCount > 0 ? "danger" : "neutral"}
        />
        <TimetableStatCard
          size="lg"
          icon={<TriangleAlert className="h-4 w-4" />}
          label="Bewusst unbesetzt"
          value={gapsUnavailable ? "—" : String(ackCount)}
          sublabel={
            gapsUnavailable ? "nicht verfügbar" : "ohne Personal akzeptiert"
          }
          tone={!gapsUnavailable && ackCount > 0 ? "warning" : "neutral"}
        />
      </div>
    </section>
  );
}

/**
 * Rendered when the tenant setting `timetable.enabled` resolves to false. The
 * sidebar already hides the nav entry for disabled tenants; this guard covers
 * direct navigation without redirecting (no redirect loops). Mirrors
 * /timetables' TimetableDisabledState.
 */
function VertretungsplanDisabledState() {
  return (
    <div
      className="flex flex-col gap-4"
      data-testid="vertretungsplan-disabled-state"
    >
      <PageHeader title="Vertretungsplan" />
      <div className={`${timetableSurface} p-10 text-center`}>
        <CalendarOff className="mx-auto h-10 w-10 text-gray-300" aria-hidden />
        <h2 className="mt-4 text-base font-semibold text-gray-900">
          Vertretungsplan ist deaktiviert
        </h2>
        <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-600">
          Der Vertretungsplan gehört zum Betreuungsplan, der für diese Schule
          ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“ wieder
          aktiviert werden.
        </p>
      </div>
    </div>
  );
}

export default function VertretungsplanPage() {
  return (
    <Suspense fallback={null}>
      <VertretungsplanContent />
    </Suspense>
  );
}
