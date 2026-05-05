// Helpers for computing staff work-time metrics (Soll, Ist, Saldo).
//
// All minutes are integers. "dayOfWeek" uses ISO convention: 0=Mon, 6=Sun.
// Sat/Sun are intentionally excluded from the Soll when they are 0 in the
// schedule — the typical Mon–Fri use case.

import type {
  ScheduleEntry,
  StaffHistorySession,
  StaffSchedule,
} from "./staff-api";

/**
 * Convert a JavaScript Date to our ISO dayOfWeek (0=Mon, 6=Sun).
 */
export function toIsoDayOfWeek(date: Date): number {
  const jsDay = date.getDay(); // 0=Sun, 1=Mon, ...
  return jsDay === 0 ? 6 : jsDay - 1;
}

/**
 * Parse a YYYY-MM-DD string (or an ISO datetime that starts with one) into
 * a local Date at 00:00. Used for grouping sessions by day without timezone
 * drift.
 */
export function parseSessionDate(input: string): Date | null {
  const datePart = input.slice(0, 10);
  const [y, m, d] = datePart.split("-").map(Number);
  if (
    y === undefined ||
    m === undefined ||
    d === undefined ||
    Number.isNaN(y) ||
    Number.isNaN(m) ||
    Number.isNaN(d)
  ) {
    return null;
  }
  return new Date(y, m - 1, d);
}

/**
 * Build a single-week map from dayOfWeek (0-6) to target minutes for a
 * specific rotation week. Useful for the simple-week display path; for the
 * full pattern with rotation use {@link resolveTargetForDate}.
 */
export function buildTargetMap(
  entries: ScheduleEntry[],
  weekIndex = 0,
): Map<number, number> {
  const map = new Map<number, number>();
  for (const entry of entries) {
    if (entry.weekIndex === weekIndex) {
      map.set(entry.dayOfWeek, entry.targetMinutes);
    }
  }
  return map;
}

/**
 * Compute the rotation week index for a given calendar date relative to
 * the schedule's anchor. Returns 0 for single-week patterns.
 */
export function resolveWeekIndex(
  schedule: Pick<StaffSchedule, "rotationLength" | "rotationAnchorDate">,
  date: Date,
): number {
  const rotation = Math.max(1, schedule.rotationLength);
  if (rotation === 1) return 0;
  const anchor = parseSessionDate(schedule.rotationAnchorDate);
  if (!anchor) return 0;
  const day = 24 * 60 * 60 * 1000;
  const aMs = startOfWeek(anchor).getTime();
  const dMs = startOfWeek(
    new Date(date.getFullYear(), date.getMonth(), date.getDate()),
  ).getTime();
  const weeks = Math.floor((dMs - aMs) / (7 * day));
  const mod = weeks % rotation;
  return mod < 0 ? mod + rotation : mod;
}

/**
 * Look up the Soll-Minuten for a single calendar date, honouring rotation.
 */
export function resolveTargetForDate(
  schedule: StaffSchedule,
  date: Date,
): number {
  const weekIndex = resolveWeekIndex(schedule, date);
  const dow = toIsoDayOfWeek(date);
  for (const entry of schedule.entries) {
    if (entry.weekIndex === weekIndex && entry.dayOfWeek === dow) {
      return entry.targetMinutes;
    }
  }
  return 0;
}

/**
 * Sum the schedule target minutes for all calendar days in [from, to]
 * (inclusive). Honours multi-week rotations via the schedule's anchor.
 */
export function computeSollForRange(
  schedule: StaffSchedule,
  from: Date,
  to: Date,
): number {
  const start = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const end = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  const dayMs = 24 * 60 * 60 * 1000;
  const totalDays = Math.floor((end.getTime() - start.getTime()) / dayMs) + 1;
  if (totalDays <= 0) return 0;
  let sum = 0;
  for (let i = 0; i < totalDays; i++) {
    const day = new Date(start);
    day.setDate(day.getDate() + i);
    sum += resolveTargetForDate(schedule, day);
  }
  return sum;
}

/**
 * Sum the actual net minutes of all sessions falling into [from, to]
 * (inclusive).
 */
export function computeIstForRange(
  sessions: StaffHistorySession[],
  from: Date,
  to: Date,
): number {
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  let sum = 0;
  for (const session of sessions) {
    const key = session.date.slice(0, 10);
    if (key >= fromKey && key <= toKey) {
      sum += session.net_minutes ?? 0;
    }
  }
  return sum;
}

/**
 * Group sessions by local date key (YYYY-MM-DD) with summed net minutes.
 */
export function groupSessionsByDay(
  sessions: StaffHistorySession[],
): Map<string, number> {
  const map = new Map<string, number>();
  for (const session of sessions) {
    const key = session.date.slice(0, 10);
    map.set(key, (map.get(key) ?? 0) + (session.net_minutes ?? 0));
  }
  return map;
}

/**
 * Format a date as YYYY-MM-DD in local time (no timezone shift).
 */
export function toDateKey(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/**
 * Monday of the week containing the given date (local time).
 */
export function startOfWeek(date: Date): Date {
  const result = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const dow = toIsoDayOfWeek(result); // 0=Mon
  result.setDate(result.getDate() - dow);
  return result;
}

/**
 * Sunday of the week containing the given date (local time).
 */
export function endOfWeek(date: Date): Date {
  const monday = startOfWeek(date);
  const sunday = new Date(monday);
  sunday.setDate(sunday.getDate() + 6);
  return sunday;
}

/**
 * First day of the month (local).
 */
export function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

/**
 * Last day of the month (local).
 */
export function endOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0);
}

/**
 * First day of the year (local).
 */
export function startOfYear(date: Date): Date {
  return new Date(date.getFullYear(), 0, 1);
}

/**
 * Returns true when two dates fall on the same local calendar day.
 */
export function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export interface StaffMetrics {
  // Current week
  weekSoll: number;
  weekIst: number;
  weekDelta: number;
  // Current month
  monthSoll: number;
  monthIst: number;
  monthDelta: number;
  // Year-to-date
  ytdSoll: number;
  ytdIst: number;
  ytdBalance: number; // total overtime/undertime since Jan 1
}

/**
 * Compute all KPI metrics for a staff member given their schedule, the
 * sessions covering the year-to-date range, and a reference "now" date.
 *
 * Future days (after `now`) are excluded from Soll so the balance only
 * counts days that have already happened.
 */
export function computeStaffMetrics(
  schedule: StaffSchedule,
  sessions: StaffHistorySession[],
  now: Date,
): StaffMetrics {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

  const weekStart = startOfWeek(today);
  const weekEnd = endOfWeek(today);
  // Soll for week stops at today (no future credit)
  const weekSollEnd = today < weekEnd ? today : weekEnd;
  const weekSoll = computeSollForRange(schedule, weekStart, weekSollEnd);
  const weekIst = computeIstForRange(sessions, weekStart, weekEnd);
  const weekDelta = weekIst - weekSoll;

  const monthStart = startOfMonth(today);
  const monthEnd = endOfMonth(today);
  const monthSollEnd = today < monthEnd ? today : monthEnd;
  const monthSoll = computeSollForRange(schedule, monthStart, monthSollEnd);
  const monthIst = computeIstForRange(sessions, monthStart, monthEnd);
  const monthDelta = monthIst - monthSoll;

  const yearStart = startOfYear(today);
  const ytdSoll = computeSollForRange(schedule, yearStart, today);
  const ytdIst = computeIstForRange(sessions, yearStart, today);
  const ytdBalance = ytdIst - ytdSoll;

  return {
    weekSoll,
    weekIst,
    weekDelta,
    monthSoll,
    monthIst,
    monthDelta,
    ytdSoll,
    ytdIst,
    ytdBalance,
  };
}

/**
 * Ampel color for a delta. Green if within ±5% of target (or target 0),
 * amber if positive (overtime), gray if negative (undertime).
 */
export function getDeltaStatus(
  delta: number,
  target: number,
): "green" | "amber" | "gray" {
  if (target === 0) return delta > 0 ? "amber" : "gray";
  const threshold = Math.max(15, target * 0.05); // at least 15min tolerance
  if (Math.abs(delta) <= threshold) return "green";
  if (delta > 0) return "amber";
  return "gray";
}

/**
 * Build a 6-row x 7-col grid (Mon-Sun) for a given month. Each cell has
 * the Date and a flag for "out of month". The grid always starts on the
 * Monday on or before the 1st and ends on the Sunday on or after the last.
 */
export interface CalendarCell {
  date: Date;
  inMonth: boolean;
}

export function buildMonthGrid(monthAnchor: Date): CalendarCell[][] {
  const first = startOfMonth(monthAnchor);
  const gridStart = startOfWeek(first);
  const rows: CalendarCell[][] = [];
  const cursor = new Date(gridStart);
  for (let r = 0; r < 6; r++) {
    const row: CalendarCell[] = [];
    for (let c = 0; c < 7; c++) {
      row.push({
        date: new Date(cursor),
        inMonth: cursor.getMonth() === first.getMonth(),
      });
      cursor.setDate(cursor.getDate() + 1);
    }
    rows.push(row);
  }
  return rows;
}

/**
 * Format month header like "April 2026"
 */
export function formatMonthHeader(date: Date): string {
  const months = [
    "Januar",
    "Februar",
    "Maerz",
    "April",
    "Mai",
    "Juni",
    "Juli",
    "August",
    "September",
    "Oktober",
    "November",
    "Dezember",
  ];
  return `${months[date.getMonth()]} ${date.getFullYear()}`;
}
