"use client";

import { DecisionHint } from "~/components/timetable/decision-hint";
import {
  describeRosterDeviation,
  type RosterDeviation,
} from "~/lib/roster-deviation";

/**
 * The warning a Regeltermin carries when its hand-kept children list does
 * not match the bookings of a care offering (#5103). It replaces the grey
 * „Manuell“ badge. Planners decide here: connect the series to the offering
 * or keep the list by hand. Readers without planning rights only see it.
 */
export function RosterDeviationHint({
  deviation,
  onConnect,
  onKeepManual,
  busy = false,
  className,
}: Readonly<{
  deviation: RosterDeviation;
  onConnect?: () => void;
  onKeepManual?: () => void;
  /** A decision is being saved; both buttons wait. */
  busy?: boolean;
  className?: string;
}>) {
  return (
    <DecisionHint
      message={describeRosterDeviation(deviation)}
      primary={
        onConnect
          ? { label: "Mit Angebot verbinden", onClick: onConnect }
          : undefined
      }
      secondary={
        onKeepManual
          ? { label: "Bleibt von Hand gepflegt", onClick: onKeepManual }
          : undefined
      }
      busy={busy}
      className={className}
    />
  );
}
