"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import type { KeyedMutator } from "swr";

import { hasPermission } from "~/lib/auth-utils";
import { staffShiftService } from "~/lib/shift-api";
import {
  groupShiftsByStaffAndDate,
  type StaffScheduleAssignment,
  type StaffScheduleOverview,
  type StaffScheduleStaff,
  type StaffShift,
  type StaffWeeklySummary,
} from "~/lib/shift-helpers";
import { shiftTypeService } from "~/lib/shift-type-api";
import { indexShiftTypes, type ShiftType } from "~/lib/shift-type-helpers";
import { staffService, type Staff } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import {
  VERTRETUNG_GAPS_KEY_PREFIX,
  VERTRETUNG_WEEK_KEY_PREFIX,
} from "~/lib/timetable-helpers";

/**
 * Every SWR cache-key prefix a shift or absence change on the Dienstplan can
 * affect: this week's Dienstplan (both permission paths), the Vertretungsplan
 * (coverage/gaps derive from shifts), and the absence lists surfaced on the
 * staff detail and time-tracking surfaces (#1843). Shared by the sick-report
 * flow (refreshPlanCaches) and the resource grid's post-move refresh so both
 * invalidate the identical set.
 */
export const PLAN_CACHE_KEY_PREFIXES = [
  "dienstplan-overview-",
  "dienstplan-shifts-",
  VERTRETUNG_WEEK_KEY_PREFIX,
  VERTRETUNG_GAPS_KEY_PREFIX,
  "staff-pending-absences-",
  "time-tracking-absences-",
  "time-tracking-table-absences-",
  "time-tracking-own-absences-",
  "staff-shifts-visible-",
  "time-tracking-own-shifts-today-",
] as const;

// Groups Betreuungsplan assignments (only present via the full overview
// path) by staff and calendar day, mirroring groupShiftsByStaffAndDate for
// shifts. Kept file-local: nothing outside the hook needs this shape.
function groupAssignmentsByStaffAndDate(
  assignments: readonly StaffScheduleAssignment[],
): Map<string, Map<string, StaffScheduleAssignment[]>> {
  const byStaff = new Map<string, Map<string, StaffScheduleAssignment[]>>();
  for (const assignment of assignments) {
    let byDate = byStaff.get(assignment.staffId);
    if (!byDate) {
      byDate = new Map<string, StaffScheduleAssignment[]>();
      byStaff.set(assignment.staffId, byDate);
    }
    const dayAssignments = byDate.get(assignment.date);
    if (dayAssignments) {
      dayAssignments.push(assignment);
    } else {
      byDate.set(assignment.date, [assignment]);
    }
  }
  return byStaff;
}

export interface UseDienstplanDataResult {
  /** `time_tracking:manage` — gates shift editing and the sick-report
   *  context action; the caller uses it to decide whether to wire up
   *  `onSickReport`. */
  canManageAbsences: boolean;
  /** True when the caller lacks the full overview permission set
   *  (`schedules:read` + `users:read` on top of `time_tracking:manage`), i.e.
   *  the reduced staff/shifts-only path. Inverse of `canUseAssignmentOverview`;
   *  the grid collapses each row header to the name only on this path. */
  reducedPath: boolean;
  /** Staff rows for the grid, alphabetical by last name (German collation). */
  sortedStaff: StaffScheduleStaff[];
  shiftsByStaff: Map<string, Map<string, StaffShift[]>>;
  /** Empty on the reduced permission path (no `schedules:read`). */
  assignmentsByStaff: Map<string, Map<string, StaffScheduleAssignment[]>>;
  /** Weekly planned/target summary per staff, already filtered to the
   *  requested week (its `weekStart` equals `weekFrom`). Empty on the
   *  reduced permission path. */
  summaryByStaff: Map<string, StaffWeeklySummary>;
  typesById: Map<string, ShiftType>;
  /** Every shift in the requested week — used to resolve the replacement
   *  shifts attached to a cancelled shift when opening the edit modal. */
  allShifts: StaffShift[];
  shiftTypes: ShiftType[] | undefined;
  shiftTypesError: Error | undefined;
  shiftTypesLoading: boolean;
  mutateShiftTypes: KeyedMutator<ShiftType[]>;
  /** Set when the active data path (overview, or legacy staff/shifts) failed
   *  to load; the caller replaces the grid with a blocking error state. */
  scheduleError: Error | undefined;
  scheduleLoading: boolean;
  /** Re-fetches whichever data path is currently active. */
  retryLoad: () => void;
  /** Invalidates every cache a sick report can affect: this week's
   *  Dienstplan, the Vertretungsplan, and the absence lists surfaced
   *  elsewhere (#1843). */
  refreshPlanCaches: () => Promise<unknown>;
}

/**
 * Data layer for the Dienstplan week view (#1376; hook extraction #1886).
 *
 * Encapsulates both permission-gated data paths — the full overview
 * (`time_tracking:manage` + `schedules:read` + `users:read`, one
 * `staffShiftService.getOverview` call) vs. the reduced staff/shifts-only
 * path (`time_tracking:manage` only, `staffService.getAllStaff` +
 * `staffShiftService.getShifts`) — plus their SWR keys, mutate functions, and
 * the per-staff/day lookups the week grid renders from.
 *
 * Week boundaries are the caller's concern: pass the Monday/Friday of the
 * displayed week as `"YYYY-MM-DD"`. The hook does not own week navigation so
 * that swapping the week state for URL state never has to touch this file.
 */
export function useDienstplanData(
  weekFrom: string,
  weekTo: string,
): UseDienstplanDataResult {
  const { data: session } = useSession();
  const canManageAbsences = hasPermission(session, "time_tracking:manage");
  const canUseAssignmentOverview =
    canManageAbsences &&
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "users:read");

  // A sick report cancels shifts (Dienstplan) and marks Betreuungsblöcke
  // absent (Vertretungsplan) server-side, so revalidate every cache that
  // renders those rows plus the absence lists on the staff detail /
  // self-service surfaces (#1843).
  const refreshPlanCaches = useTenantMutateMatching(PLAN_CACHE_KEY_PREFIXES);

  const {
    data: overview,
    error: overviewError,
    isLoading: overviewLoading,
    mutate: mutateOverview,
  } = useSWRAuth<StaffScheduleOverview>(
    canUseAssignmentOverview
      ? `dienstplan-overview-${weekFrom}-${weekTo}`
      : null,
    () => staffShiftService.getOverview(weekFrom, weekTo),
  );

  const {
    data: legacyStaff,
    error: legacyStaffError,
    isLoading: legacyStaffLoading,
    mutate: mutateLegacyStaff,
  } = useSWRAuth<Staff[]>(
    canUseAssignmentOverview ? null : "dienstplan-staff",
    () => staffService.getAllStaff(),
  );

  const {
    data: legacyShifts,
    error: legacyShiftsError,
    isLoading: legacyShiftsLoading,
    mutate: mutateLegacyShifts,
  } = useSWRAuth<StaffShift[]>(
    canUseAssignmentOverview ? null : `dienstplan-shifts-${weekFrom}-${weekTo}`,
    () => staffShiftService.getShifts(weekFrom, weekTo),
  );

  const {
    data: shiftTypes,
    error: shiftTypesError,
    isLoading: shiftTypesLoading,
    mutate: mutateShiftTypes,
  } = useSWRAuth<ShiftType[]>("dienstplan-shift-types", () =>
    shiftTypeService.getShiftTypes(),
  );

  const typesById = useMemo(
    () => indexShiftTypes(shiftTypes ?? []),
    [shiftTypes],
  );

  const sortedStaff = useMemo(() => {
    const staff: StaffScheduleStaff[] = canUseAssignmentOverview
      ? (overview?.staff ?? [])
      : (legacyStaff ?? []).map((member) => ({
          id: member.id,
          firstName: member.firstName,
          lastName: member.lastName,
        }));
    return [...staff].sort((a, b) =>
      `${a.lastName} ${a.firstName}`.localeCompare(
        `${b.lastName} ${b.firstName}`,
        "de",
      ),
    );
  }, [canUseAssignmentOverview, legacyStaff, overview?.staff]);

  const allShifts = useMemo(
    () =>
      canUseAssignmentOverview
        ? (overview?.shifts ?? [])
        : (legacyShifts ?? []),
    [canUseAssignmentOverview, legacyShifts, overview?.shifts],
  );

  const shiftsByStaff = useMemo(
    () => groupShiftsByStaffAndDate(allShifts),
    [allShifts],
  );

  const assignmentsByStaff = useMemo(
    () => groupAssignmentsByStaffAndDate(overview?.assignments ?? []),
    [overview?.assignments],
  );

  // Weekly planned/target summaries for the displayed week (weekFrom is its
  // Monday). Empty on the legacy no-overview path.
  const summaryByStaff = useMemo(() => {
    const byStaff = new Map<string, StaffWeeklySummary>();
    for (const summary of overview?.weeklySummaries ?? []) {
      if (summary.weekStart === weekFrom) {
        byStaff.set(summary.staffId, summary);
      }
    }
    return byStaff;
  }, [overview?.weeklySummaries, weekFrom]);

  const scheduleError = canUseAssignmentOverview
    ? overviewError
    : (legacyStaffError ?? legacyShiftsError);
  const scheduleLoading = canUseAssignmentOverview
    ? overviewLoading
    : legacyStaffLoading || legacyShiftsLoading;
  const retryLoad = () => {
    void Promise.all(
      canUseAssignmentOverview
        ? [mutateOverview()]
        : [mutateLegacyStaff(), mutateLegacyShifts()],
    );
  };
  return {
    canManageAbsences,
    reducedPath: !canUseAssignmentOverview,
    sortedStaff,
    shiftsByStaff,
    assignmentsByStaff,
    summaryByStaff,
    typesById,
    allShifts,
    shiftTypes,
    shiftTypesError,
    shiftTypesLoading,
    mutateShiftTypes,
    scheduleError,
    scheduleLoading,
    retryLoad,
    refreshPlanCaches,
  };
}
