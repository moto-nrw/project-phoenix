"use client";

/**
 * InstanceCard — one materialized activity instance as it appears on the
 * weekly planner grid.
 *
 * Visual contract:
 * - Coloured left bar based on activity type (care/activity/external)
 * - Light background tint matching the bar
 * - LAEUFT badge with green pulse-dot when status === "active"
 * - Strikethrough text + ABGESAGT badge when status === "cancelled"
 * - Conflict warning icon when conflictWarnings is non-empty
 * - Selected state (clicked) raises the card with a stronger shadow
 *
 * Intentionally read-only at this layer: the click handler bubbles up; the
 * page decides whether to open the slide-over.
 */

import { TriangleAlert } from "lucide-react";

import {
  getActivityColor,
  getActivityLightTint,
  getActivityTextColor,
  getActivityTypeBadge,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

interface InstanceCardProps {
  instance: EnrichedInstance;
  onClick?: () => void;
  isSelected?: boolean;
}

export function InstanceCard({
  instance,
  onClick,
  isSelected = false,
}: InstanceCardProps) {
  const isCancelled = instance.status === "cancelled";
  const isActive = instance.status === "active";
  const hasConflict = instance.conflictWarnings.length > 0;
  const typeBadge = getActivityTypeBadge(instance.activityType);

  const barColor = isCancelled
    ? "#FF3130"
    : getActivityColor(instance.activityType);
  const tint = isCancelled
    ? "#FAFAFA"
    : getActivityLightTint(instance.activityType);
  const textColor = isCancelled
    ? "#9CA3AF"
    : getActivityTextColor(instance.activityType);

  const ringClass = isSelected
    ? "ring-2 ring-offset-1 ring-[#5080D8]"
    : "ring-1 ring-transparent hover:ring-slate-200";

  const liveBorder = isActive ? "border-2 border-[#83CD2D]" : "border-l-[3px]";

  return (
    <button
      type="button"
      onClick={onClick}
      className={`group block w-full rounded-r-lg ${liveBorder} ${ringClass} cursor-pointer p-2 text-left transition-all`}
      style={{
        backgroundColor: tint,
        borderLeftColor: barColor,
      }}
      aria-pressed={isSelected}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1">
            <span
              className={`truncate text-[12px] font-semibold ${isCancelled ? "line-through" : ""}`}
              style={{ color: textColor }}
            >
              {instance.title}
            </span>
            {hasConflict && (
              <TriangleAlert
                className="h-3 w-3 shrink-0 text-[#A16207]"
                aria-label={`${instance.conflictWarnings.length} Konflikte`}
              />
            )}
          </div>
          <div
            className={`text-[10px] ${isCancelled ? "text-[#9CA3AF] line-through" : "text-slate-600"}`}
          >
            {instance.startTime} – {instance.endTime} •{" "}
            {instance.roomName || `Raum #${instance.roomId}`}
          </div>
          {!isCancelled && (
            <div className="text-[10px] text-slate-500">
              {instance.staffCount} P •{" "}
              {instance.expectedStudentsCount + instance.presentStudentsCount} K
              {isActive &&
              instance.expectedStudentsCount + instance.presentStudentsCount > 0
                ? ` • ${instance.presentStudentsCount} anwesend`
                : ""}
            </div>
          )}
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          {isActive && (
            <span className="inline-flex items-center gap-1 rounded-full bg-[#83CD2D] px-1.5 py-0.5 text-[8px] font-bold text-white">
              <span className="h-1 w-1 rounded-full bg-white" />
              LAEUFT
            </span>
          )}
          {isCancelled && (
            <span className="inline-flex rounded bg-[#FF3130] px-1.5 py-0.5 text-[8px] font-bold text-white">
              ABGESAGT
            </span>
          )}
          {!isCancelled && !isActive && typeBadge && (
            <span
              className="inline-flex rounded px-1.5 py-0.5 text-[8px] font-bold text-white"
              style={{ backgroundColor: typeBadge.bg }}
            >
              {typeBadge.label}
            </span>
          )}
        </div>
      </div>
    </button>
  );
}
