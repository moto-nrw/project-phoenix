import * as React from "react";

import type {
  DisplayMode,
  LocationStyle,
  StudentLocationContext,
} from "@/lib/location-helper";
import {
  LOCATION_COLORS,
  getLocationColor,
  getLocationDisplay,
  getLocationGlowEffect,
  parseLocation,
} from "@/lib/location-helper";

export interface LocationBadgeProps {
  student: StudentLocationContext;
  displayMode: DisplayMode;
  userGroups?: string[];
  groupRooms?: string[]; // Räume der eigenen OGS-Gruppen (für grüne Farbe)
  supervisedRooms?: string[];
  isGroupRoom?: boolean;
  variant?: "simple" | "modern";
  size?: "sm" | "md" | "lg";
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
}: LocationBadgeProps) {
  const parsed = parseLocation(student.current_location);
  const label = getLocationDisplay(
    student,
    displayMode,
    userGroups,
    supervisedRooms,
  );
  const color =
    displayMode === "groupName"
      ? LOCATION_COLORS.GROUP_ROOM // Green - showing group name for present students
      : getLocationColor(student.current_location, isGroupRoom, groupRooms);
  const glowEffect = getLocationGlowEffect(color);

  const locationStyle: LocationStyle = {
    color,
    glowEffect,
    label,
  };

  const sizeKey = size ?? DEFAULT_SIZE;
  const sizeConfig = SIZE_MAP[sizeKey] ?? SIZE_MAP[DEFAULT_SIZE];

  if (variant === "simple") {
    return (
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
    );
  }

  return (
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
  );
}

export default LocationBadge;
