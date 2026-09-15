"use client";

import { Hand, RefreshCw, SplitSquareHorizontal } from "lucide-react";

import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { Tooltip } from "~/components/ui/tooltip";
import { describeRosterMaintenance } from "~/lib/roster-maintenance";
import type {
  RosterMaintenanceMode,
  TemplateRosterMaintenance,
} from "~/lib/timetable-types";
import { cn } from "~/lib/utils";

const TONES: Record<RosterMaintenanceMode, StatusBadgeTone> = {
  automatic: "green",
  partial: "blue",
  manual: "gray",
};

const ICONS: Record<RosterMaintenanceMode, typeof RefreshCw> = {
  automatic: RefreshCw,
  partial: SplitSquareHorizontal,
  manual: Hand,
};

/**
 * Says whether later children reach a Regeltermin by themselves (#3140).
 * A display, not a switch: the explanation opens on hover, keyboard focus
 * and tap through the kit Tooltip. Label and icon differ per state, so the
 * tint is never the only cue.
 */
export function RosterMaintenanceBadge({
  state,
  className,
  bubbleClassName,
}: Readonly<{
  state: TemplateRosterMaintenance;
  className?: string;
  /** Reposition the bubble where a container would clip it. */
  bubbleClassName?: string;
}>) {
  const description = describeRosterMaintenance(state);
  const tone = TONES[state.mode];
  const Icon = ICONS[state.mode];
  return (
    <Tooltip
      content={description.explanation}
      className={cn("align-middle", className)}
      bubbleClassName={cn("max-w-[16rem] font-normal", bubbleClassName)}
    >
      <StatusBadge
        label={description.label}
        tone={tone}
        icon={Icon}
        accessibleLabel="Teilnehmerpflege: "
        compact
      />
    </Tooltip>
  );
}
