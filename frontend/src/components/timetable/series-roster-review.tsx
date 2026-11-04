"use client";

import { Button } from "~/components/ui/button";
import type { RosterDeviation } from "~/lib/roster-deviation";
import { cn } from "~/lib/utils";

import { RosterDeviationHint } from "./roster-deviation-hint";

/**
 * The roster decision of a Regeltermin in the occurrence details and on the
 * series card (#5103):
 * the deviation hint with both decisions, or, after „Liste so lassen“, one
 * quiet line with the way back. Shows nothing otherwise.
 */
export function SeriesRosterReview({
  deviation,
  manualRosterConfirmed,
  busy = false,
  className = "mt-2",
  onConnect,
  onSetManualRoster,
}: Readonly<{
  deviation?: RosterDeviation;
  manualRosterConfirmed: boolean;
  busy?: boolean;
  className?: string;
  onConnect?: (deviation: RosterDeviation) => void;
  onSetManualRoster?: (confirmed: boolean) => void;
}>) {
  if (deviation) {
    return (
      <RosterDeviationHint
        deviation={deviation}
        busy={busy}
        className={className}
        onConnect={onConnect ? () => onConnect(deviation) : undefined}
        onKeepManual={
          onSetManualRoster ? () => onSetManualRoster(true) : undefined
        }
      />
    );
  }
  if (!manualRosterConfirmed) return null;
  return (
    <p
      className={cn(
        "flex flex-wrap items-center gap-x-2 text-sm text-gray-600",
        className,
      )}
    >
      <span>Die Kinderliste bleibt bewusst so.</span>
      {onSetManualRoster ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          disabled={busy}
          onClick={() => onSetManualRoster(false)}
        >
          Hinweis wieder zeigen
        </Button>
      ) : null}
    </p>
  );
}
