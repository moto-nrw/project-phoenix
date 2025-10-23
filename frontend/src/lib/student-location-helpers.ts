// lib/student-location-helpers.ts
// Type definitions and helpers for student location tracking

import {
  STUDENT_LOCATION_BADGE_TOKENS,
  type StudentLocationBadgeThemeEntry,
} from "~/theme/student-location-status-tokens";

/**
 * Canonical student location states
 */
export type LocationState =
  | "PRESENT_IN_ROOM"  // Student is in a specific room
  | "TRANSIT"          // Student is checked in but between rooms
  | "SCHOOLYARD"       // Student is on the schoolyard
  | "HOME";            // Student is not checked in (at home/off-site)

/**
 * Room owner type
 */
export type RoomOwnerType = "GROUP" | "ACTIVITY";

/**
 * Room metadata for location status
 */
export interface LocationRoom {
  id: number;
  name: string;
  isGroupRoom: boolean;  // True if this is the student's educational group room
  ownerType: RoomOwnerType;
}

/**
 * Structured student location status
 * This is the canonical source of truth for student location data
 */
export interface StudentLocationStatus {
  state: LocationState;
  room?: LocationRoom;  // Present only when state is PRESENT_IN_ROOM or SCHOOLYARD
}

/**
 * Backend response types (snake_case from API)
 */
export interface BackendLocationRoom {
  id: number;
  name: string;
  is_group_room: boolean;
  owner_type: RoomOwnerType;
}

export interface BackendStudentLocationStatus {
  state: LocationState;
  room?: BackendLocationRoom;
}

/**
 * Badge styling configuration derived from centralized tokens.
 */
export interface StudentLocationBadgeConfig {
  label: string;
  colorToken: string;
  shadow: string;
  textClass: string;
}

export interface StudentLocationBadgeOptions {
  /**
   * Label used when no status information is available.
   * Defaults to "Zuhause".
   */
  fallbackLabel?: string;
}

/**
 * Maps backend location status to frontend format
 */
export function mapLocationStatus(
  backendStatus: BackendStudentLocationStatus | null | undefined
): StudentLocationStatus | null {
  if (!backendStatus) {
    return null;
  }

  const status: StudentLocationStatus = {
    state: backendStatus.state,
  };

  if (backendStatus.room) {
    status.room = {
      id: backendStatus.room.id,
      name: backendStatus.room.name,
      isGroupRoom: backendStatus.room.is_group_room,
      ownerType: backendStatus.room.owner_type,
    };
  }

  return status;
}

/**
 * Build a badge configuration from the canonical status using shared tokens.
 */
export function getStudentLocationBadge(
  status: StudentLocationStatus | null,
  options: StudentLocationBadgeOptions = {},
): StudentLocationBadgeConfig {
  const fallbackLabel = options.fallbackLabel ?? "Zuhause";

  const buildConfig = (
    label: string,
    theme: StudentLocationBadgeThemeEntry,
  ): StudentLocationBadgeConfig => ({
    label,
    colorToken: theme.colorToken,
    shadow: theme.shadow,
    textClass: theme.textClass,
  });

  if (!status) {
    return buildConfig(fallbackLabel, STUDENT_LOCATION_BADGE_TOKENS.home);
  }

  switch (status.state) {
    case "PRESENT_IN_ROOM": {
      // If no room data, treat as HOME (data inconsistency fallback)
      if (!status.room) {
        return buildConfig(fallbackLabel, STUDENT_LOCATION_BADGE_TOKENS.home);
      }

      const isGroupRoom = status.room.isGroupRoom;
      const label = status.room.name;
      const theme = isGroupRoom
        ? STUDENT_LOCATION_BADGE_TOKENS.groupRoom
        : STUDENT_LOCATION_BADGE_TOKENS.otherRoom;
      return buildConfig(label, theme);
    }
    case "TRANSIT":
      return buildConfig("Unterwegs", STUDENT_LOCATION_BADGE_TOKENS.transit);
    case "SCHOOLYARD":
      return buildConfig("Schulhof", STUDENT_LOCATION_BADGE_TOKENS.schoolyard);
    case "HOME":
      return buildConfig("Zuhause", STUDENT_LOCATION_BADGE_TOKENS.home);
    default:
      return buildConfig("Unbekannt", {
        colorToken: "#6B7280",
        shadow: "0 8px 25px rgba(107, 114, 128, 0.4)",
        textClass: "text-white backdrop-blur-sm",
        icon: "question",
      });
  }
}

/**
 * Checks if a location status indicates the student is currently on-site
 */
export function isStudentOnSite(status: StudentLocationStatus | null): boolean {
  if (!status) return false;
  return status.state !== "HOME";
}

/**
 * Checks if a location status indicates the student is in a specific room
 */
export function isStudentInRoom(status: StudentLocationStatus | null): boolean {
  if (!status) return false;
  return status.state === "PRESENT_IN_ROOM" && status.room !== undefined;
}

/**
 * Gets the current room name from location status, or null if not in a room
 */
export function getCurrentRoomName(status: StudentLocationStatus | null): string | null {
  if (status?.state !== "PRESENT_IN_ROOM" || !status.room) {
    return null;
  }
  return status.room.name;
}
