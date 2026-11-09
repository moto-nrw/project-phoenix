import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";

interface PlacedBooking {
  readonly placement: "placed" | "waitlisted";
  readonly waitlistPosition: number | null;
  readonly promotedAt: string | null;
}

export function holidayPlacementStatus(booking: PlacedBooking): {
  label: string;
  tone: StatusBadgeTone;
} {
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
