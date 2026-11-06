// Shared "Belegt" / "Frei" pill used by the rooms grid card and the room
// detail header, so both say the same thing in the same colour.
import { StatusBadge } from "~/components/ui/status-badge";

export function RoomStatusBadge({
  isOccupied,
}: {
  readonly isOccupied: boolean;
}) {
  return (
    <StatusBadge
      label={isOccupied ? "Belegt" : "Frei"}
      tone={isOccupied ? "red" : "green"}
    />
  );
}
