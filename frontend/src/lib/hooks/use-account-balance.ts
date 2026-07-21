"use client";

import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { staffMonthSummaryService } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { timeTrackingService } from "~/lib/time-tracking-api";
import {
  OPEN_MONTH_REFRESH_MS,
  type MonthSummary,
} from "~/lib/time-tracking-helpers";

export interface AccountBalance {
  /** Cumulative Saldo in minutes, or null while loading / on error. */
  readonly balanceMinutes: number | null;
  readonly isLoading: boolean;
  readonly error: unknown;
}

/**
 * Live Stundenkonto (cumulative Saldo since the account start) for a staff
 * member, read from the server-computed Monatskarte model (#1842).
 *
 * `computeStaffMetrics` cannot produce this number correctly: its only
 * schedule input is the CURRENT StaffSchedule, which it applies to every day
 * back to the account start. After a contract change (8h -> 4h) it re-prices
 * months of history at today's hours, so its `accountBalance` contradicted the
 * Monatskarte — both labelled "Stundenkonto" — by hours on the same screen.
 * The backend resolves each day against the schedule that was actually valid
 * then, and the current month's `closingBalanceMinutes` is that balance as of
 * today: exactly what the Monatskarte prints as "Stundenkonto Stand".
 *
 * Range note: the date-valid `schedule-targets` endpoint cannot serve this,
 * since it caps a request at 366 days while the account start is configurable
 * and may lie years back. The carry chain behind `closingBalanceMinutes` walks
 * those months server-side instead.
 *
 * Pass a `staffId` for the admin (`time_tracking:manage`) endpoint; omit it to
 * read the caller's own account. The keys deliberately match the Monatskarte's
 * current-month keys so SWR dedupes both widgets into one request.
 *
 * The month is the BERLIN month, not the browser's: the backend derives its
 * "current month" from `timezone.TodayDate()`, so a browser in another
 * timezone would ask for the wrong month around a month boundary (shortly
 * after Berlin midnight on Aug 1, a UTC browser still says July) and print
 * last month's closing balance under a key that no longer matches the
 * Monatskarte. `useBerlinToday` also re-renders on the rollover itself.
 *
 * An account start in the FUTURE — a later month, or a later day of the
 * current month — yields no balance. The backend summarizes months before the
 * anchor standalone (full month, no carry), so `closingBalanceMinutes` is then
 * this month's own Saldo — not a cumulative account balance — and labelling it
 * "Stundenkonto seit <future date>" would be materially wrong. Consumers
 * already render `null` as "–". The anchor is required input, so a failed
 * config read is surfaced rather than swallowed, mirroring the backend's
 * `chainAnchor`.
 */
export function useAccountBalance(staffId?: string): AccountBalance {
  const today = useBerlinToday();
  const year = Number(today.slice(0, 4));
  const month = Number(today.slice(5, 7));

  const {
    data: config,
    isLoading: configLoading,
    error: configError,
  } = useSWRAuth("time-tracking-config", () => timeTrackingService.getConfig());

  // ISO "YYYY-MM-DD" compares lexicographically. An empty setting means "no
  // anchor configured" — the backend then starts the chain on January 1st.
  // The comparison is on the FULL date, not the month: an account starting
  // later in the CURRENT month has not started either, and the backend then
  // aggregates an empty span (every day of the anchor month before the start
  // day carries no Soll and no Ist) and answers a clean 0 — which the widget
  // would print as a real "Stundenkonto Stand: 0:00" days before the account
  // exists (#1842).
  const anchor = config?.accountStartDate ?? "";
  const anchorIsInFuture = anchor !== "" && anchor > today;
  const canResolveAccount = !configLoading && !configError && !anchorIsInFuture;

  const { data, isLoading, error } = useSWRAuth<MonthSummary>(
    !canResolveAccount
      ? null
      : staffId
        ? `staff-month-summary-${staffId}-${year}-${month}`
        : `time-tracking-month-summary-${year}-${month}`,
    () =>
      staffId
        ? staffMonthSummaryService.getMonthSummary(staffId, year, month)
        : timeTrackingService.getMonthSummary(year, month),
    // The current month is always open: a running session grows Ist
    // server-side, so the Saldo has to keep up.
    { refreshInterval: OPEN_MONTH_REFRESH_MS },
  );

  return {
    balanceMinutes: canResolveAccount
      ? (data?.closingBalanceMinutes ?? null)
      : null,
    isLoading: configLoading || (canResolveAccount && isLoading),
    error: configError ?? error,
  };
}
