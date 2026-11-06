"use client";

/**
 * TimetableRatioPill — shared "X / Y" ratio indicator for the timetable
 * feature (issue #1838). Renders the kit pill with its symbol in front,
 * compact inline text, or a bare dot depending on available space (detail slide-over vs. weekly
 * grid block vs. month/year grid cells).
 *
 * Purely presentational: callers compute their own TimetableTone via a
 * small pure function, since the threshold rules differ per metric
 * (attendance presence vs. staffing-capacity coverage).
 */

import type { ReactNode } from "react";

import { timetableToneColors, type TimetableTone } from "./timetable-style";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { StatusDot } from "~/components/ui/status-dot";

interface TimetableRatioPillProps {
  readonly icon: ReactNode;
  readonly label: string;
  /** e.g. "2 / 3" or "–" when there is nothing to count. */
  readonly value: string;
  readonly tone: TimetableTone;
  /**
   * "pill" (default): full chrome for the detail slide-over.
   * "compact": inline dot + text for the weekly grid block.
   * "dot": bare colored dot for month/year grid cells (no room for text).
   */
  readonly variant?: "pill" | "compact" | "dot";
  /** Accessible name — required content for "dot" (no visible text). */
  readonly title?: string;
}

export function TimetableRatioPill({
  icon,
  label,
  value,
  tone,
  variant = "pill",
  title,
}: Readonly<TimetableRatioPillProps>) {
  const color = timetableToneColors[tone];
  const accessibleName = title ?? `${label}: ${value}`;

  if (variant === "dot") {
    return <StatusDot color={color} label={accessibleName} />;
  }

  if (variant === "compact") {
    return (
      <span className="inline-flex items-center gap-1" title={accessibleName}>
        <StatusDot color={color} decorative />
        <span className="sr-only">{label}: </span>
        <span className="tabular-nums">{value}</span>
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1">
      <span aria-hidden className="flex shrink-0 items-center">
        {icon}
      </span>
      <StatusColorBadge label={`${label} ${value}`} color={color} />
    </span>
  );
}

/** Attendance tone — verbatim port of the old StatsRow thresholds (issue
 * #1838 renames StatPill -> TimetableRatioPill, no UX change here). */
export function attendanceStudentTone(
  present: number,
  total: number,
): TimetableTone {
  if (total === 0) return "neutral";
  return present > 0 ? "success" : "neutral";
}

export function attendanceStaffTone(
  staffCount: number,
  absentStaffCount: number,
): TimetableTone {
  if (staffCount === 0) return "neutral";
  return absentStaffCount > 0 ? "warning" : "info";
}

/** Capacity/coverage tone — new for issue #1838's Betreuungsschlüssel
 * indicator: required===0 means no ratio-derived requirement applies (e.g.
 * an empty block); otherwise color by how much of the requirement is met. */
export function capacityTone(
  assigned: number,
  required: number,
): TimetableTone {
  if (required === 0) return "neutral";
  if (assigned >= required) return "success";
  if (assigned > 0) return "warning";
  return "danger";
}
