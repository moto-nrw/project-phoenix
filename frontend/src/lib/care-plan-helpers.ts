/**
 * Pure, React-free helpers for the per-child Betreuungsplan view.
 *
 * The backend endpoint (student-care-plan-api.ts) returns the resolved plan —
 * arrival, activity instances, pickup — but neither an explicit "free care"
 * (Freispiel) entity nor the broad sick/excused/class-trip day deviations.
 * These helpers compose both on the frontend from data already available:
 *  - free-care bands are the gaps inside the arrival→pickup window not covered
 *    by a non-cancelled instance;
 *  - day deviations come from the student's status days (already fetched by the
 *    student detail page) overlaid onto the plan date.
 */

import type {
  CarePlanDay,
  CarePlanInstance,
} from "~/lib/student-care-plan-api";
import type {
  StudentStatusDay,
  StudentStatusKind,
} from "~/lib/student-status-days-api";
import { parseTimeToMinutes } from "~/lib/timetable-helpers";

/** A stretch of supervised time with no planned activity. */
export interface FreeCareBand {
  startTime: string; // HH:MM
  endTime: string; // HH:MM
}

/** Resolved deviation for one day, for the banner at the top of a day. */
export interface DayDeviation {
  kind: StudentStatusKind;
  label: string;
  note?: string | null;
}

/**
 * Gaps below this many minutes are treated as transitions, not free care, so
 * a 5-minute seam between back-to-back blocks does not render its own band.
 */
const MIN_FREE_CARE_MINUTES = 10;

function toHHMM(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}`;
}

/** An instance occupies its slot unless it was cancelled. */
function isOccupying(instance: CarePlanInstance): boolean {
  return instance.status !== "cancelled";
}

/** Merge overlapping/adjacent [start,end] minute intervals into a sorted list. */
function mergeIntervals(
  intervals: Array<[number, number]>,
): Array<[number, number]> {
  if (intervals.length === 0) return [];
  const sorted = [...intervals].sort((a, b) => a[0] - b[0]);
  const merged: Array<[number, number]> = [sorted[0]!];
  for (let i = 1; i < sorted.length; i++) {
    const cur = sorted[i]!;
    const last = merged[merged.length - 1]!;
    if (cur[0] <= last[1]) {
      last[1] = Math.max(last[1], cur[1]);
    } else {
      merged.push(cur);
    }
  }
  return merged;
}

/**
 * Compute the free-care bands for a day: the parts of the arrival→pickup
 * window not covered by a non-cancelled instance.
 *
 * The window is bounded by the arrival time (falling back to the earliest
 * instance start) and the pickup time (falling back to the latest instance
 * end). When neither bound can be resolved, or the window is empty, there are
 * no bands. Bands shorter than MIN_FREE_CARE_MINUTES are dropped as
 * transitions.
 */
export function computeFreeCareBands(day: CarePlanDay): FreeCareBand[] {
  const occupied = mergeIntervals(
    day.instances
      .filter(isOccupying)
      .map((i): [number, number] => [
        parseTimeToMinutes(i.startTime),
        parseTimeToMinutes(i.endTime),
      ])
      .filter(([s, e]) => !Number.isNaN(s) && !Number.isNaN(e) && e > s),
  );

  const arrivalMin =
    day.arrival.expectedTime != null
      ? parseTimeToMinutes(day.arrival.expectedTime)
      : NaN;
  const pickupMin =
    day.pickup.expectedTime != null
      ? parseTimeToMinutes(day.pickup.expectedTime)
      : NaN;

  const windowStart = !Number.isNaN(arrivalMin)
    ? arrivalMin
    : (occupied[0]?.[0] ?? NaN);
  const windowEnd = !Number.isNaN(pickupMin)
    ? pickupMin
    : (occupied[occupied.length - 1]?.[1] ?? NaN);

  if (Number.isNaN(windowStart) || Number.isNaN(windowEnd)) return [];
  if (windowEnd <= windowStart) return [];

  const bands: FreeCareBand[] = [];
  let cursor = windowStart;
  for (const [start, end] of occupied) {
    // Only intervals inside the window shrink the free time.
    if (end <= windowStart || start >= windowEnd) continue;
    const clampedStart = Math.max(start, windowStart);
    if (clampedStart - cursor >= MIN_FREE_CARE_MINUTES) {
      bands.push({ startTime: toHHMM(cursor), endTime: toHHMM(clampedStart) });
    }
    cursor = Math.max(cursor, Math.min(end, windowEnd));
  }
  if (windowEnd - cursor >= MIN_FREE_CARE_MINUTES) {
    bands.push({ startTime: toHHMM(cursor), endTime: toHHMM(windowEnd) });
  }
  return bands;
}

/**
 * Resolve the deviation to surface for a given day, if any. Prefers an active
 * status-day row for the date; falls back to the live sick/excused flags only
 * for today (those flags are today-scoped on the student). A cleared status
 * row (cleared_at set) is ignored.
 */
export function resolveDayDeviation(
  date: string,
  statusDays: readonly StudentStatusDay[],
  options?: { isSick?: boolean; isExcused?: boolean; isToday?: boolean },
): DayDeviation | null {
  const row = statusDays.find((s) => s.date === date && !s.cleared_at);
  if (row) {
    return { kind: row.status, label: row.label, note: row.note };
  }
  if (options?.isToday) {
    if (options.isSick) return { kind: "sick", label: "Krank" };
    if (options.isExcused) return { kind: "excused", label: "Entschuldigt" };
  }
  return null;
}
