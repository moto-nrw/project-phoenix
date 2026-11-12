import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";

interface PlacedBooking {
  readonly placement: "placed" | "waitlisted" | "pending";
  readonly waitlistPosition: number | null;
  readonly promotedAt: string | null;
}

/**
 * The colour of a booking the office has not decided yet (#4700). Purple
 * stands apart from the place (green), the waiting list (yellow) and the
 * received registration (blue) beside it.
 */
export const HOLIDAY_PENDING_TONE: StatusBadgeTone = "purple";

export function holidayPlacementStatus(booking: PlacedBooking): {
  label: string;
  tone: StatusBadgeTone;
} {
  if (booking.placement === "pending") {
    return { label: "Platz offen", tone: HOLIDAY_PENDING_TONE };
  }
  if (booking.placement === "waitlisted") {
    return {
      label: booking.waitlistPosition
        ? `Warteliste, Platz ${booking.waitlistPosition}`
        : "Warteliste",
      tone: "warning",
    };
  }
  return {
    label: booking.promotedAt
      ? `Platz, nachgerückt am ${formatBerlinDate(booking.promotedAt)}`
      : "Platz",
    tone: "green",
  };
}
