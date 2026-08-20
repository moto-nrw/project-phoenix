import * as React from "react";

import type { StudentLocationContext } from "@/lib/location-helper";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  getLocationBadgeTone,
  isHomeLocation,
  isSchoolyardLocation,
  parseLocation,
} from "@/lib/location-helper";

/**
 * PresenceBadge — simplified binary-mode counterpart to LocationBadge.
 *
 * Renders only three visual states: Anwesend (green), Schulhof (orange),
 * Abwesend (gray — LOCATION_COLORS.HOME is the neutral #6B7280 since the
 * palette move; red now means SICK/DANGER). No room details, no "Unterwegs",
 * no display-mode switching.
 * Sick / excused overlays mirror LocationBadge so those students still look
 * right in binary-mode tenants.
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
function derivePresenceState(
  location?: string | null,
): "anwesend" | "schulhof" | "abwesend" {
  if (!location || isHomeLocation(location)) {
    return "abwesend";
  }
  if (isSchoolyardLocation(location)) {
    return "schulhof";
  }
  return "anwesend";
}

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

interface PresenceBadgeProps {
  readonly student: StudentLocationContext;
  readonly variant?: "simple" | "modern";
  readonly size?: "sm" | "md" | "lg";
  /** Show "seit XX:XX Uhr" under the badge. Default: false. */
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

function renderOverlayBadge({
  overlayLabel,
  overlayColor,
  dataAttr,
  sizeConfig,
}: {
  overlayLabel: string;
  overlayColor: string;
  dataAttr: string;
  sizeConfig: (typeof SIZE_MAP)[keyof typeof SIZE_MAP];
}) {
  const tone = getLocationBadgeTone(overlayColor);

  return (
    <span
      className={`mt-1 ${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
      style={{
        backgroundColor: tone.backgroundColor,
        color: tone.textColor,
      }}
      {...{ [dataAttr]: "true" }}
    >
      <span
        className={`${sizeConfig.dot} rounded-full`}
        style={{ backgroundColor: tone.dotColor }}
      />
      {overlayLabel}
    </span>
  );
}

// Sick/excused status takes visual precedence over absence (a sick student
// at home reads as "Krank", not "Abwesend") — matches LocationBadge behaviour.
function resolveBadgeStyle(
  state: "anwesend" | "schulhof" | "abwesend",
  student: StudentLocationContext,
): { label: string; color: string } {
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

  switch (state) {
    case "anwesend":
      return {
        label: LOCATION_STATUSES.PRESENT,
        color: LOCATION_COLORS.GROUP_ROOM,
      };
    case "schulhof":
      return {
        label: LOCATION_STATUSES.SCHOOLYARD,
        color: LOCATION_COLORS.SCHOOLYARD,
      };
    default:
      return { label: LOCATION_STATUSES.HOME, color: LOCATION_COLORS.HOME };
  }
}

export function PresenceBadge({
  student,
  variant = "modern",
  size = DEFAULT_SIZE,
  showLocationSince = false,
}: PresenceBadgeProps) {
  const state = derivePresenceState(student.current_location);
  const { label, color } = resolveBadgeStyle(state, student);

  const sizeKey = size ?? DEFAULT_SIZE;
  const sizeConfig = SIZE_MAP[sizeKey] ?? SIZE_MAP[DEFAULT_SIZE];
  const tone = getLocationBadgeTone(color);

  // A contradiction between actual presence and the plan is more actionable
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
  const showSinceTime = showLocationSince && formattedTime !== null;

  const dataStatus = parseLocation(student.current_location).status;

  const pill =
    variant === "simple" ? (
      <span
        className={`${SIMPLE_BASE_CLASS} ${sizeConfig.simple}`}
        style={{
          backgroundColor: tone.backgroundColor,
          color: tone.textColor,
        }}
        data-location-status={dataStatus}
        data-presence-state={state}
      >
        {label}
      </span>
    ) : (
      <span
        className={`${MODERN_BASE_CLASS} ${sizeConfig.modern}`}
        style={{
          backgroundColor: tone.backgroundColor,
          color: tone.textColor,
        }}
        data-location-status={dataStatus}
        data-presence-state={state}
      >
        <span
          className={`${sizeConfig.dot} rounded-full`}
          style={{ backgroundColor: tone.dotColor }}
        />
        {label}
      </span>
    );

  return (
    <div className="flex flex-col items-center">
      {pill}
      {showSinceTime && (
        <span className="mt-0.5 text-[10px] text-gray-500">
          seit {formattedTime} Uhr
        </span>
      )}
      {showUnplannedOverlay &&
        renderOverlayBadge({
          overlayLabel: LOCATION_STATUSES.UNPLANNED_PRESENT,
          overlayColor: LOCATION_COLORS.NOT_ARRIVAL,
          dataAttr: "data-not-arrival-indicator",
          sizeConfig,
        })}
      {showSickOverlay &&
        renderOverlayBadge({
          overlayLabel: LOCATION_STATUSES.SICK,
          overlayColor: LOCATION_COLORS.SICK,
          dataAttr: "data-sick-indicator",
          sizeConfig,
        })}
      {showClassTripOverlay &&
        renderOverlayBadge({
          overlayLabel: LOCATION_STATUSES.CLASS_TRIP,
          overlayColor: LOCATION_COLORS.CLASS_TRIP,
          dataAttr: "data-class-trip-indicator",
          sizeConfig,
        })}
      {showExcusedOverlay &&
        renderOverlayBadge({
          overlayLabel: LOCATION_STATUSES.EXCUSED,
          overlayColor: LOCATION_COLORS.EXCUSED,
          dataAttr: "data-excused-indicator",
          sizeConfig,
        })}
    </div>
  );
}
