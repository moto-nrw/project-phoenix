"use client";

import { staffMonthSummaryService } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { staffMonthSummaryKeys } from "~/lib/swr/staff-keys";

/**
 * Whether a Soll is maintained for the staff member in the month of `date`
 * (#6887). It reads `hasTarget` of that month's Monatskarte, which the backend
 * derives from the same resolver it prices the month with, under the key the
 * Monatskarte already uses, so an open record shares the request.
 *
 * `enabled` is the caller's `time_tracking:manage`: the admin month summary
 * is gated on it, so without it nothing is requested.
 *
 * Returns null while unknown: disabled, no date, still loading or a failed
 * read. A dialog then shows no Soll hint at all; it is an extra notice, and
 * the dialog's own work does not depend on it, so a failed read stays silent.
 */
export function useStaffHasTarget(
  staffId: string,
  date: string,
  enabled: boolean,
): boolean | null {
  const year = Number(date.slice(0, 4));
  const month = Number(date.slice(5, 7));
  const valid = enabled && staffId !== "" && year > 0 && month > 0;
  const { data } = useSWRAuth(
    valid ? staffMonthSummaryKeys.month(staffId, year, month) : null,
    () => staffMonthSummaryService.getMonthSummary(staffId, year, month),
    { keepPreviousData: false },
  );
  return valid && data ? data.hasTarget : null;
}
