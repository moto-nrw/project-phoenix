"use client";

import { Eye } from "lucide-react";

import { Tooltip } from "~/components/ui/tooltip";
import { cn } from "~/lib/utils";
import { PARENT_VISIBLE_DEFAULT_HINT } from "~/lib/parent-visible-fields";

/**
 * Marks a Stammdaten field or section whose content the parent portal mirrors,
 * so staff see before saving what guardians will read (#2163).
 *
 * Two shapes for two hosts: `compact` is the icon-only marker that fits behind
 * a dense form label, the default pill carries the words and sits in a section
 * heading. Both stay legible without color — the icon and the text do the work,
 * the tint is decoration.
 *
 * Which fields qualify is NOT decided here; it lives in
 * `~/lib/parent-visible-fields` next to the backend structs it mirrors.
 */
export function ParentVisibleBadge({
  hint = PARENT_VISIBLE_DEFAULT_HINT,
  compact = false,
  className,
  bubbleClassName,
}: Readonly<{
  /** Tooltip text — pass one of `PARENT_VISIBLE_HINTS`. */
  hint?: string;
  /** Icon-only variant for inline form labels. */
  compact?: boolean;
  className?: string;
  /** Reposition the tooltip bubble where a container would clip it. */
  bubbleClassName?: string;
}>) {
  return (
    <Tooltip
      content={hint}
      className={cn("align-middle", className)}
      bubbleClassName={bubbleClassName}
    >
      <span
        className={cn(
          "inline-flex items-center gap-1 rounded-full bg-[#5080D8]/10 font-medium text-[#355A9A]",
          compact ? "p-1" : "px-2 py-0.5 text-[11px]",
        )}
      >
        <Eye className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        {compact ? (
          <span className="sr-only">Für Eltern sichtbar</span>
        ) : (
          "Für Eltern sichtbar"
        )}
      </span>
    </Tooltip>
  );
}

/**
 * The one-line explanation that turns the markers above into a rule. Renders a
 * live marker so the icon-only form needs no separate key, and names the
 * child's address because that is the field schools ask about most.
 */
export function ParentVisibilityLegend({
  className,
}: Readonly<{ className?: string }>) {
  return (
    // Inline flow rather than flex, so a narrow container wraps the sentence
    // around the marker instead of stranding it on a line of its own.
    <p className={cn("text-xs leading-relaxed text-gray-500", className)}>
      <ParentVisibleBadge className="mr-1" />
      kennzeichnet Angaben, die Erziehungsberechtigte im Elternportal sehen.
      Alle übrigen Angaben, auch die Adresse des Kindes, bleiben intern.
    </p>
  );
}
