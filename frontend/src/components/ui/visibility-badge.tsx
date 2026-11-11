"use client";

import { Eye } from "lucide-react";

import { StatusBadge } from "~/components/ui/status-badge";
import { Tooltip } from "~/components/ui/tooltip";
import { TINT_CLASSES } from "~/components/ui/kit-palette";
import { cn } from "~/lib/utils";
import { PARENT_VISIBLE_DEFAULT_HINT } from "~/lib/parent-visible-fields";

const AUDIENCES = {
  parents: {
    label: "Für Eltern sichtbar",
    tone: "blue",
    compactClass: `${TINT_CLASSES.blue.surface} ${TINT_CLASSES.blue.text}`,
  },
  ogs: {
    label: "Für die OGS sichtbar",
    tone: "gray",
    compactClass: "bg-gray-100 text-gray-600",
  },
} as const;

/**
 * Sagt, wer eine Angabe außerdem liest. `audience="parents"` markiert im
 * Tenant-Portal, was das Elternportal spiegelt (#2163); `audience="ogs"` sagt
 * Eltern, was ihre OGS sieht.
 *
 * Two shapes for two hosts: `compact` is the icon-only marker that fits behind
 * a dense form label, the default pill carries the words and sits in a section
 * heading. Both stay legible without color — the icon and the text do the work,
 * the tint is decoration.
 *
 * Which fields qualify is NOT decided here; it lives in
 * `~/lib/parent-visible-fields` next to the backend structs it mirrors.
 */
export function VisibilityBadge({
  audience = "parents",
  label,
  hint = PARENT_VISIBLE_DEFAULT_HINT,
  compact = false,
  className,
  bubbleClassName,
}: Readonly<{
  audience?: keyof typeof AUDIENCES;
  /** Übersetzter Text für das Elternportal; sonst der deutsche Standardtext. */
  label?: string;
  /** Tooltip text — pass one of `PARENT_VISIBLE_HINTS`. */
  hint?: string;
  /** Icon-only variant for inline form labels. */
  compact?: boolean;
  className?: string;
  /** Reposition the tooltip bubble where a container would clip it. */
  bubbleClassName?: string;
}>) {
  const { label: defaultLabel, tone, compactClass } = AUDIENCES[audience];
  const text = label ?? defaultLabel;
  return (
    <Tooltip
      content={hint}
      className={cn("align-middle", className)}
      bubbleClassName={bubbleClassName}
    >
      {compact ? (
        <span
          className={cn(
            "inline-flex items-center rounded-full p-1",
            compactClass,
          )}
        >
          <Eye className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="sr-only">{text}</span>
        </span>
      ) : (
        <StatusBadge tone={tone} compact icon={Eye} label={text} />
      )}
    </Tooltip>
  );
}

/**
 * The key for the markers above. Two jobs, one line: it labels the icon-only
 * field marker (which shows the eye without the words), and it states the
 * inverse, which the markers alone can only imply. What the marker means is
 * already in its own text, so the legend does not repeat it.
 */
export function ParentVisibilityLegend({
  className,
}: Readonly<{ className?: string }>) {
  return (
    // Inline flow rather than flex, so a narrow container wraps the sentence
    // around the marker instead of stranding it on a line of its own.
    <p className={cn("text-xs leading-relaxed text-gray-500", className)}>
      <VisibilityBadge className="mr-1.5" />
      Nicht gekennzeichnete Angaben bleiben intern.
    </p>
  );
}
