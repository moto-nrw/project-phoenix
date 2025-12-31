import * as React from "react";

import type {
  DisplayMode,
  LocationStyle,
  StudentLocationContext,
} from "@/lib/location-helper";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  canSeeDetailedLocation,
  getLocationColor,
  getLocationDisplay,
  getLocationGlowEffect,
  parseLocation,
} from "@/lib/location-helper";

/**
 * Formats the location_since timestamp for display.
 * Shows only the time (HH:MM) since it's for "current" location.
 */
function formatLocationSince(
  isoTimestamp: string | null | undefined,
): string | null {
  if (!isoTimestamp) return null;

  try {
    const date = new Date(isoTimestamp);
    if (Number.isNaN(date.getTime())) return null;

    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return null;
  }
}

export interface LocationBadgeProps {
  student: StudentLocationContext;
  displayMode: DisplayMode;
  userGroups?: string[];
  groupRooms?: string[]; // Räume der eigenen OGS-Gruppen (für grüne Farbe)
  supervisedRooms?: string[];
  isGroupRoom?: boolean;
  variant?: "simple" | "modern";
  size?: "sm" | "md" | "lg";
  /** Show "seit XX:XX Uhr" below the badge for Anwesend/Zuhause status. Default: false */
  showLocationSince?: boolean;
}

const MODERN_BASE_CLASS =
  "inline-flex items-center rounded-full font-bold text-white backdrop-blur-sm";
const SIMPLE_BASE_CLASS = "inline-flex items-center rounded-full font-semibold";

const SIZE_MAP = {
  sm: {
    modern: "px-2 py-0.5 text-[11px]",
    simple: "px-2 py-0.5 text-[11px]",
    dot: "mr-1.5 h-1 w-1",
  },
  md: {
    modern: "px-3 py-1.5 text-xs",
    simple: "px-2.5 py-0.5 text-xs",
    dot: "mr-2 h-1.5 w-1.5",
  },
  lg: {
    modern: "px-4 py-2 text-sm",
    simple: "px-3 py-1 text-sm",
    dot: "mr-2.5 h-2 w-2",
  },
} as const;

const DEFAULT_SIZE = "md";

/**
 * Renders a location badge using the centralized helper logic.
 */
export function LocationBadge({
  student,
  displayMode,
  userGroups,
  groupRooms,
  supervisedRooms,
  isGroupRoom,
  variant = "modern",
  size = DEFAULT_SIZE,
  showLocationSince = false,
}: LocationBadgeProps) {
  const parsed = parseLocation(student.current_location);
  const label = getLocationDisplay(
    student,
    displayMode,
    userGroups,
    supervisedRooms,
  );

  // Determine color based on display mode and permissions
  let color: string;
  if (displayMode === "groupName") {
    color = LOCATION_COLORS.GROUP_ROOM; // Green - showing group name
  } else if (displayMode === "contextAware") {
    // For contextAware mode, check if user has detailed access
    const hasDetailedAccess = canSeeDetailedLocation(
      student,
      userGroups,
      supervisedRooms,
    );
    if (hasDetailedAccess) {
      // Own students - user can see full room details
      // Green: OGS group room, Blue: other room, Orange: Schulhof, etc.
      color = getLocationColor(
        student.current_location,
        isGroupRoom,
        groupRooms,
      );
    } else {
      // Foreign students - user sees limited info (only status, no room)
      // Use the filtered label (e.g., "Anwesend") to determine color
      // This ensures: Anwesend=Green, Zuhause=Red (never Blue/Orange/Purple)
      color = getLocationColor(label, false, []);
    }
  } else {
    // roomName mode - use full location for color
    color = getLocationColor(student.current_location, isGroupRoom, groupRooms);
  }

  const glowEffect = getLocationGlowEffect(color);

  const locationStyle: LocationStyle = {
    color,
    glowEffect,
    label,
  };

  const sizeKey = size ?? DEFAULT_SIZE;
  const sizeConfig = SIZE_MAP[sizeKey] ?? SIZE_MAP[DEFAULT_SIZE];

  // Determine if we should show "seit XX:XX" for this status
  // Only when prop is enabled AND for Anwesend (in room) or Zuhause (checked out)
  const formattedTime = formatLocationSince(student.location_since);
  const showSinceTime =
    showLocationSince &&
    formattedTime &&
    (parsed.status === LOCATION_STATUSES.PRESENT ||
      parsed.status === LOCATION_STATUSES.HOME);

  if (variant === "simple") {
    return (
      <div className="flex flex-col items-center">
        <span
          className={`${SIMPLE_BASE_CLASS} ${sizeConfig.simple}`}
          style={{
            backgroundColor: locationStyle.color,
            color: "#fff",
          }}
          data-location-status={parsed.status}
        >
          {locationStyle.label}
        </span>
        {showSinceTime && (
          <span className="mt-0.5 text-[10px] text-gray-500">
            seit {formattedTime} Uhr
          </span>
        )}
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center">
      <span
        className={`${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
        style={{
          backgroundColor: locationStyle.color,
          boxShadow: locationStyle.glowEffect,
        }}
        data-location-status={parsed.status}
      >
        <span
          className={`${sizeConfig.dot} animate-pulse rounded-full bg-white/80`}
        />
        {locationStyle.label}
      </span>
      {showSinceTime && (
        <span className="mt-0.5 text-[10px] text-gray-500">
          seit {formattedTime} Uhr
        </span>
      )}
    </div>
  );
}
