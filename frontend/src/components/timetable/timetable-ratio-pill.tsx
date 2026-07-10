"use client";

/**
 * TimetableRatioPill — shared "X / Y" ratio indicator for the timetable
 * feature (issue #1838). Renders a tone-colored pill, compact inline text,
 * or a bare dot depending on available space (detail slide-over vs. weekly
 * grid block vs. month/year grid cells).
 *
 * Purely presentational: callers compute their own TimetableTone via a
 * small pure function, since the threshold rules differ per metric
 * (attendance presence vs. staffing-capacity coverage).
 */

import type { ReactNode } from "react";

import { timetableToneColors, type TimetableTone } from "./timetable-style";

/** Subtracts `amount` (0-255) from each RGB channel, clamped at 0 — a
 * simple, dependency-free darken so tone text/icons stay legible on the
 * light tint background (mirrors why the old StatPill's "accent" colors
 * were hand-picked darker than the raw brand hex, e.g. amber #EAB308 as
 * plain text has poor contrast on white). */
function darkenHex(hex: string, amount: number): string {
  const num = Number.parseInt(hex.slice(1), 16);
  const r = Math.max(0, (num >> 16) - amount);
  const g = Math.max(0, ((num >> 8) & 0xff) - amount);
  const b = Math.max(0, (num & 0xff) - amount);
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, "0")}`;
}

interface TimetableRatioPillProps {
  readonly icon: ReactNode;
  readonly label: string;
  /** e.g. "2 / 3" or "—" when there is nothing to count. */
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
    return (
      <span
        className="inline-flex h-1.5 w-1.5 shrink-0 rounded-full"
        style={{ backgroundColor: color }}
      >
        <span className="sr-only">{accessibleName}</span>
      </span>
    );
  }

  if (variant === "compact") {
    return (
      <span className="inline-flex items-center gap-1" title={accessibleName}>
        <span
          aria-hidden
          className="inline-block h-1.5 w-1.5 shrink-0 rounded-full"
          style={{ backgroundColor: color }}
        />
        <span className="sr-only">{label}: </span>
        <span className="tabular-nums">{value}</span>
      </span>
    );
  }

  const textColor = darkenHex(color, 70);

  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs"
      style={{
        backgroundColor: `${color}1A`,
        borderColor: `${color}33`,
        color: textColor,
      }}
    >
      <span aria-hidden className="flex shrink-0 items-center">
        {icon}
      </span>
      <span className="font-medium" style={{ color: `${textColor}CC` }}>
        {label}
      </span>
      <span className="font-bold tabular-nums">{value}</span>
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
