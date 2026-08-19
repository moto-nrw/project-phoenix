import type {
  DisplayMode,
  StudentLocationContext,
} from "@/lib/location-helper";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  canSeeDetailedLocation,
  getLocationBadgeTone,
  getLocationColor,
  getLocationDisplay,
  isHomeLocation,
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

/**
 * Determines how to display sickness status on the badge.
 * - If sick AND at home: replace "Zuhause" with "Krank"
 * - If sick AND present: show additional "Krank" indicator
 */
function getSickDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.sick) return "none";

  // If at home and sick, replace the badge entirely
  if (isHomeLocation(student.current_location)) {
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

  if (isHomeLocation(student.current_location)) {
    return "replace";
  }

  return "additional";
}

function getClassTripDisplayMode(
  student: StudentLocationContext,
): "replace" | "additional" | "none" {
  if (!student.class_trip) return "none";

  if (isHomeLocation(student.current_location)) {
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

  if (isHomeLocation(student.current_location)) {
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
  /** Show "seit XX:XX Uhr" below the badge for Anwesend/Zuhause status. Default: false */
  readonly showLocationSince?: boolean;
}

const MODERN_BASE_CLASS = "inline-flex items-center rounded-full font-semibold";
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
  let label = getLocationDisplay(
    student,
    displayMode,
    userGroups,
    supervisedRooms,
  );

  // Check sick / class trip / excused / notArrival status display modes.
  // Priority: sick > class trip > excused > notArrival. Only one replace-mode applies at a time.
  const sickMode = getSickDisplayMode(student);
  const classTripMode =
    sickMode === "none" ? getClassTripDisplayMode(student) : "none";
  const excusedMode =
    sickMode === "none" && classTripMode === "none"
      ? getExcusedDisplayMode(student)
      : "none";
  const notArrivalMode =
    sickMode === "none" && classTripMode === "none" && excusedMode === "none"
      ? getNotArrivalDisplayMode(student)
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

  const locationTone = getLocationBadgeTone(color);
  const sickTone = getLocationBadgeTone(LOCATION_COLORS.SICK);
  const classTripTone = getLocationBadgeTone(LOCATION_COLORS.CLASS_TRIP);
  const excusedTone = getLocationBadgeTone(LOCATION_COLORS.EXCUSED);
  const notArrivalTone = getLocationBadgeTone(LOCATION_COLORS.NOT_ARRIVAL);

  const sizeKey = size ?? DEFAULT_SIZE;
  const sizeConfig = SIZE_MAP[sizeKey] ?? SIZE_MAP[DEFAULT_SIZE];

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
  const showSinceTime =
    showLocationSince &&
    formattedTime &&
    (parsed.status === LOCATION_STATUSES.PRESENT ||
      parsed.status === LOCATION_STATUSES.HOME ||
      sickMode === "replace" ||
      classTripMode === "replace" ||
      excusedMode === "replace");

  const sickIndicator = (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: sickTone.backgroundColor,
        color: sickTone.textColor,
      }}
      data-sick-indicator="true"
    >
      <span
        className={`${sizeConfig.dot} rounded-full`}
        style={{ backgroundColor: sickTone.dotColor }}
      />
      {LOCATION_STATUSES.SICK}
    </span>
  );

  const excusedIndicator = (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: excusedTone.backgroundColor,
        color: excusedTone.textColor,
      }}
      data-excused-indicator="true"
    >
      <span
        className={`${sizeConfig.dot} rounded-full`}
        style={{ backgroundColor: excusedTone.dotColor }}
      />
      {LOCATION_STATUSES.EXCUSED}
    </span>
  );

  const classTripIndicator = (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: classTripTone.backgroundColor,
        color: classTripTone.textColor,
      }}
      data-class-trip-indicator="true"
    >
      <span
        className={`${sizeConfig.dot} rounded-full`}
        style={{ backgroundColor: classTripTone.dotColor }}
      />
      {LOCATION_STATUSES.CLASS_TRIP}
    </span>
  );

  const notArrivalIndicator = (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: notArrivalTone.backgroundColor,
        color: notArrivalTone.textColor,
      }}
      data-not-arrival-indicator="true"
      title={student.not_arrival_reason ?? undefined}
    >
      <span
        className={`${sizeConfig.dot} rounded-full`}
        style={{ backgroundColor: notArrivalTone.dotColor }}
      />
      {LOCATION_STATUSES.NOT_ARRIVAL}
    </span>
  );

  if (variant === "simple") {
    return (
      <div className="flex flex-col items-center">
        <span
          className={`${SIMPLE_BASE_CLASS} ${sizeConfig.simple}`}
          style={{
            backgroundColor: locationTone.backgroundColor,
            color: locationTone.textColor,
          }}
          data-location-status={parsed.status}
          title={
            notArrivalMode === "replace"
              ? (student.not_arrival_reason ?? undefined)
              : undefined
          }
        >
          {label}
        </span>
        {showSinceTime && (
          <span className="mt-0.5 text-[10px] text-gray-500">
            seit {formattedTime} Uhr
          </span>
        )}
        {sickMode === "additional" && sickIndicator}
        {classTripMode === "additional" && classTripIndicator}
        {excusedMode === "additional" && excusedIndicator}
        {notArrivalMode === "additional" && notArrivalIndicator}
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center">
      <span
        className={`${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
        style={{
          backgroundColor: locationTone.backgroundColor,
          color: locationTone.textColor,
        }}
        data-location-status={parsed.status}
        title={
          notArrivalMode === "replace"
            ? (student.not_arrival_reason ?? undefined)
            : undefined
        }
      >
        <span
          className={`${sizeConfig.dot} rounded-full`}
          style={{ backgroundColor: locationTone.dotColor }}
        />
        {label}
      </span>
      {showSinceTime && (
        <span className="mt-0.5 text-[10px] text-gray-500">
          seit {formattedTime} Uhr
        </span>
      )}
      {sickMode === "additional" && sickIndicator}
      {classTripMode === "additional" && classTripIndicator}
      {excusedMode === "additional" && excusedIndicator}
      {notArrivalMode === "additional" && notArrivalIndicator}
    </div>
  );
}
