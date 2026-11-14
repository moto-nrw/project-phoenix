"use client";

import { useCallback, useMemo } from "react";

import { useAccountBalance } from "~/lib/hooks/use-account-balance";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { parseISODate } from "~/lib/date-helpers";
import { staffMonthSummaryService } from "~/lib/staff-api";
import {
  endOfWeek,
  periodTotalsFromProjection,
  resolveAccountStartDate,
  startOfWeek,
  toDateKey,
  type PeriodTotals,
} from "~/lib/staff-metrics-helpers";
import { useLiveRefreshInterval, useSWRAuth } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import {
  OPEN_MONTH_REFRESH_MS,
  type DayProjection,
  type MonthSummary,
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
  /** A Sonderarbeitszeit sets the Soll of a day in the current week. */
  readonly hasTargetOverride?: boolean;
  /** False when no Soll is maintained for the current month (#6887);
   *  undefined while the Monatskarte loads. */
  readonly hasTarget?: boolean;
  /** A source failed to load; the null figures will not arrive. */
  readonly failed?: boolean;
  /** The source error for the in-place load error display. */
  readonly error?: unknown;
  /** Revalidates the failed source. */
  readonly retry: () => Promise<void>;
}

/**
 * Date-valid Soll/Ist/Saldo for the CURRENT week and month, plus the
 * Stundenkonto (#1842).
 *
 * Every figure here is priced against the schedule that was valid on each
 * individual day, which is the whole point: the removed `computeStaffMetrics`
 * saw ONE StaffSchedule and applied it to the entire range, so a week or
 * month spanning a contract change (8h -> 4h) gets re-priced at today's hours.
 * Rendering that next to the date-valid Monatskarte and the date-valid daily
 * rows put two contradicting totals for the same period on one screen.
 *
 * Sources, all server-computed:
 * - month  -> the current month's Monatskarte (`month-summary`)
 * - week   -> the sum of `schedule-targets` (the per-day projection: Soll,
 *             Gutschrift, Ist, Saldo) over the week. The server prices each
 *             day with the helpers the Monatskarte sums, so the week can no
 *             longer credit an absence differently than the month (#6847).
 * - Saldo  -> `useAccountBalance`
 *
 * Pass a `staffId` for the admin (`time_tracking:manage`) endpoints; omit it to
 * read the caller's own figures. Keys deliberately match the ones the
 * Zeiterfassung table and Monatskarte already use, so SWR dedupes the overlap
 * into one request and the existing save/check-in invalidation patterns
 * ("staff-schedule-targets-", "time-tracking-schedule-targets-", …) refresh
 * these cards too.
 */
export function usePeriodMetrics(staffId?: string): PeriodMetrics {
  const todayISO = useBerlinToday();
  const today = useMemo(() => parseISODate(todayISO), [todayISO]);
  const year = Number(todayISO.slice(0, 4));
  const monthNumber = Number(todayISO.slice(5, 7));
  // staff_time_tracking_changed invalidates every key here; the interval
  // follows the growing Ist of an open block (#5017).
  const openMonthRefreshMs = useLiveRefreshInterval(OPEN_MONTH_REFRESH_MS);

  // The month card reads the same aggregate as the Monatskarte, under the same
  // key. No config gating: the backend resolves the account anchor itself, and
  // Soll/Ist/Saldo of a single month are valid regardless of where the
  // cumulative chain starts.
  const {
    data: monthSummary,
    error: monthSummaryError,
    mutate: mutateMonthSummary,
  } = useSWRAuth<MonthSummary>(
    staffId
      ? `staff-month-summary-${staffId}-${year}-${monthNumber}`
      : `time-tracking-month-summary-${year}-${monthNumber}`,
    () =>
      staffId
        ? staffMonthSummaryService.getMonthSummary(staffId, year, monthNumber)
        : timeTrackingService.getMonthSummary(year, monthNumber),
    // The current month is always open: a running session grows Ist
    // server-side, so the card has to keep up.
    { refreshInterval: openMonthRefreshMs },
  );

  const weekStart = useMemo(() => startOfWeek(today), [today]);
  const weekEnd = useMemo(() => endOfWeek(today), [today]);
  const weekFromKey = toDateKey(weekStart);
  const weekToKey = toDateKey(weekEnd);

  // Dieselbe Nutzlast wie die Tagestabelle unter demselben Key: die
  // Tagesprojektion (#2443). Die Wochenkachel ist ihre Summe, also zeigen
  // Tabelle und Kachel dieselben Zahlen.
  const {
    data: weekProjection,
    error: weekProjectionError,
    mutate: mutateWeekProjection,
  } = useSWRAuth<ReadonlyMap<string, DayProjection>>(
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
    // The current week is always open: a running session grows the day's Ist
    // server-side, and the month card and the Stundenkonto beside this card
    // poll at the same cadence.
    { revalidateOnFocus: false, refreshInterval: openMonthRefreshMs },
  );

  const { data: config } = useSWRAuth("time-tracking-config", () =>
    timeTrackingService.getConfig(),
  );
  const {
    balanceMinutes: accountBalanceMinutes,
    error: balanceError,
    retry: retryAccountBalance,
  } = useAccountBalance(staffId);

  // A failed refresh keeps SWR's cached projection. The week stays
  // unavailable then: the cached days may miss a new sick day's credit and
  // would read as a reliable value (#3885).
  const week = useMemo<PeriodTotals | null>(
    () =>
      weekProjection && !weekProjectionError
        ? periodTotalsFromProjection(weekProjection)
        : null,
    [weekProjection, weekProjectionError],
  );

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

  // A Sonderarbeitszeit (#3259) in the current week changes where its Soll
  // comes from; the origin chip names it.
  const hasTargetOverride = useMemo(
    () =>
      weekProjection
        ? [...weekProjection.values()].some((day) => day.isOverride === true)
        : false,
    [weekProjection],
  );

  // The figures stay null when a source failed; the caller must not wait for
  // them forever (#2514).
  const error = monthSummaryError ?? weekProjectionError ?? balanceError;
  const failed = Boolean(error);
  const retry = useCallback(async () => {
    const retries: Promise<unknown>[] = [];
    if (monthSummaryError) retries.push(mutateMonthSummary());
    if (weekProjectionError) retries.push(mutateWeekProjection());
    if (balanceError) retries.push(retryAccountBalance());
    await Promise.all(retries);
  }, [
    monthSummaryError,
    weekProjectionError,
    balanceError,
    mutateMonthSummary,
    mutateWeekProjection,
    retryAccountBalance,
  ]);

  return {
    week,
    month,
    accountStart,
    accountBalanceMinutes,
    hasTargetOverride,
    hasTarget: monthSummary?.hasTarget,
    failed,
    error,
    retry,
  };
}
