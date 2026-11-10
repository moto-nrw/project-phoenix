import * as React from "react";

import type { StudentLocationContext } from "@/lib/location-helper";
import {
  formatLocationSince,
  locationPillShape,
  showsLocationSince,
} from "./location-badge";
import { PresenceHint } from "./presence-hint";
import { StatusColorBadge } from "./status-color-badge";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  isAtSchoolLocation,
  isInClassLocation,
  isNotCheckedInLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
} from "@/lib/location-helper";

/**
 * PresenceBadge — simplified binary-mode counterpart to LocationBadge.
 *
 * Renders only three visual states: Anwesend (green), Schulhof (orange),
 * Abwesend (gray — LOCATION_COLORS.HOME is the neutral #6B7280 since the
 * palette move; red now means SICK/DANGER), which reads "Schule" for an
 * expected child before the first check-in (#3260). No room details, no "Unterwegs",
 * no display-mode switching.
 * Sick / excused hints mirror LocationBadge so those students still look
 * right in binary-mode tenants: at most one pill, further facts as a
 * {@link PresenceHint} line below it (#4371).
 *
 * Consumers should usually import {@link StudentPresenceBadge} instead —
 * that wrapper routes detailed-mode tenants to LocationBadge automatically
 * via `usePresenceMode()`. Use PresenceBadge directly only when you want
 * to force the binary visual regardless of tenant mode (e.g. in a
 * mode-agnostic compact list view).
 */

/**
 * Compute the binary presence state from the same `current_location` string
 * that LocationBadge consumes. Binary-mode backends emit these exact labels;
 * for safety we also collapse detailed-mode outputs (Unterwegs, room names)
 * into "anwesend" when this component is reached via the wrapper in
 * unexpected circumstances.
 */
function derivePresenceState(location?: string | null): PresenceState {
  if (isNotCheckedInLocation(location)) {
    return "abwesend";
  }
  if (isSchoolyardLocation(location)) {
    return "schulhof";
  }
  return "anwesend";
}

interface PresenceBadgeProps {
  readonly student: StudentLocationContext;
  readonly variant?: "simple" | "modern";
  readonly size?: "sm" | "md" | "lg";
  /** Show "seit XX:XX Uhr" under the badge. Default: false. */
  readonly showLocationSince?: boolean;
}

const DEFAULT_SIZE = "md";

type PresenceState = "anwesend" | "schulhof" | "abwesend";
type BadgeStyle = { label: string; color: string };

// Sick/excused status takes visual precedence over absence (a sick student
// at home reads as "Krank", not "Abwesend") — matches LocationBadge behaviour.
// Null when no day status replaces the location.
function resolveDayStatusStyle(
  state: PresenceState,
  student: StudentLocationContext,
): BadgeStyle | null {
  const atHome = state === "abwesend";
  if (student.sick && atHome) {
    return { label: LOCATION_STATUSES.SICK, color: LOCATION_COLORS.SICK };
  }
  if (student.class_trip && atHome) {
    return {
      label: LOCATION_STATUSES.CLASS_TRIP,
      color: LOCATION_COLORS.CLASS_TRIP,
    };
  }
  if (student.excused && atHome) {
    return { label: LOCATION_STATUSES.EXCUSED, color: LOCATION_COLORS.EXCUSED };
  }
  if (student.not_arrival_today && atHome) {
    return {
      label: LOCATION_STATUSES.NOT_ARRIVAL,
      color: LOCATION_COLORS.NOT_ARRIVAL,
    };
  }
  return null;
}

/** The location the pill shows when no day status replaces it. */
function resolveLocationStyle(
  state: PresenceState,
  student: StudentLocationContext,
): BadgeStyle {
  switch (state) {
    case "anwesend":
      return {
        label: LOCATION_STATUSES.PRESENT,
        color: LOCATION_COLORS.GROUP_ROOM,
      };
    case "schulhof":
      return {
        label: LOCATION_STATUSES.SCHOOLYARD,
        // Schools colour-code rooms and tablets, so the yard follows the
        // Schulhof room's own colour when one is set (#2405). Binary mode has
        // no room visit behind the state, so the backend resolves the room
        // once per request and ships it in `current_room_color`. Unset (or
        // blank) falls back to the orange default.
        color: student.current_room_color?.trim()
          ? student.current_room_color
          : LOCATION_COLORS.SCHOOLYARD,
      };
    default:
      // Before the first check-in an expected child is still in class (#3260).
      if (isAtSchoolLocation(student.current_location)) {
        return {
          label: LOCATION_STATUSES.AT_SCHOOL,
          color: LOCATION_COLORS.AT_SCHOOL,
        };
      }
      if (isInClassLocation(student.current_location)) {
        return {
          label: LOCATION_STATUSES.IN_CLASS,
          color: LOCATION_COLORS.IN_CLASS,
        };
      }
      return { label: LOCATION_STATUSES.HOME, color: LOCATION_COLORS.HOME };
  }
}

export function PresenceBadge({
  student,
  variant = "modern",
  size = DEFAULT_SIZE,
  showLocationSince = false,
}: PresenceBadgeProps) {
  const parsedLocation = parseLocation(student.current_location);
  const state = derivePresenceState(student.current_location);
  const location = resolveLocationStyle(state, student);
  const dayStatus = resolveDayStatusStyle(state, student);
  const { label, color } = dayStatus ?? location;

  const sizeKey = size ?? DEFAULT_SIZE;

  // One hint at most. A contradiction between actual presence and the plan is more actionable
  // than the underlying absence reason (krank, entschuldigt, etc.).
  const showUnplannedOverlay =
    student.not_arrival_today && state !== "abwesend";
  const showSickOverlay =
    student.sick && state !== "abwesend" && !showUnplannedOverlay;
  const showClassTripOverlay =
    student.class_trip &&
    state !== "abwesend" &&
    !showUnplannedOverlay &&
    !showSickOverlay;
  const showExcusedOverlay =
    student.excused &&
    state !== "abwesend" &&
    !showUnplannedOverlay &&
    !showSickOverlay &&
    !showClassTripOverlay;

  // "seit XX:XX" label: prefer sick/excused timestamp when replace mode
  // triggered, else fall back to generic location_since. Mirrors LocationBadge
  // semantics for cross-mode visual consistency.
  const replaceTimeSource =
    student.sick && state === "abwesend"
      ? (student.sick_since ?? student.location_since)
      : student.class_trip && state === "abwesend"
        ? (student.class_trip_since ?? student.location_since)
        : student.excused && state === "abwesend"
          ? (student.excused_since ?? student.location_since)
          : null;
  const timeSource = replaceTimeSource ?? student.location_since;
  const formattedTime = formatLocationSince(timeSource);
  const locationStatusForSince =
    // A room name has no status prefix but is displayed as „Anwesend“ in
    // binary mode. Unknown and missing locations must not inherit that time.
    parsedLocation.status === LOCATION_STATUSES.UNKNOWN
      ? parsedLocation.status
      : isTransitLocation(student.current_location)
        ? LOCATION_STATUSES.TRANSIT
        : location.label;
  const showSinceTime =
    showLocationSince &&
    formattedTime !== null &&
    // „Kommt heute nicht“ keeps the established time at home, but not after
    // the child was checked out to class.
    (label !== LOCATION_STATUSES.NOT_ARRIVAL ||
      parsedLocation.status === LOCATION_STATUSES.HOME) &&
    showsLocationSince(locationStatusForSince, replaceTimeSource !== null);

  const dataStatus = parsedLocation.status;

  return (
    <div className="flex max-w-full min-w-0 flex-col items-end">
      <StatusColorBadge
        {...locationPillShape(sizeKey, variant)}
        wrap
        label={label}
        color={color}
        data-location-status={dataStatus}
        data-presence-state={state}
        title={
          label === LOCATION_STATUSES.NOT_ARRIVAL
            ? (student.not_arrival_reason ?? undefined)
            : undefined
        }
      />
      {showSinceTime && (
        <span className="mt-0.5 text-xs text-gray-500">
          seit {formattedTime} Uhr
        </span>
      )}
      {showUnplannedOverlay && (
        <PresenceHint
          kind="notArrival"
          size={sizeKey}
          title={student.not_arrival_reason ?? undefined}
        />
      )}
      {showSickOverlay && <PresenceHint kind="sick" size={sizeKey} />}
      {showClassTripOverlay && <PresenceHint kind="classTrip" size={sizeKey} />}
      {showExcusedOverlay && <PresenceHint kind="excused" size={sizeKey} />}
    </div>
  );
}
