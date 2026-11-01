"use client";

import { useId, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  rosterDeviationGroups,
  rosterDeviationMatch,
  type RosterDeviation,
  type RosterDeviationGroup,
} from "~/lib/roster-deviation";

/**
 * The warning a Regeltermin carries when its children list, kept by hand or
 * filled through a Klasse or Jahrgang, does not match the bookings of a care
 * offering (#5103). It replaces the grey „Manuell“ badge and names the
 * children that differ with their weekdays; several fold into a list.
 * Planners decide here: connect the series to the offering or keep the list
 * as it is, two equal ways with the same kit button. Readers without planning
 * rights only see it.
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
  const groups = rosterDeviationGroups(deviation);
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
            variant="surface"
            size="compact"
            onClick={onKeepManual}
            disabled={busy}
          >
            Liste so lassen
          </Button>
        ) : null}
      </span>
    ) : undefined;
  return (
    <Alert
      type="warning"
      // A hint in a list, not an event: it must not interrupt screen readers.
      announce="off"
      title="Weicht von Anmeldungen ab"
      message={rosterDeviationMatch(deviation)}
      details={
        groups.length > 0 ? (
          <div className="mt-1 flex flex-col gap-1">
            {groups.map((group) => (
              <DeviationGroup key={group.sentence} group={group} />
            ))}
          </div>
        ) : undefined
      }
      action={actions}
      actionLayout="stacked"
      className={className}
    />
  );
}

function DeviationGroup({ group }: Readonly<{ group: RosterDeviationGroup }>) {
  const [open, setOpen] = useState(false);
  const listId = useId();
  if (group.children.length === 0) return <p>{group.sentence}</p>;
  return (
    <div>
      <p className="flex flex-wrap items-center gap-x-2">
        <span>{group.sentence}</span>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setOpen((current) => !current)}
        >
          {open ? "Namen ausblenden" : "Namen zeigen"}
        </Button>
      </p>
      <ul id={listId} hidden={!open} className="mt-1 list-disc pl-5">
        {group.children.map((child) => (
          <li key={child.studentId}>{child.label}</li>
        ))}
      </ul>
    </div>
  );
}
