import type { DisplayMode } from "@/lib/location-helper";
import { resolvePresence, type PresenceStudent } from "@/lib/presence-status";
import {
  PresencePill,
  type PresencePillSize,
  type PresencePillVariant,
} from "./presence-pill";

interface LocationBadgeProps {
  /** The child as loaded; today's plan is read from its planning fields. */
  readonly student: PresenceStudent;
  readonly displayMode: DisplayMode;
  readonly userGroups?: string[];
  readonly groupRooms?: string[]; // Räume der eigenen OGS-Gruppen (für grüne Farbe)
  readonly supervisedRooms?: string[];
  readonly isGroupRoom?: boolean;
  readonly variant?: PresencePillVariant;
  readonly size?: PresencePillSize;
  /** Show "seit XX:XX Uhr" below the badge where the status allows it. Default: false */
  readonly showLocationSince?: boolean;
}

/**
 * The detailed presence badge: room, status or group name as the pill, with
 * the day status and today's plan from the presence status table
 * (`resolvePresence`).
 */
export function LocationBadge({
  student,
  displayMode,
  userGroups,
  groupRooms,
  supervisedRooms,
  isGroupRoom,
  variant = "modern",
  size = "md",
  showLocationSince = false,
}: LocationBadgeProps) {
  const presence = resolvePresence(student, {
    mode: "detailed",
    displayMode,
    userGroups,
    groupRooms,
    supervisedRooms,
    isGroupRoom,
  });
  return (
    <PresencePill
      presence={presence}
      size={size}
      variant={variant}
      showLocationSince={showLocationSince}
    />
  );
}
