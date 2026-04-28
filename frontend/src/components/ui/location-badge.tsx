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
  let label = getLocationDisplay(
    student,
    displayMode,
    userGroups,
    supervisedRooms,
  );

  // Check sick / excused / notArrival status display modes.
  // Priority: sick > excused > notArrival. Only one replace-mode applies at a time.
  const sickMode = getSickDisplayMode(student);
  const excusedMode =
    sickMode === "none" ? getExcusedDisplayMode(student) : "none";
  const notArrivalMode =
    sickMode === "none" && excusedMode === "none"
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
      // Green: OGS group room, individual color: other rooms with custom hex,
      // Blue: other rooms without custom color, Orange: Schulhof, etc.
      color = getLocationColor(
        student.current_location,
        isGroupRoom,
        groupRooms,
        student.badge_color,
      );
    } else {
      // Foreign students - user sees limited info (only status, no room)
      // Use the filtered label (e.g., "Anwesend") to determine color
      // This ensures: Anwesend=Green, Zuhause=Red (never Blue/Orange/Purple)
      color = getLocationColor(label, false, []);
    }
  } else {
    // roomName mode - use full location for color
    color = getLocationColor(
      student.current_location,
      isGroupRoom,
      groupRooms,
      student.badge_color,
    );
  }

  // Override for sick/excused/notArrival students at home: replace the base label
  if (sickMode === "replace") {
    color = LOCATION_COLORS.SICK;
    label = LOCATION_STATUSES.SICK;
  } else if (excusedMode === "replace") {
    color = LOCATION_COLORS.EXCUSED;
    label = LOCATION_STATUSES.EXCUSED;
  } else if (notArrivalMode === "replace") {
    color = LOCATION_COLORS.NOT_ARRIVAL;
    label = LOCATION_STATUSES.NOT_ARRIVAL;
  }

  const glowEffect = getLocationGlowEffect(color);
  const sickGlowEffect = getLocationGlowEffect(LOCATION_COLORS.SICK);
  const excusedGlowEffect = getLocationGlowEffect(LOCATION_COLORS.EXCUSED);
  const notArrivalGlowEffect = getLocationGlowEffect(
    LOCATION_COLORS.NOT_ARRIVAL,
  );

  const locationStyle: LocationStyle = {
    color,
    glowEffect,
    label,
  };

  const sizeKey = size ?? DEFAULT_SIZE;
  const sizeConfig = SIZE_MAP[sizeKey] ?? SIZE_MAP[DEFAULT_SIZE];

  // Determine if we should show "seit XX:XX" for this status.
  // For sick/excused at home, prefer the dedicated *_since timestamp but fall
  // back to location_since if missing. notArrival has no dedicated timestamp.
  const replaceTimeSource =
    sickMode === "replace"
      ? (student.sick_since ?? student.location_since)
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
      excusedMode === "replace");

  // Sick indicator badge for "additional" mode (sick but present)
  // Uses same size configuration as the main badge for consistency
  const SickIndicator = () => (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: LOCATION_COLORS.SICK,
        boxShadow: sickGlowEffect,
      }}
      data-sick-indicator="true"
    >
      <span
        className={`${sizeConfig.dot} animate-pulse rounded-full bg-white/80`}
      />
      {LOCATION_STATUSES.SICK}
    </span>
  );

  // Excused indicator badge for "additional" mode (excused but present).
  // Same size/shape as sick indicator, purple to differentiate.
  const ExcusedIndicator = () => (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: LOCATION_COLORS.EXCUSED,
        boxShadow: excusedGlowEffect,
      }}
      data-excused-indicator="true"
    >
      <span
        className={`${sizeConfig.dot} animate-pulse rounded-full bg-white/80`}
      />
      {LOCATION_STATUSES.EXCUSED}
    </span>
  );

  // "Kommt heute nicht" indicator for "additional" mode (planned absence but present).
  const NotArrivalIndicator = () => (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: LOCATION_COLORS.NOT_ARRIVAL,
        boxShadow: notArrivalGlowEffect,
      }}
      data-not-arrival-indicator="true"
      title={student.not_arrival_reason ?? undefined}
    >
      <span
        className={`${sizeConfig.dot} animate-pulse rounded-full bg-white/80`}
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
            backgroundColor: locationStyle.color,
            color: "#fff",
          }}
          data-location-status={parsed.status}
          title={
            notArrivalMode === "replace"
              ? (student.not_arrival_reason ?? undefined)
              : undefined
          }
        >
          {locationStyle.label}
        </span>
        {showSinceTime && (
          <span className="mt-0.5 text-[10px] text-gray-500">
            seit {formattedTime} Uhr
          </span>
        )}
        {sickMode === "additional" && <SickIndicator />}
        {excusedMode === "additional" && <ExcusedIndicator />}
        {notArrivalMode === "additional" && <NotArrivalIndicator />}
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
        title={
          notArrivalMode === "replace"
            ? (student.not_arrival_reason ?? undefined)
            : undefined
        }
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
      {sickMode === "additional" && <SickIndicator />}
      {excusedMode === "additional" && <ExcusedIndicator />}
      {notArrivalMode === "additional" && <NotArrivalIndicator />}
    </div>
  );
}
