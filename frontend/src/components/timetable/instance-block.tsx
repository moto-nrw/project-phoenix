"use client";

/**
 * InstanceBlock — absolutely-positioned event card for the WeeklyCalendarGrid.
 *
 * Visual style matches the TemplateCard idiom:
 * - 3px left color-bar tied to activity type via `getActivityColor`
 * - neutral surface with single hairline border and subtle shadow
 * - status indicator as a 6px dot, not a noisy pill
 * - text hierarchy: title 12px semibold, time/room 10px gray-500
 *
 * Brand colours come exclusively from the helpers in `timetable-helpers.ts`.
 * Cancelled instances render with a dashed red border + line-through, matching
 * the month-grid convention.
 */

import { TriangleAlert } from "lucide-react";

import { getActivityColor } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { timetableStatusColors } from "./timetable-style";

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

  // #1840 Vertretungsplan deviation signals. A removed substitute keeps
  // is_substitute=true but is marked is_absent=true — they are no longer
  // covering, so only a NON-absent substitute counts as an active replacement.
  const isUnderstaffedAck = instance.understaffedAck === true && !isCancelled;
  const hasSubstitute = instance.staff.some(
    (s) => s.isSubstitute && !s.isAbsent,
  );
  const absentCount = instance.absentStaffCount;

  const ringClass = isSelected
    ? "ring-2 ring-offset-1 ring-gray-900"
    : "ring-0 hover:ring-1 hover:ring-gray-300";

  const borderClass = isCancelled
    ? "border border-dashed border-[#FF3130]"
    : "border border-gray-200";
  // A deliberately-unstaffed block gets an amber left bar so it reads as a
  // known shortfall, not a normal block.
  const activityColor = isCancelled
    ? timetableStatusColors.cancelled
    : isUnderstaffedAck
      ? "#EAB308"
      : getActivityColor(instance.activityType);

  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={isSelected}
      className={`group absolute ${ringClass} ${borderClass} cursor-pointer overflow-hidden rounded-xl border-l-[3px] bg-white text-left shadow-sm transition-[background-color,border-color,box-shadow] hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none`}
      style={{
        top: `${top}px`,
        height: `${Math.max(height, 18)}px`,
        left,
        width,
        borderLeftColor: activityColor,
        padding: isTiny ? "2px 6px" : "4px 6px",
      }}
    >
      <div className="flex h-full flex-col gap-0.5">
        <div className="flex items-start justify-between gap-1">
          <span
            className={`min-w-0 truncate font-semibold text-gray-900 ${
              isCancelled ? "text-gray-400 line-through" : ""
            } ${isTiny ? "text-[11px]" : "text-xs"}`}
          >
            {isActive && !isCancelled && (
              <span
                className="mr-1 inline-block h-1.5 w-1.5 rounded-full bg-[#83CD2D] align-middle"
                aria-label="läuft"
              />
            )}
            {instance.title}
          </span>
          {isUnderstaffedAck ? (
            <TriangleAlert
              className="h-3 w-3 shrink-0 text-[#EAB308]"
              aria-label="Bewusst unbesetzt"
            />
          ) : (
            hasConflict && (
              <TriangleAlert
                className="h-3 w-3 shrink-0 text-[#EAB308]"
                aria-label={`${instance.conflictWarnings.length} Konflikte`}
              />
            )
          )}
        </div>

        {!isTiny && (
          <div
            className={`truncate text-[10px] tabular-nums ${
              isCancelled ? "text-gray-400 line-through" : "text-gray-500"
            }`}
          >
            {instance.startTime} – {instance.endTime}
            {!isCompact && instance.roomName ? ` · ${instance.roomName}` : ""}
          </div>
        )}

        {!isCompact && instance.isSpontaneous && !isCancelled && (
          <div
            className="truncate text-[10px] font-semibold text-gray-600"
            title="Dieser Termin wurde spontan gestartet und war nicht geplant."
          >
            <span
              className="mr-1 inline-block h-1.5 w-1.5 rounded-full align-middle"
              style={{
                backgroundColor: activityColor,
              }}
              aria-hidden
            />
            Spontan
          </div>
        )}

        {!isCompact && !isCancelled && (
          <div className="truncate text-[10px] text-gray-500">
            {instance.staffCount} P ·{" "}
            {instance.expectedStudentsCount + instance.presentStudentsCount} K
            {isActive &&
            instance.expectedStudentsCount + instance.presentStudentsCount > 0
              ? ` · ${instance.presentStudentsCount} anwesend`
              : ""}
          </div>
        )}

        {!isCompact && !isCancelled && (absentCount > 0 || hasSubstitute) && (
          <div className="flex flex-wrap gap-1 text-[10px]">
            {absentCount > 0 && (
              <span className="font-semibold text-[#CC2626]">
                {absentCount} abwesend
              </span>
            )}
            {hasSubstitute && (
              <span className="font-semibold text-[#5A8E1F]">Ersatz</span>
            )}
          </div>
        )}
      </div>
    </button>
  );
}
