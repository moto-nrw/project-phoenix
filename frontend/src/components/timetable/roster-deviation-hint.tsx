"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
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
  const actions =
    onConnect || onKeepManual ? (
      <span className="flex flex-wrap gap-2">
        {onConnect ? (
          <Button
            type="button"
            variant="surface"
            size="compact"
            onClick={onConnect}
            disabled={busy}
          >
            Mit Angebot verbinden
          </Button>
        ) : null}
        {onKeepManual ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onKeepManual}
            disabled={busy}
          >
            Bleibt von Hand gepflegt
          </Button>
        ) : null}
      </span>
    ) : undefined;
  return (
    <Alert
      type="warning"
      // A hint in a list, not an event: it must not interrupt screen readers.
      announce="off"
      message={describeRosterDeviation(deviation)}
      action={actions}
      actionLayout="stacked"
      className={className}
    />
  );
}
