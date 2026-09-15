"use client";

import { Hand, RefreshCw, SplitSquareHorizontal } from "lucide-react";

import { Tooltip } from "~/components/ui/tooltip";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { describeRosterMaintenance } from "~/lib/roster-maintenance";
import type {
  RosterMaintenanceMode,
  TemplateRosterMaintenance,
} from "~/lib/timetable-types";
import { cn } from "~/lib/utils";

const TONES: Record<RosterMaintenanceMode, { bg: string; text: string }> = {
  automatic: {
    bg: MOTO_COLOR_PALETTE.green.soft,
    text: MOTO_COLOR_PALETTE.green.strong,
  },
  partial: {
    bg: MOTO_COLOR_PALETTE.blue.soft,
    text: MOTO_COLOR_PALETTE.blue.strong,
  },
  manual: {
    bg: MOTO_COLOR_PALETTE.neutral.soft,
    text: MOTO_COLOR_PALETTE.neutral.strong,
  },
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
      <span
        className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap"
        style={{ backgroundColor: tone.bg, color: tone.text }}
      >
        <Icon className="h-3 w-3 shrink-0" aria-hidden="true" />
        <span className="sr-only">Teilnehmerpflege: </span>
        {description.label}
      </span>
    </Tooltip>
  );
}
