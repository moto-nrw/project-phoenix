/**
 * Model of the rebooking of a child (#4663): one choice per week of the
 * period, a time model of that week or "not registered", and the days of a
 * week by days (#4701).
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

/** The bookings of a draft, in the order of the weeks. */
export function rebookingBookings(
  period: HolidayPeriod,
  draft: HolidayRebookingDraft,
  days: HolidayRebookingDays = {},
): HolidayBookingChange[] {
  const bookings: HolidayBookingChange[] = [];
  for (const week of period.weeks) {
    const timeModelId = week.id ? draft[week.id] : "";
    if (!week.id || !timeModelId) continue;
    if (week.bookingUnit === "day") {
      bookings.push({
        weekId: week.id,
        timeModelId,
        days: [...(days[week.id] ?? [])].sort(),
      });
    } else {
      bookings.push({ weekId: week.id, timeModelId });
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

/** Whether the draft books other weeks, time models or days than the child. */
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
          sameDays(current.days, booking.days),
      ),
  );
}
