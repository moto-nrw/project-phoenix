/**
 * Model of the rebooking of a child (#4663): one choice per week of the
 * period, a time model of that week or "not registered".
 */

import type { HolidayPeriod } from "~/lib/traeger-api";
import type {
  HolidayBookingChange,
  HolidayRegistrationChild,
} from "~/lib/traeger-holiday-registrations-api";

/** Week ID → time model ID, "" for a week the child is not registered for. */
export type HolidayRebookingDraft = Readonly<Record<string, string>>;

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

/** The bookings of a draft, in the order of the weeks. */
export function rebookingBookings(
  period: HolidayPeriod,
  draft: HolidayRebookingDraft,
): HolidayBookingChange[] {
  const bookings: HolidayBookingChange[] = [];
  for (const week of period.weeks) {
    const timeModelId = week.id ? draft[week.id] : "";
    if (week.id && timeModelId) bookings.push({ weekId: week.id, timeModelId });
  }
  return bookings;
}

/** Whether the draft books other weeks or time models than the child. */
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
          current.timeModelId === booking.timeModelId,
      ),
  );
}
