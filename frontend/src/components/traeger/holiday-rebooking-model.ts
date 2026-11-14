/**
 * Model of the rebooking of a child (#4663): one choice per week of the
 * period, a time model of that week or "not registered", the days of a
 * week by days (#4701) and the lunch of a week with lunch (#5631).
 */

import type { HolidayPeriod } from "~/lib/traeger-api";
import type {
  HolidayBookingChange,
  HolidayRegistrationChild,
} from "~/lib/traeger-holiday-registrations-api";

/** Week ID → time model ID, "" for a week the child is not registered for. */
export type HolidayRebookingDraft = Readonly<Record<string, string>>;

/** Week ID → the chosen days of a week by days. */
export type HolidayRebookingDays = Readonly<Record<string, readonly string[]>>;

export function rebookingDraft(
  period: HolidayPeriod,
  child: HolidayRegistrationChild,
): HolidayRebookingDraft {
  const draft: Record<string, string> = {};
  for (const week of period.weeks) {
    if (!week.id) continue;
    draft[week.id] =
      child.bookings.find((booking) => booking.weekId === week.id)
        ?.timeModelId ?? "";
  }
  return draft;
}

/** The booked days of the child per week by days. */
export function rebookingDays(
  child: HolidayRegistrationChild,
): HolidayRebookingDays {
  const days: Record<string, readonly string[]> = {};
  for (const booking of child.bookings) {
    if (booking.days && booking.days.length > 0) {
      days[booking.weekId] = booking.days;
    }
  }
  return days;
}

/** Week ID → the child eats lunch in the week. */
export type HolidayRebookingLunch = Readonly<Record<string, boolean>>;

/** The weeks the child has lunch in. */
export function rebookingLunch(
  child: HolidayRegistrationChild,
): HolidayRebookingLunch {
  const lunch: Record<string, boolean> = {};
  for (const booking of child.bookings) {
    if (booking.lunch) lunch[booking.weekId] = true;
  }
  return lunch;
}

/**
 * The bookings of a draft, in the order of the weeks. Lunch counts only in a
 * week that offers it.
 */
export function rebookingBookings(
  period: HolidayPeriod,
  draft: HolidayRebookingDraft,
  days: HolidayRebookingDays = {},
  lunch: HolidayRebookingLunch = {},
): HolidayBookingChange[] {
  const bookings: HolidayBookingChange[] = [];
  for (const week of period.weeks) {
    const timeModelId = week.id ? draft[week.id] : "";
    if (!week.id || !timeModelId) continue;
    const withLunch =
      week.lunchPriceCents != null && lunch[week.id] ? { lunch: true } : {};
    if (week.bookingUnit === "day") {
      bookings.push({
        weekId: week.id,
        timeModelId,
        days: [...(days[week.id] ?? [])].sort(),
        ...withLunch,
      });
    } else {
      bookings.push({ weekId: week.id, timeModelId, ...withLunch });
    }
  }
  return bookings;
}

/** A booked week by days without a day: the draft cannot be saved yet. */
export function rebookingMissesDays(
  bookings: readonly HolidayBookingChange[],
): boolean {
  return bookings.some(
    (booking) => booking.days !== undefined && booking.days.length === 0,
  );
}

function sameDays(
  a: readonly string[] | undefined,
  b: readonly string[] | undefined,
): boolean {
  const left = [...(a ?? [])].sort();
  const right = [...(b ?? [])].sort();
  return (
    left.length === right.length && left.every((day, i) => day === right[i])
  );
}

/**
 * Whether the draft books other weeks, time models, days or another lunch
 * than the child.
 */
export function rebookingChanged(
  child: HolidayRegistrationChild,
  bookings: readonly HolidayBookingChange[],
): boolean {
  if (bookings.length !== child.bookings.length) return true;
  return bookings.some(
    (booking) =>
      !child.bookings.some(
        (current) =>
          current.weekId === booking.weekId &&
          current.timeModelId === booking.timeModelId &&
          sameDays(current.days, booking.days) &&
          (current.lunch ?? false) === (booking.lunch ?? false),
      ),
  );
}
