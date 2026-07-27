"use client";

import { RotateCw } from "lucide-react";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import {
  ResourceGrid,
  type ResourceGridColumn,
} from "~/components/ui/resource-grid";
import { Skeleton } from "~/components/ui/skeleton";
import { calendarPeriodService } from "~/lib/calendar-period-api";
import {
  findPeriodForDate,
  type CalendarPeriod,
} from "~/lib/calendar-period-helpers";
import {
  expandClosingDaysToMap,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { staffShiftService } from "~/lib/shift-api";
import {
  formatColumnDate,
  summaryLabel,
  summaryTone,
  type StaffScheduleOverview,
  type StaffScheduleStaff,
  type StaffShift,
} from "~/lib/shift-helpers";
import { startOfWeek } from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { getWeekNumber } from "~/lib/time-tracking-helpers";
import { parseTimeToMinutes } from "~/lib/timetable-helpers";

// Half-year view (docs/05-dienstplan.md Abschnitt 3): rows are staff, columns
// are the calendar weeks of the planning period containing the anchor date `d`.
// A pure read-and-jump surface — no editing, no blocks, no menus. Each cell is
// the person's planned weekly sum with the same delta coloring as the week
// view's row header; clicking it jumps to that week (view=woche). Data loads
// per week. Full-permission interior weeks reuse the week view's SWR keys;
// reduced-permission sums deliberately load Monday-Sunday because raw shifts,
// unlike server summaries, are limited to the requested range.

// Keep the key format in lockstep with use-dienstplan-data.ts. The overview
// path shares its ordinary Mon-Fri windows with the week view; the raw-shift
// path uses a distinct full-week key so weekend work is not undercounted.
const overviewKey = (from: string, to: string): string =>
  `dienstplan-overview-${from}-${to}`;
const shiftsKey = (from: string, to: string): string =>
  `dienstplan-shifts-${from}-${to}`;

// A half-year is ~20–26 weeks, i.e. that many overview/shift requests. The
// module-local semaphore caps concurrent network calls so a mount does not
// fire two dozen requests at once (docs/05 Abschnitt 3). SWR-cached weeks
// never enter the fetcher, so they never consume a slot.
const MAX_CONCURRENT_WEEK_FETCHES = 4;
let activeFetches = 0;
const waiters: Array<() => void> = [];

function acquireSlot(): Promise<void> {
  if (activeFetches < MAX_CONCURRENT_WEEK_FETCHES) {
    activeFetches += 1;
    return Promise.resolve();
  }
  // The slot is handed directly to the next waiter on release, so the count is
  // never decremented-then-reincremented (which would race a fresh acquire in
  // between and overshoot the cap).
  return new Promise<void>((resolve) => waiters.push(resolve));
}

function releaseSlot(): void {
  const next = waiters.shift();
  if (next) {
    next();
    return;
  }
  activeFetches -= 1;
}

async function withSlot<T>(fn: () => Promise<T>): Promise<T> {
  await acquireSlot();
  try {
    return await fn();
  } finally {
    releaseSlot();
  }
}

// One week column's planned sum for a person. targetMinutes/deltaMinutes are
// null on the reduced permission path (no Soll), which yields the neutral tone.
interface WeekSummaryCell {
  plannedMinutes: number;
  targetMinutes: number | null;
  deltaMinutes: number | null;
}

type WeekStatus = "loading" | "error" | "ready";

interface WeekColumnState {
  status: WeekStatus;
  summaryByStaff: Map<string, WeekSummaryCell>;
  retry: () => void;
}

interface WeekMeta {
  /** Monday of the week as "YYYY-MM-DD" — stable column/summary key. */
  monday: string;
  /** Overview bounds preserve the normal Mon-Fri SWR key for interior weeks. */
  overviewFrom: string;
  overviewTo: string;
  /** Raw-shift bounds cover the complete Mon-Sun week, clamped to the period. */
  shiftsFrom: string;
  shiftsTo: string;
  /** The planning period covers only part of the complete Mon-Sun week. */
  isBoundaryClamped: boolean;
  kw: number;
}

// summaryTone / summaryLabel are shared with the week grid via
// ~/lib/shift-helpers (both operate on the {planned,target,delta}Minutes shape
// WeekSummaryCell satisfies). Under contract → red, over → amber, exact/no
// target → neutral (docs/05 Abschnitt 2.3).

// Net minutes of a single shift: span minus break, never negative.
// parseTimeToMinutes returns NaN for malformed "HH:MM" (guarded via Math.max
// only for valid data — shifts always carry valid wall-clock times here).
function shiftNetMinutes(shift: StaffShift): number {
  const span =
    parseTimeToMinutes(shift.endTime) - parseTimeToMinutes(shift.startTime);
  return Math.max(0, span - shift.breakMinutes);
}

// Reduced permission path: client-side weekly sums from raw shifts. Cancelled
// shifts contribute nothing (Abgleich B6, mirrors the server's overview math);
// replacement shifts count — they are real shifts. No Soll comparison.
function summariesFromShifts(
  shifts: readonly StaffShift[],
): Map<string, WeekSummaryCell> {
  const byStaff = new Map<string, WeekSummaryCell>();
  for (const shift of shifts) {
    if (shift.cancelled) continue;
    const net = shiftNetMinutes(shift);
    const existing = byStaff.get(shift.staffId);
    if (existing) {
      existing.plannedMinutes += net;
    } else {
      byStaff.set(shift.staffId, {
        plannedMinutes: net,
        targetMinutes: null,
        deltaMinutes: null,
      });
    }
  }
  return byStaff;
}

// Full path: the server already delivers net planned minutes plus Soll/Delta in
// weeklySummaries; keep only the summary for this exact week window.
function summariesFromOverview(
  overview: StaffScheduleOverview | undefined,
  weekStart: string,
  scopePlannedToViewport: boolean,
): Map<string, WeekSummaryCell> {
  const byStaff = new Map<string, WeekSummaryCell>();
  for (const summary of overview?.weeklySummaries ?? []) {
    if (summary.weekStart !== weekStart) continue;
    byStaff.set(summary.staffId, {
      plannedMinutes: summary.plannedMinutes,
      targetMinutes: summary.targetMinutes,
      deltaMinutes: summary.deltaMinutes,
    });
  }
  if (!scopePlannedToViewport) return byStaff;

  // Overview summaries deliberately cover the complete Monday-Sunday week,
  // even when the requested viewport is shorter. Boundary columns must instead
  // count only the shifts returned inside the clamped planning-period range.
  // Keep the server-resolved weekly target, then recompute planned and delta.
  const viewportPlanned = summariesFromShifts(overview?.shifts ?? []);
  const staffIds = new Set([...byStaff.keys(), ...viewportPlanned.keys()]);
  for (const staffId of staffIds) {
    const server = byStaff.get(staffId);
    const plannedMinutes = viewportPlanned.get(staffId)?.plannedMinutes ?? 0;
    const targetMinutes = server?.targetMinutes ?? null;
    if (plannedMinutes === 0 && targetMinutes === null) {
      byStaff.delete(staffId);
      continue;
    }
    byStaff.set(staffId, {
      plannedMinutes,
      targetMinutes,
      deltaMinutes:
        targetMinutes === null ? null : plannedMinutes - targetMinutes,
    });
  }
  return byStaff;
}

// All week columns of the period. Boundary columns retain their Monday as the
// stable week key, but fetch only the intersection with the planning period so
// shifts from an adjacent period never leak into the weekly total.
function weeksInPeriod(period: CalendarPeriod): WeekMeta[] {
  const weeks: WeekMeta[] = [];
  const cursor = startOfWeek(parseISODate(period.startDate));
  // Guard against a malformed period (end before start) so the loop always
  // terminates.
  let guard = 0;
  while (toISODate(cursor) <= period.endDate && guard < 400) {
    const friday = new Date(cursor);
    friday.setDate(friday.getDate() + 4);
    const sunday = new Date(cursor);
    sunday.setDate(sunday.getDate() + 6);
    const monday = toISODate(cursor);
    const fridayISO = toISODate(friday);
    const sundayISO = toISODate(sunday);
    const isBoundaryClamped =
      monday < period.startDate || sundayISO > period.endDate;
    const shiftsFrom = monday < period.startDate ? period.startDate : monday;
    const shiftsTo = sundayISO > period.endDate ? period.endDate : sundayISO;
    weeks.push({
      monday,
      // Interior overview requests keep sharing the week view's Mon-Fri SWR
      // key; boundary requests need the exact period intersection so the
      // client-side correction sees every in-period weekend shift.
      overviewFrom: isBoundaryClamped ? shiftsFrom : monday,
      overviewTo: isBoundaryClamped ? shiftsTo : fridayISO,
      shiftsFrom,
      shiftsTo,
      isBoundaryClamped,
      kw: getWeekNumber(cursor),
    });
    cursor.setDate(cursor.getDate() + 7);
    guard += 1;
  }
  return weeks;
}

// ─── Invisible per-week loader ───────────────────────────────────────────────
// Renders nothing; each instance owns exactly one active SWR subscription (the
// other key is null on the inactive permission path) and reports its state up.
// Fixing the hook count this way is what lets the grid keep per-column
// progressivity without breaking the rules-of-hooks (the parent can't call a
// variable number of useSWRAuth for a variable number of columns).
function HalbjahrColumnData({
  weekKey,
  overviewFrom,
  overviewTo,
  shiftsFrom,
  shiftsTo,
  isBoundaryClamped,
  reducedPath,
  onState,
}: {
  readonly weekKey: string;
  readonly overviewFrom: string;
  readonly overviewTo: string;
  readonly shiftsFrom: string;
  readonly shiftsTo: string;
  readonly isBoundaryClamped: boolean;
  readonly reducedPath: boolean;
  readonly onState: (weekKey: string, state: WeekColumnState) => void;
}) {
  const {
    data: overview,
    error: overviewError,
    mutate: mutateOverview,
  } = useSWRAuth<StaffScheduleOverview>(
    reducedPath ? null : overviewKey(overviewFrom, overviewTo),
    () =>
      withSlot(() => staffShiftService.getOverview(overviewFrom, overviewTo)),
  );

  const {
    data: shifts,
    error: shiftsError,
    mutate: mutateShifts,
  } = useSWRAuth<StaffShift[]>(
    reducedPath ? shiftsKey(shiftsFrom, shiftsTo) : null,
    () => withSlot(() => staffShiftService.getShifts(shiftsFrom, shiftsTo)),
  );

  const summaryByStaff = useMemo(
    () =>
      reducedPath
        ? summariesFromShifts(shifts ?? [])
        : summariesFromOverview(overview, weekKey, isBoundaryClamped),
    [reducedPath, shifts, overview, weekKey, isBoundaryClamped],
  );

  const error = reducedPath ? shiftsError : overviewError;
  const hasData = reducedPath ? shifts !== undefined : overview !== undefined;
  const retry = reducedPath ? mutateShifts : mutateOverview;
  const status: WeekStatus = error ? "error" : hasData ? "ready" : "loading";

  useEffect(() => {
    onState(weekKey, { status, summaryByStaff, retry });
  }, [weekKey, status, summaryByStaff, retry, onState]);

  return null;
}

interface HalbjahrGridInnerProps {
  readonly period: CalendarPeriod;
  readonly staff: readonly StaffScheduleStaff[];
  readonly reducedPath: boolean;
  readonly todayIso: string;
  readonly closingDayRanges: readonly ClosingDayRange[];
  readonly onWeekClick: (mondayISO: string) => void;
}

// Keyed by period.id from the parent, so a period change remounts this subtree
// with a fresh weekStates map instead of carrying stale week entries over.
function HalbjahrGridInner({
  period,
  staff,
  reducedPath,
  todayIso,
  closingDayRanges,
  onWeekClick,
}: HalbjahrGridInnerProps) {
  const scrollHintId = useId();
  const [weekStates, setWeekStates] = useState<Map<string, WeekColumnState>>(
    () => new Map(),
  );

  const reportWeekState = useCallback(
    (weekKey: string, state: WeekColumnState) => {
      setWeekStates((prev) => {
        const next = new Map(prev);
        next.set(weekKey, state);
        return next;
      });
    },
    [],
  );

  const weeks = useMemo(() => weeksInPeriod(period), [period]);
  const kwByKey = useMemo(() => {
    const map = new Map<string, number>();
    for (const week of weeks) map.set(week.monday, week.kw);
    return map;
  }, [weeks]);

  // Schließtage des Zeitraums (#2032). Die Halbjahres-Sicht plant nichts, sie
  // macht nur sichtbar, in welchen Wochen Schließtage liegen — gezählt werden
  // Mo–Fr, weil die Wochenansicht dieselben fünf Tage zeigt.
  const closingDays = useMemo(() => {
    const firstWeek = weeks[0];
    const lastWeek = weeks.at(-1);
    if (!firstWeek || !lastWeek) return new Map<string, string>();
    // weeksInPeriod has a defensive 400-column guard. Clamp the expansion to
    // those actually rendered weeks as well, so a mistakenly huge period or
    // closing-day range cannot allocate entries through its remote end date.
    return expandClosingDaysToMap(
      closingDayRanges,
      firstWeek.shiftsFrom,
      lastWeek.shiftsTo,
    );
  }, [closingDayRanges, weeks]);
  const closingByWeek = useMemo(() => {
    const byWeek = new Map<string, string[]>();
    if (closingDays.size === 0) return byWeek;
    for (const week of weeks) {
      const monday = parseISODate(week.monday);
      const days: string[] = [];
      for (let offset = 0; offset < 5; offset++) {
        const day = new Date(monday);
        day.setDate(day.getDate() + offset);
        const iso = toISODate(day);
        const reason = closingDays.get(iso);
        if (reason !== undefined) {
          days.push(`${formatColumnDate(iso)} ${reason}`.trim());
        }
      }
      if (days.length > 0) byWeek.set(week.monday, days);
    }
    return byWeek;
  }, [weeks, closingDays]);

  const currentMonday = toISODate(startOfWeek(parseISODate(todayIso)));

  // Jede fertig geladene Woche meldet ihren Zustand hoch, das Raster rendert
  // während des Ladens also ein gutes Dutzend Mal neu. Die Spalten hängen an
  // keinem dieser Zustände, sie werden nur einmal je Zeitraum gebaut.
  const columns: ResourceGridColumn[] = useMemo(
    () =>
      weeks.map((week) => {
        const closing = closingByWeek.get(week.monday);
        return {
          key: week.monday,
          label: `KW ${week.kw}`,
          sublabel: formatColumnDate(week.monday),
          isCurrent: week.monday === currentMonday,
          isMuted: closing?.length === 5,
          headerNote:
            closing === undefined ? undefined : (
              <ClosingDayChip
                text={String(closing.length)}
                label={`${closing.length} ${closing.length === 1 ? "Schließtag" : "Schließtage"}`}
                reason={closing.join(", ")}
                className="mx-auto flex w-fit"
              />
            ),
        };
      }),
    [weeks, closingByWeek, currentMonday],
  );

  const firstStaffId = staff[0]?.id ?? null;

  const renderRowHeader = (member: StaffScheduleStaff): ReactNode => (
    <span className="block truncate text-sm font-medium text-gray-900">
      {member.lastName}, {member.firstName}
    </span>
  );

  // Interaktive Zellinhalte tragen flex-1: das ResourceGrid gibt der Zelle eine
  // Mindesthöhe, ohne Füllen bliebe die Klick-/Hoverfläche kleiner als ihre
  // Zelle (Issue #2026).
  const renderCell = (
    member: StaffScheduleStaff,
    column: ResourceGridColumn,
  ): ReactNode => {
    const state = weekStates.get(column.key);
    const kw = kwByKey.get(column.key) ?? 0;

    // Still loading (or the loader has not reported yet): a skeleton cell.
    if (!state || state.status === "loading") {
      return <Skeleton className="mx-auto h-4 w-10" />;
    }

    // Failed week: a single retry affordance for the column (docs/05 Abschnitt
    // 5), placed in the first row's cell so the column carries exactly one
    // button; the remaining rows stay empty and the other columns keep working.
    if (state.status === "error") {
      if (member.id !== firstStaffId) return null;
      return (
        <button
          type="button"
          onClick={() => state.retry()}
          aria-label={`Woche ${kw} erneut laden`}
          className="flex w-full flex-1 items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-50 hover:text-gray-600 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
        >
          <RotateCw className="h-4 w-4" aria-hidden="true" />
        </button>
      );
    }

    const cell = state.summaryByStaff.get(member.id);
    // Empty week: no planned minutes → empty cell, no "0,0 h" noise (docs/05
    // Abschnitt 3). Returning null with no empty-cell affordance leaves it bare.
    if (!cell || cell.plannedMinutes === 0) return null;

    return (
      <button
        type="button"
        onClick={() => onWeekClick(column.key)}
        aria-label={`Woche ${kw} öffnen, ${member.firstName} ${member.lastName}`}
        className="flex w-full flex-1 items-center justify-center rounded-md px-1 py-1.5 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
      >
        <CoverageIndicator
          state="covered"
          size="sm"
          label={summaryLabel(cell)}
          tone={summaryTone(cell)}
        />
      </button>
    );
  };

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      {/* Invisible per-week loaders — no DOM, they only feed weekStates. */}
      {weeks.map((week) => (
        <HalbjahrColumnData
          key={week.monday}
          weekKey={week.monday}
          overviewFrom={week.overviewFrom}
          overviewTo={week.overviewTo}
          shiftsFrom={week.shiftsFrom}
          shiftsTo={week.shiftsTo}
          isBoundaryClamped={week.isBoundaryClamped}
          reducedPath={reducedPath}
          onState={reportWeekState}
        />
      ))}
      <p id={scrollHintId} className="mb-2 text-xs text-gray-600 sm:hidden">
        Wische horizontal, um weitere Wochen zu sehen.
      </p>
      <ResourceGrid
        columns={columns}
        rows={staff}
        getRowKey={(member) => member.id}
        renderRowHeader={renderRowHeader}
        renderCell={renderCell}
        columnMode="weeks"
        cornerHeader="Person"
        scrollHintId={scrollHintId}
        ariaLabel="Dienstplan-Halbjahresansicht"
      />
    </div>
  );
}

interface DienstplanHalbjahrGridProps {
  /** Anchor calendar day as "YYYY-MM-DD"; its planning period drives the
   *  columns. */
  readonly dayISO: string;
  /** Staff rows (from the week overview — staff is week-independent, so it is
   *  valid for every week including ones with no data). */
  readonly staff: readonly StaffScheduleStaff[];
  /** Reduced permission path: weekly sums are computed client-side from raw
   *  shifts, without Soll comparison. */
  readonly reducedPath: boolean;
  /** Today as "YYYY-MM-DD" for the current-week column tint. */
  readonly todayIso: string;
  /** OGS-Schließtage des Tenants (#2032). Wochen mit Schließtagen werden im
   *  Spaltenkopf gekennzeichnet; ohne die Liste entfaellt die Kennzeichnung. */
  readonly closingDayRanges?: readonly ClosingDayRange[];
  /** Jump into the week view for the clicked week (Monday "YYYY-MM-DD"). */
  readonly onWeekClick: (mondayISO: string) => void;
}

const EMPTY_CLOSING_DAY_RANGES: readonly ClosingDayRange[] = [];

export function DienstplanHalbjahrGrid({
  dayISO,
  staff,
  reducedPath,
  todayIso,
  closingDayRanges = EMPTY_CLOSING_DAY_RANGES,
  onWeekClick,
}: DienstplanHalbjahrGridProps) {
  const router = useTenantRouter();
  const {
    data: periods,
    error: periodsError,
    isLoading: periodsLoading,
    mutate: mutatePeriods,
  } = useSWRAuth<CalendarPeriod[]>(
    // Gleicher SWR-Key wie Dienstplan-Kontextzeile und Betreuungsplan: nach
    // Anlegen/Bearbeiten/Löschen eines Zeitraums genügt ein mutate auf diesen
    // Key, damit auch die Halbjahres-Sicht frische Spalten zeigt.
    "database-calendar-periods-list",
    () => calendarPeriodService.list(),
  );

  if (periodsLoading || (!periods && !periodsError)) {
    return (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="space-y-2">
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
          <Skeleton className="h-8 w-full" />
        </div>
      </div>
    );
  }

  if (periodsError) {
    return (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="space-y-3">
          <Alert
            type="error"
            message="Die Planungszeiträume konnten nicht geladen werden. Die Halbjahres-Sicht ist erst verfügbar, wenn sie erfolgreich geladen wurden."
          />
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => void mutatePeriods()}
          >
            Erneut laden
          </Button>
        </div>
      </div>
    );
  }

  const period = findPeriodForDate(periods ?? [], dayISO);

  // No planning period covers the anchor date (docs/05 Abschnitt 4): a plain
  // card with a jump to the calendar-period management. No artwork.
  if (!period) {
    return (
      <div className="moto-content-surface rounded-2xl border p-6 shadow-sm sm:p-8">
        <div className="flex flex-col items-center gap-3 py-6 text-center">
          <p className="text-sm font-semibold text-gray-900">
            Kein Planungszeitraum für dieses Datum
          </p>
          <p className="max-w-md text-sm leading-relaxed text-gray-600">
            Für den {formatDate(dayISO)} ist kein Planungszeitraum hinterlegt.
            Lege einen Kalenderzeitraum an, um die Halbjahres-Sicht zu nutzen.
          </p>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => router.push("/calendar-periods")}
          >
            Zu den Planungszeiträumen
          </Button>
        </div>
      </div>
    );
  }

  return (
    <HalbjahrGridInner
      key={period.id}
      period={period}
      staff={staff}
      reducedPath={reducedPath}
      todayIso={todayIso}
      closingDayRanges={closingDayRanges}
      onWeekClick={onWeekClick}
    />
  );
}
