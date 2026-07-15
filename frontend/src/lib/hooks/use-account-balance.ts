"use client";

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
 */
export function useAccountBalance(staffId?: string): AccountBalance {
  const now = new Date();
  const year = now.getFullYear();
  const month = now.getMonth() + 1;

  const { data, isLoading, error } = useSWRAuth<MonthSummary>(
    staffId
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
    balanceMinutes: data?.closingBalanceMinutes ?? null,
    isLoading,
    error,
  };
}
