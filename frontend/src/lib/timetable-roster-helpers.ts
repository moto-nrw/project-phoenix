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
