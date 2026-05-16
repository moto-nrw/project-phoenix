"use client";

/**
 * InstanceBlock — absolutely-positioned event card for the WeeklyCalendarGrid.
 *
 * Visual style matches the TemplateCard idiom:
 * - 3px left color-bar tied to activity type via `getActivityColor`
 * - flat surface with single hairline border, no default shadow
 * - status indicator as a 6px dot, not a noisy pill
 * - text hierarchy: title 12px semibold, time/room 10px slate-500
 *
 * Brand colours come exclusively from the helpers in `timetable-helpers.ts`.
 * Cancelled instances render with a dashed red border + line-through, matching
 * the month-grid convention.
 */

import { TriangleAlert } from "lucide-react";

import { getActivityLightTint } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

interface InstanceBlockProps {
  instance: EnrichedInstance;
  /** Pixel offset from the day column's hour-0 line. */
  top: number;
  /** Block height in pixels. */
  height: number;
  /** Inset from the column's left edge as a percentage string ("0%" / "50%"). */
  left: string;
  /** Block width as a percentage string ("100%" / "50%"). */
  width: string;
  isSelected: boolean;
  onClick: () => void;
}

const COMPACT_HEIGHT_PX = 48;
const TINY_HEIGHT_PX = 30;

export function InstanceBlock({
  instance,
  top,
  height,
  left,
  width,
  isSelected,
  onClick,
}: InstanceBlockProps) {
  const isCancelled = instance.status === "cancelled";
  const isActive = instance.status === "active";
  const hasConflict = instance.conflictWarnings.length > 0;
  const isCompact = height <= COMPACT_HEIGHT_PX;
  const isTiny = height <= TINY_HEIGHT_PX;

  const tint = isCancelled
    ? "#FAFAFA"
    : getActivityLightTint(instance.activityType);

  const ringClass = isSelected
    ? "ring-2 ring-offset-1 ring-slate-900"
    : "ring-0 hover:ring-1 hover:ring-slate-300";

  const borderClass = isCancelled
    ? "border border-dashed border-[#FF3130]"
    : "border border-slate-200/60";

  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={isSelected}
      className={`group absolute ${ringClass} ${borderClass} cursor-pointer overflow-hidden rounded-md text-left transition-all`}
      style={{
        top: `${top}px`,
        height: `${Math.max(height, 18)}px`,
        left,
        width,
        backgroundColor: tint,
        padding: isTiny ? "2px 6px" : "4px 6px",
      }}
    >
      <div className="flex h-full flex-col gap-0.5">
        <div className="flex items-start justify-between gap-1">
          <span
            className={`min-w-0 truncate font-semibold text-slate-900 ${
              isCancelled ? "text-slate-400 line-through" : ""
            } ${isTiny ? "text-[11px]" : "text-[12px]"}`}
          >
            {isActive && !isCancelled && (
              <span
                className="mr-1 inline-block h-1.5 w-1.5 rounded-full bg-[#83CD2D] align-middle"
                aria-label="läuft"
              />
            )}
            {instance.title}
          </span>
          {hasConflict && (
            <TriangleAlert
              className="h-3 w-3 shrink-0 text-[#A16207]"
              aria-label={`${instance.conflictWarnings.length} Konflikte`}
            />
          )}
        </div>

        {!isTiny && (
          <div
            className={`truncate text-[10px] tabular-nums ${
              isCancelled ? "text-slate-400 line-through" : "text-slate-500"
            }`}
          >
            {instance.startTime} – {instance.endTime}
            {!isCompact && instance.roomName ? ` · ${instance.roomName}` : ""}
          </div>
        )}

        {!isCompact && !isCancelled && (
          <div className="truncate text-[10px] text-slate-500">
            {instance.staffCount} P ·{" "}
            {instance.expectedStudentsCount + instance.presentStudentsCount} K
            {isActive &&
            instance.expectedStudentsCount + instance.presentStudentsCount > 0
              ? ` · ${instance.presentStudentsCount} anwesend`
              : ""}
          </div>
        )}
      </div>
    </button>
  );
}
