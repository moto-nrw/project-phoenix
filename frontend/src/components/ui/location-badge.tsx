import type {
  DisplayMode,
  StudentLocationContext,
} from "@/lib/location-helper";
import { PresenceHint } from "./presence-hint";
import { StatusColorBadge } from "./status-color-badge";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  canSeeDetailedLocation,
  dayStatusPresentation,
  getLocationColor,
  getLocationDisplay,
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

type DayStatusDisplayMode = "replace" | "additional" | "none";

/**
 * How a day status (sick, class trip, excused, not arriving) shows on the
 * badge. The rule lives in {@link dayStatusPresentation}: without a check-in
 * the status replaces the location, a checked-in child keeps its location with
 * the status as a hint line, and a child staff checked out to class reads
 * "Im Unterricht" only (#5710).
 */
function dayStatusDisplayMode(
  active: boolean | null | undefined,
  student: StudentLocationContext,
): DayStatusDisplayMode {
  if (!active) return "none";
  switch (dayStatusPresentation(student.current_location)) {
    case "replace":
      return "replace";
    case "hint":
      return "additional";
    default:
      return "none";
  }
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
  const notArrivalMode = dayStatusDisplayMode(
    student.not_arrival_today,
    student,
  );
  const sickMode =
    notArrivalMode === "additional"
      ? "none"
      : dayStatusDisplayMode(student.sick, student);
  const classTripMode =
    notArrivalMode !== "additional" && sickMode === "none"
      ? dayStatusDisplayMode(student.class_trip, student)
      : "none";
  const excusedMode =
    notArrivalMode !== "additional" &&
    sickMode === "none" &&
    classTripMode === "none"
      ? dayStatusDisplayMode(student.excused, student)
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
  const showSinceTime =
    showLocationSince &&
    formattedTime &&
    (parsed.status === LOCATION_STATUSES.PRESENT ||
      parsed.status === LOCATION_STATUSES.HOME ||
      parsed.status === LOCATION_STATUSES.IN_CLASS ||
      sickMode === "replace" ||
      classTripMode === "replace" ||
      excusedMode === "replace");

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
