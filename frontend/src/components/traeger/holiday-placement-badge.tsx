// Platz oder Warteliste einer gebuchten Ferienwoche (#4699), als Plakette
// neben der Woche. Auf der Warteliste steht der Rang dabei; ein nachgerücktes
// Kind zeigt, wann es den Platz bekommen hat.

import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";

interface PlacedBooking {
  readonly placement: "placed" | "waitlisted";
  readonly waitlistPosition: number | null;
  readonly promotedAt: string | null;
}

export function HolidayPlacementBadge({
  booking,
}: Readonly<{ booking: PlacedBooking }>) {
  if (booking.placement === "waitlisted") {
    return (
      <StatusBadge
        label={
          booking.waitlistPosition
            ? `Warteliste, Platz ${booking.waitlistPosition}`
            : "Warteliste"
        }
        tone="warning"
        size="xs"
      />
    );
  }
  return (
    <StatusBadge
      label={
        booking.promotedAt
          ? `Platz, nachgerückt am ${formatBerlinDate(booking.promotedAt)}`
          : "Platz"
      }
      tone="green"
      size="xs"
    />
  );
}
