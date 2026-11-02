"use client";

import { useId, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  SectionCollapseHead,
  SectionCollapseToggle,
} from "~/components/ui/section-card";
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
 * children that differ with their weekdays; several fold into a list with
 * the kit disclosure.
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
  const [collapsed, setCollapsed] = useState(true);
  const listId = useId();
  if (group.children.length === 0) return <p>{group.sentence}</p>;
  return (
    <div>
      {/* The kit disclosure (#5021): the whole sentence row opens the list,
          the toggle stays the one button with aria-expanded. */}
      <SectionCollapseHead className="flex items-center gap-2">
        <span className="min-w-0 flex-1">{group.sentence}</span>
        <SectionCollapseToggle
          title="Namen"
          collapsed={collapsed}
          controls={collapsed ? undefined : listId}
          onToggle={() => setCollapsed((value) => !value)}
        />
      </SectionCollapseHead>
      {collapsed ? null : (
        <ul id={listId} className="mt-1 list-disc pl-5">
          {group.children.map((child) => (
            <li key={child.studentId}>{child.label}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
