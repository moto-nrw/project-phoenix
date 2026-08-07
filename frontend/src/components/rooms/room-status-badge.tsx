// Shared "Belegt" / "Frei" pill used by the rooms grid card and the room
// detail header. Previously duplicated (page.tsx card badge vs.
// room-detail-content.tsx StatusBadge) with slightly different sizing and one
// copy hardcoding a raw hex for the "frei" text color instead of the
// moto-green-strong token.
import { cn } from "~/lib/utils";

interface RoomStatusBadgeProps {
  readonly isOccupied: boolean;
  readonly size?: "sm" | "md";
  readonly className?: string;
}

export function RoomStatusBadge({
  isOccupied,
  size = "md",
  className,
}: RoomStatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full font-semibold",
        size === "sm" ? "px-2.5 py-1 text-xs" : "px-3 py-1.5 text-xs",
        // Both labels take the -strong ramp step: the base hue on its own 15%
        // tint is 3.81:1, under the 4.5:1 AA bar the "Frei" side already meets.
        isOccupied
          ? "bg-moto-red/15 text-moto-red-strong"
          : "bg-moto-green/15 text-moto-green-strong",
        className,
      )}
    >
      <span
        className={cn(
          "rounded-full",
          size === "sm" ? "mr-1.5 h-1.5 w-1.5" : "mr-2 h-2 w-2",
          isOccupied ? "bg-moto-red animate-pulse" : "bg-moto-green",
        )}
      />
      {isOccupied ? "Belegt" : "Frei"}
    </span>
  );
}
