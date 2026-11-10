import type {
  DisplayMode,
  StudentLocationContext,
} from "@/lib/location-helper";
import { formatTime } from "@/lib/date-helpers";
import { PresenceHint } from "./presence-hint";
import { StatusColorBadge } from "./status-color-badge";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  canSeeDetailedLocation,
  getLocationColor,
  getLocationDisplay,
  isNotCheckedInLocation,
  parseLocation,
} from "@/lib/location-helper";

/**
 * The clock time for „seit …“: Berlin wall clock, whatever the browser's zone.
 * Null when the time is missing or unreadable, so no line shows.
 */
export function formatLocationSince(
  isoTimestamp: string | null | undefined,
): string | null {
  if (!isoTimestamp || Number.isNaN(Date.parse(isoTimestamp))) return null;
  return formatTime(isoTimestamp);
}

/**
 * Determines how to display sickness status on the badge.
 * - If sick AND at home: replace "Zuhause" with "Krank"
 * - If sick AND present: show an additional "Krank" hint line
 */
function getSickDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.sick) return "none";

  // If at home and sick, replace the badge entirely
  if (isNotCheckedInLocation(student.current_location)) {
    return "replace";
  }

  // If present somewhere but sick, show additional indicator
  return "additional";
}

/**
 * Determines how to display excused status on the badge.
 * Mirrors the sick pattern:
 * - If excused AND at home: replace "Zuhause" with "Entschuldigt"
 * - If excused AND present: show additional "Entschuldigt" indicator
 */
function getExcusedDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.excused) return "none";

  if (isNotCheckedInLocation(student.current_location)) {
    return "replace";
  }

  return "additional";
}

function getClassTripDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.class_trip) return "none";

  if (isNotCheckedInLocation(student.current_location)) {
    return "replace";
  }

  return "additional";
}

/**
 * Determines how to display "kommt heute nicht" (arrival-schedule exception with null time).
 * Same replace/additional pattern as sick/excused.
 */
function getNotArrivalDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.not_arrival_today) return "none";

  if (isNotCheckedInLocation(student.current_location)) {
    return "replace";
  }

  return "additional";
}

interface LocationBadgeProps {
  readonly student: StudentLocationContext;
  readonly displayMode: DisplayMode;
  readonly userGroups?: string[];
  readonly groupRooms?: string[]; // Räume der eigenen OGS-Gruppen (für grüne Farbe)
  readonly supervisedRooms?: string[];
  readonly isGroupRoom?: boolean;
  readonly variant?: "simple" | "modern";
  readonly size?: "sm" | "md" | "lg";
  /** Show "seit XX:XX Uhr" below the badge when the backend sends a time. Default: false */
  readonly showLocationSince?: boolean;
}

type LocationPillSize = NonNullable<LocationBadgeProps["size"]>;
type LocationPillVariant = NonNullable<LocationBadgeProps["variant"]>;

/** The kit pill shape for the badge's own size and variant props. */
export function locationPillShape(
  size: LocationPillSize,
  variant: LocationPillVariant,
): { size?: "xs"; compact?: boolean } {
  if (size === "sm") return { size: "xs" };
  return size === "md" && variant === "simple" ? { compact: true } : {};
}

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
  let label = getLocationDisplay(
    student,
    displayMode,
    userGroups,
    supervisedRooms,
  );

  // An actual presence that contradicts the plan is the actionable state.
  // At home, the specific absence status keeps its existing precedence.
  const notArrivalMode = getNotArrivalDisplayMode(student);
  const sickMode =
    notArrivalMode === "additional" ? "none" : getSickDisplayMode(student);
  const classTripMode =
    notArrivalMode !== "additional" && sickMode === "none"
      ? getClassTripDisplayMode(student)
      : "none";
  const excusedMode =
    notArrivalMode !== "additional" &&
    sickMode === "none" &&
    classTripMode === "none"
      ? getExcusedDisplayMode(student)
      : "none";

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
      // Green: OGS group room, per-room hex when set, Blue otherwise; Orange: Schulhof, etc.
      color = getLocationColor(
        student.current_location,
        isGroupRoom,
        groupRooms,
        student.current_room_color,
      );
    } else {
      // Foreign students - user sees limited info (only status, no room)
      // Use the filtered label (e.g., "Anwesend") to determine color
      // This ensures: Anwesend=Green, Zuhause=Gray (never Blue/Purple)
      // The label can still be the plain "Schulhof" status — that is a coarse
      // status, not room detail, so it keeps following the school's configured
      // yard color (#2405) and falls back to orange when none is set.
      color = getLocationColor(label, false, [], student.current_room_color);
    }
  } else {
    // roomName mode - use full location for color
    color = getLocationColor(
      student.current_location,
      isGroupRoom,
      groupRooms,
      student.current_room_color,
    );
  }

  // Override for sick/classTrip/excused/notArrival students at home: replace the base label
  if (sickMode === "replace") {
    color = LOCATION_COLORS.SICK;
    label = LOCATION_STATUSES.SICK;
  } else if (classTripMode === "replace") {
    color = LOCATION_COLORS.CLASS_TRIP;
    label = LOCATION_STATUSES.CLASS_TRIP;
  } else if (excusedMode === "replace") {
    color = LOCATION_COLORS.EXCUSED;
    label = LOCATION_STATUSES.EXCUSED;
  } else if (notArrivalMode === "replace") {
    color = LOCATION_COLORS.NOT_ARRIVAL;
    label = LOCATION_STATUSES.NOT_ARRIVAL;
  }

  const sizeKey = size ?? DEFAULT_SIZE;

  // Determine if we should show "seit XX:XX" for this status.
  // For sick/excused at home, prefer the dedicated *_since timestamp but fall
  // back to location_since if missing. notArrival has no dedicated timestamp.
  const replaceTimeSource =
    sickMode === "replace"
      ? (student.sick_since ?? student.location_since)
      : classTripMode === "replace"
        ? (student.class_trip_since ?? student.location_since)
        : excusedMode === "replace"
          ? (student.excused_since ?? student.location_since)
          : notArrivalMode === "replace"
            ? student.location_since
            : null;
  const timeSource = replaceTimeSource ?? student.location_since;
  const formattedTime = formatLocationSince(timeSource);
  // The backend sends a time only where it means something for the state.
  const showSinceTime = showLocationSince && formattedTime !== null;

  // At most one pill per child (#4371): the location stays the pill, a
  // further fact becomes a neutral line below it.
  const hint =
    sickMode === "additional" ? (
      <PresenceHint kind="sick" size={sizeKey} />
    ) : classTripMode === "additional" ? (
      <PresenceHint kind="classTrip" size={sizeKey} />
    ) : excusedMode === "additional" ? (
      <PresenceHint kind="excused" size={sizeKey} />
    ) : notArrivalMode === "additional" ? (
      <PresenceHint
        kind="notArrival"
        size={sizeKey}
        title={student.not_arrival_reason ?? undefined}
      />
    ) : null;

  return (
    <div className="flex max-w-full min-w-0 flex-col items-end">
      <StatusColorBadge
        {...locationPillShape(sizeKey, variant)}
        wrap
        label={label}
        color={color}
        data-location-status={parsed.status}
        title={
          notArrivalMode === "replace"
            ? (student.not_arrival_reason ?? undefined)
            : undefined
        }
      />
      {showSinceTime && (
        <span className="mt-0.5 text-xs text-gray-500">
          seit {formattedTime} Uhr
        </span>
      )}
      {hint}
    </div>
  );
}
