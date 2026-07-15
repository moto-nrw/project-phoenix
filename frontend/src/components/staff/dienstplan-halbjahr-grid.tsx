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

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  CoverageIndicator,
  type CoverageTone,
} from "~/components/ui/coverage-indicator";
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
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { staffShiftService } from "~/lib/shift-api";
import {
  formatPlannedHours,
  type StaffScheduleOverview,
  type StaffScheduleStaff,
  type StaffShift,
} from "~/lib/shift-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { getWeekNumber } from "~/lib/time-tracking-helpers";

// Half-year view (docs/05-dienstplan.md Abschnitt 3): rows are staff, columns
// are the calendar weeks of the planning period containing the anchor date `d`.
// A pure read-and-jump surface — no editing, no blocks, no menus. Each cell is
// the person's planned weekly sum with the same delta coloring as the week
// view's row header; clicking it jumps to that week (view=woche). Data loads
// per week through the SAME SWR keys the week view uses, so navigating
// week↔half-year serves cached windows both ways.

// Reuse of the week hook's SWR keys is the whole point of the cache-sharing
// contract (docs/05 Abschnitt 3, Chunk 3 report): the half-year loader for a
// week and the week view's own hook resolve to the identical key string, so
// SWR dedupes and the round trip never refetches. Keep these in lockstep with
// use-dienstplan-data.ts.
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
  /** Monday of the week as "YYYY-MM-DD" — the column key and SWR key `from`. */
  monday: string;
  /** Friday of the week as "YYYY-MM-DD" — the SWR key `to`. */
  friday: string;
  kw: number;
}

// ─── Delta coloring (duplicated 1:1 from dienstplan-resource-grid.tsx, Chunk 4)
// The parallel agent owns that file, so importing from it would be a merge
// hazard; per the Chunk 7 brief these ~10 lines are duplicated with this
// origin note and Chunk 10 may consolidate them. Under contract → red, over →
// amber, exact/no target → neutral (docs/05 Abschnitt 2.3). Only the free-text
// label of the CoverageIndicator is tinted.
function summaryTone(cell: WeekSummaryCell): CoverageTone {
  if (cell.targetMinutes === null || cell.deltaMinutes === null) {
    return "neutral";
  }
  if (cell.deltaMinutes < 0) return "under";
  if (cell.deltaMinutes > 0) return "over";
  return "neutral";
}

// "18/20,25 h" with a target, "18 h" without one (docs/05 Abschnitt 2.3).
function summaryLabel(cell: WeekSummaryCell): string {
  const planned = formatPlannedHours(cell.plannedMinutes);
  if (cell.targetMinutes === null) return planned;
  const plannedValue = planned.replace(/\s*h$/, "");
  return `${plannedValue}/${formatPlannedHours(cell.targetMinutes)}`;
}

function timeToMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(":");
  return Number(h) * 60 + Number(m);
}

// Net minutes of a single shift: span minus break, never negative.
function shiftNetMinutes(shift: StaffShift): number {
  const span = timeToMinutes(shift.endTime) - timeToMinutes(shift.startTime);
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
  weekFrom: string,
): Map<string, WeekSummaryCell> {
  const byStaff = new Map<string, WeekSummaryCell>();
  for (const summary of overview?.weeklySummaries ?? []) {
    if (summary.weekStart !== weekFrom) continue;
    byStaff.set(summary.staffId, {
      plannedMinutes: summary.plannedMinutes,
      targetMinutes: summary.targetMinutes,
      deltaMinutes: summary.deltaMinutes,
    });
  }
  return byStaff;
}

function startOfWeek(d: Date): Date {
  const monday = new Date(d);
  const day = (monday.getDay() + 6) % 7; // Mon = 0
  monday.setDate(monday.getDate() - day);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

function formatColumnDate(isoDate: string): string {
  const [, m, d] = isoDate.split("-");
  return `${d}.${m}.`;
}

// All week columns of the period: the Monday of the week containing the period
// start, stepping +7 until the Monday passes the period end. Monday iteration
// is pure Date arithmetic (no toISOString date extraction).
function weeksInPeriod(period: CalendarPeriod): WeekMeta[] {
  const weeks: WeekMeta[] = [];
  const cursor = startOfWeek(parseISODate(period.startDate));
  // Guard against a malformed period (end before start) so the loop always
  // terminates.
  let guard = 0;
  while (toISODate(cursor) <= period.endDate && guard < 400) {
    const friday = new Date(cursor);
    friday.setDate(friday.getDate() + 4);
    weeks.push({
      monday: toISODate(cursor),
      friday: toISODate(friday),
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
  weekFrom,
  weekTo,
  reducedPath,
  onState,
}: {
  readonly weekFrom: string;
  readonly weekTo: string;
  readonly reducedPath: boolean;
  readonly onState: (weekKey: string, state: WeekColumnState) => void;
}) {
  const {
    data: overview,
    error: overviewError,
    mutate: mutateOverview,
  } = useSWRAuth<StaffScheduleOverview>(
    reducedPath ? null : overviewKey(weekFrom, weekTo),
    () => withSlot(() => staffShiftService.getOverview(weekFrom, weekTo)),
  );

  const {
    data: shifts,
    error: shiftsError,
    mutate: mutateShifts,
  } = useSWRAuth<StaffShift[]>(
    reducedPath ? shiftsKey(weekFrom, weekTo) : null,
    () => withSlot(() => staffShiftService.getShifts(weekFrom, weekTo)),
  );

  const summaryByStaff = useMemo(
    () =>
      reducedPath
        ? summariesFromShifts(shifts ?? [])
        : summariesFromOverview(overview, weekFrom),
    [reducedPath, shifts, overview, weekFrom],
  );

  const error = reducedPath ? shiftsError : overviewError;
  const hasData = reducedPath ? shifts !== undefined : overview !== undefined;
  const retry = reducedPath ? mutateShifts : mutateOverview;
  const status: WeekStatus = error ? "error" : hasData ? "ready" : "loading";

  useEffect(() => {
    onState(weekFrom, { status, summaryByStaff, retry });
  }, [weekFrom, status, summaryByStaff, retry, onState]);

  return null;
}

interface HalbjahrGridInnerProps {
  readonly period: CalendarPeriod;
  readonly staff: readonly StaffScheduleStaff[];
  readonly reducedPath: boolean;
  readonly todayIso: string;
  readonly onWeekClick: (mondayISO: string) => void;
}

// Keyed by period.id from the parent, so a period change remounts this subtree
// with a fresh weekStates map instead of carrying stale week entries over.
function HalbjahrGridInner({
  period,
  staff,
  reducedPath,
  todayIso,
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

  const currentMonday = toISODate(startOfWeek(parseISODate(todayIso)));

  const columns: ResourceGridColumn[] = weeks.map((week) => ({
    key: week.monday,
    label: `KW ${week.kw}`,
    sublabel: formatColumnDate(week.monday),
    isCurrent: week.monday === currentMonday,
  }));

  const firstStaffId = staff[0]?.id ?? null;

  const renderRowHeader = (member: StaffScheduleStaff): ReactNode => (
    <span className="block truncate text-sm font-medium text-gray-900">
      {member.lastName}, {member.firstName}
    </span>
  );

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
          className="flex min-h-8 w-full items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-50 hover:text-gray-600 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
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
        className="flex w-full items-center justify-center rounded-md px-1 py-1.5 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
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
          weekFrom={week.monday}
          weekTo={week.friday}
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
  /** Jump into the week view for the clicked week (Monday "YYYY-MM-DD"). */
  readonly onWeekClick: (mondayISO: string) => void;
}

export function DienstplanHalbjahrGrid({
  dayISO,
  staff,
  reducedPath,
  todayIso,
  onWeekClick,
}: DienstplanHalbjahrGridProps) {
  const router = useTenantRouter();
  const {
    data: periods,
    error: periodsError,
    isLoading: periodsLoading,
    mutate: mutatePeriods,
  } = useSWRAuth<CalendarPeriod[]>("dienstplan-calendar-periods", () =>
    calendarPeriodService.list(),
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
      onWeekClick={onWeekClick}
    />
  );
}
