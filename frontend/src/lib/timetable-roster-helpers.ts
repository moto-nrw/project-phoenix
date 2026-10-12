import { berlinTodayISO } from "./date-helpers";
import type { TimetableRosterRow } from "./timetable-operations-types";

const berlinTimeFormatter = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Europe/Berlin",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

/**
 * Expected arrival ("HH:MM") of an `arrival_after_slot_start` warning that is
 * still ahead on the roster's Berlin calendar date, else null. The zero-padded
 * wall-clock strings compare lexicographically once both dates match.
 */
export function upcomingArrivalTime(
  warnings: TimetableRosterRow["warnings"] | undefined,
  now: Date,
  rosterDate: string,
): string | null {
  if (rosterDate !== berlinTodayISO(now)) return null;
  const nowClock = berlinTimeFormatter.format(now);
  for (const warning of warnings ?? []) {
    if (warning.kind !== "arrival_after_slot_start") continue;
    if (warning.expectedArrival && warning.expectedArrival > nowClock) {
      return warning.expectedArrival;
    }
  }
  return null;
}

export function rosterPickupTimeLabel(
  pickupTime: string | null | undefined,
  pickupTimesLoaded: boolean | undefined,
  pickupTimesRedacted = false,
): string | null {
  if (pickupTimesRedacted) return null;
  if (pickupTimesLoaded === undefined) return null;
  if (!pickupTimesLoaded) return "Nicht geladen";
  return pickupTime ?? "—";
}

/**
 * Which present children the picker of a running block offers (#3824).
 * `stays` keeps the children whose Gehzeit today is still ahead, plus those
 * without a Gehzeit (nobody knows they are leaving); `all` keeps every present
 * child. Children already in the block are never offered.
 */
export type PresentChildScope = "stays" | "all";

export interface PresentChildCandidate {
  readonly id: string;
  readonly pickupTime?: string | null;
}

export function presentChildCandidates<T extends PresentChildCandidate>(
  students: readonly T[],
  inBlockStudentIds: ReadonlySet<string>,
  now: Date,
  scope: PresentChildScope,
): T[] {
  const nowClock = berlinTimeFormatter.format(now);
  return students.filter((student) => {
    if (inBlockStudentIds.has(student.id)) return false;
    if (scope === "all") return true;
    return !student.pickupTime || student.pickupTime > nowClock;
  });
}
