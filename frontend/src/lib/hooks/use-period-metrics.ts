"use client";

import { useMemo } from "react";

import { useAccountBalance } from "~/lib/hooks/use-account-balance";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { parseISODate } from "~/lib/date-helpers";
import {
  staffAbsenceService,
  staffHistoryService,
  staffMonthSummaryService,
} from "~/lib/staff-api";
import type { StaffAbsenceRow, StaffHistorySession } from "~/lib/staff-api";
import {
  adaptAbsenceForMetrics,
  adaptHistorySessionForMetrics,
  computePeriodTotalsFromTargets,
  endOfWeek,
  resolveAccountStartDate,
  startOfWeek,
  toDateKey,
  type PeriodTotals,
} from "~/lib/staff-metrics-helpers";
import { useSWRAuth } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import {
  OPEN_MONTH_REFRESH_MS,
  targetsFromProjection,
  type DayProjection,
  type MonthSummary,
  type StaffAbsence,
  type WeeklySummary,
  type WorkSessionHistory,
} from "~/lib/time-tracking-helpers";

export interface PeriodMetrics {
  /** Current week; null while loading or when the Soll is unavailable. */
  readonly week: PeriodTotals | null;
  /** Current month; null while loading. */
  readonly month: PeriodTotals | null;
  /** Anchor date the cumulative Stundenkonto is measured from. */
  readonly accountStart: Date;
  /** Cumulative Saldo; null while loading or when it cannot be attributed. */
  readonly accountBalanceMinutes: number | null;
}

/**
 * Date-valid Soll/Ist/Saldo for the CURRENT week and month, plus the
 * Stundenkonto (#1842).
 *
 * Every figure here is priced against the schedule that was valid on each
 * individual day, which is the whole point: `computeStaffMetrics` only ever
 * sees ONE StaffSchedule and applies it to the entire range, so a week or
 * month spanning a contract change (8h -> 4h) gets re-priced at today's hours.
 * Rendering that next to the date-valid Monatskarte and the date-valid daily
 * rows put two contradicting totals for the same period on one screen.
 *
 * Sources, all server-computed:
 * - month  -> the current month's Monatskarte (`month-summary`)
 * - week   -> `schedule-targets` (the per-day projection) for the week + the
 *             week's sessions/absences
 * - Saldo  -> `useAccountBalance`
 *
 * Pass a `staffId` for the admin (`time_tracking:manage`) endpoints; omit it to
 * read the caller's own figures. Keys deliberately match the ones the
 * Zeiterfassung table and Monatskarte already use, so SWR dedupes the overlap
 * into one request and the existing save/check-in invalidation patterns
 * ("staff-history-", "time-tracking-table", …) refresh these cards too.
 */
export function usePeriodMetrics(staffId?: string): PeriodMetrics {
  const todayISO = useBerlinToday();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);
  const year = Number(todayISO.slice(0, 4));
  const monthNumber = Number(todayISO.slice(5, 7));

  // The month card reads the same aggregate as the Monatskarte, under the same
  // key. No config gating: the backend resolves the account anchor itself, and
  // Soll/Ist/Saldo of a single month are valid regardless of where the
  // cumulative chain starts.
  const { data: monthSummary } = useSWRAuth<MonthSummary>(
    staffId
      ? `staff-month-summary-${staffId}-${year}-${monthNumber}`
      : `time-tracking-month-summary-${year}-${monthNumber}`,
    () =>
      staffId
        ? staffMonthSummaryService.getMonthSummary(staffId, year, monthNumber)
        : timeTrackingService.getMonthSummary(year, monthNumber),
    // The current month is always open: a running session grows Ist
    // server-side, so the card has to keep up.
    { refreshInterval: OPEN_MONTH_REFRESH_MS },
  );

  const weekStart = useMemo(() => startOfWeek(today), [today]);
  const weekEnd = useMemo(() => endOfWeek(today), [today]);
  const weekFromKey = toDateKey(weekStart);
  const weekToKey = toDateKey(weekEnd);
  const weekHistoryFromKey = useMemo(() => {
    const previousDay = new Date(weekStart);
    previousDay.setDate(previousDay.getDate() - 1);
    return toDateKey(previousDay);
  }, [weekStart]);

  // Dieselbe Nutzlast wie die Tagestabelle unter demselben Key: die
  // Tagesprojektion (#2443). Hier zählt nur ihr Soll — aber ein SWR-Key trägt
  // EINEN Datentyp, und wer sonst das Rennen verliert, liest die Form des
  // anderen (siehe Kommentar unten zu Sessions/Abwesenheiten).
  const { data: weekProjection } = useSWRAuth<
    ReadonlyMap<string, DayProjection>
  >(
    staffId
      ? `staff-schedule-targets-${staffId}-${weekFromKey}-${weekToKey}`
      : `time-tracking-schedule-targets-${weekFromKey}-${weekToKey}`,
    () =>
      staffId
        ? staffMonthSummaryService.getDailyProjection(
            staffId,
            weekFromKey,
            weekToKey,
          )
        : timeTrackingService.getDailyProjection(weekFromKey, weekToKey),
    { revalidateOnFocus: false },
  );
  const weekTargets = useMemo(
    () => (weekProjection ? targetsFromProjection(weekProjection) : undefined),
    [weekProjection],
  );

  // Sessions and absences are fetched per portal rather than behind one
  // branching fetcher: the own-portal keys are SHARED with the Zeiterfassung
  // table, and an SWR key is one cache entry — handing the same key two
  // different payload shapes would leave whichever consumer lost the race
  // reading the other's shape. So each key keeps its portal's native shape and
  // the adapting happens below, after the cache.
  //
  // While a session is open, /history computes its net_minutes at request time
  // only — the number is stale the second the response lands. The month card
  // and the Stundenkonto both poll, so without the same cadence here the week
  // card freezes at check-in time while the other two keep climbing, and the
  // screen shows contradicting live totals until the next mutation. Polling is
  // gated on an actually open session: a week of closed sessions is final and
  // refetching it every minute would be pure noise. The keys are shared with
  // the Zeiterfassung table, which shows the same open session and benefits.
  const refreshInterval = (
    latest: readonly StaffHistorySession[] | undefined,
  ) => (latest?.some((s) => !s.check_out_time) ? OPEN_MONTH_REFRESH_MS : 0);

  const { data: adminSessions } = useSWRAuth<readonly StaffHistorySession[]>(
    staffId
      ? `staff-history-${staffId}-${weekHistoryFromKey}-${weekToKey}`
      : null,
    () =>
      staffHistoryService.getHistory(
        staffId as string,
        weekHistoryFromKey,
        weekToKey,
      ),
    { refreshInterval },
  );
  const { data: ownHistory } = useSWRAuth<{
    sessions: WorkSessionHistory[];
    weeklySummaries: WeeklySummary[];
  }>(
    staffId ? null : `time-tracking-table-${weekHistoryFromKey}-${weekToKey}`,
    () => timeTrackingService.getHistory(weekHistoryFromKey, weekToKey),
    {
      refreshInterval: (latest) =>
        latest?.sessions.some((s) => !s.checkOutTime)
          ? OPEN_MONTH_REFRESH_MS
          : 0,
    },
  );

  const { data: adminAbsences } = useSWRAuth<readonly StaffAbsenceRow[]>(
    staffId ? `staff-absences-${staffId}-${weekFromKey}-${weekToKey}` : null,
    () =>
      staffAbsenceService.getAbsences(
        staffId as string,
        weekFromKey,
        weekToKey,
      ),
  );
  const { data: ownAbsences } = useSWRAuth<StaffAbsence[]>(
    staffId ? null : `time-tracking-table-absences-${weekFromKey}-${weekToKey}`,
    () => timeTrackingService.getAbsences(weekFromKey, weekToKey),
  );

  const weekSessions = useMemo<readonly StaffHistorySession[] | undefined>(
    () =>
      staffId
        ? adminSessions
        : ownHistory?.sessions.map(adaptHistorySessionForMetrics),
    [staffId, adminSessions, ownHistory],
  );
  const weekAbsences = useMemo<readonly StaffAbsenceRow[] | undefined>(
    () => (staffId ? adminAbsences : ownAbsences?.map(adaptAbsenceForMetrics)),
    [staffId, adminAbsences, ownAbsences],
  );

  const { data: config } = useSWRAuth("time-tracking-config", () =>
    timeTrackingService.getConfig(),
  );
  const { balanceMinutes: accountBalanceMinutes } = useAccountBalance(staffId);

  const week = useMemo<PeriodTotals | null>(() => {
    // No Soll, no week card: showing Ist against a 0h Soll would read as a
    // pile of Überstunden, and applying the current schedule as a stand-in is
    // exactly the contradiction this hook exists to remove.
    if (!weekTargets || !weekSessions) return null;
    const effectiveEnd = today < weekEnd ? today : weekEnd;
    return computePeriodTotalsFromTargets(
      weekTargets,
      weekSessions,
      weekAbsences,
      weekStart,
      weekEnd,
      effectiveEnd,
    );
  }, [weekTargets, weekSessions, weekAbsences, weekStart, weekEnd, today]);

  const month = useMemo<PeriodTotals | null>(() => {
    if (!monthSummary) return null;
    const credited =
      monthSummary.creditedSickMinutes +
      monthSummary.creditedVacationMinutes +
      monthSummary.creditedTrainingMinutes +
      monthSummary.creditedOtherMinutes;
    return {
      // Full month Soll, mirroring the week card's "Ist von Soll" reading.
      soll: monthSummary.targetMinutes,
      // Ansatz B: absence credits count towards Ist, same as in the week.
      ist: monthSummary.actualMinutes + credited,
      // Already priced against the Soll up to today, server-side.
      delta: monthSummary.balanceMinutes,
    };
  }, [monthSummary]);

  const accountStart = useMemo(
    () => resolveAccountStartDate(today, config?.accountStartDate),
    [today, config?.accountStartDate],
  );

  return { week, month, accountStart, accountBalanceMinutes };
}
