// Helpers for computing staff work-time metrics (Soll, Ist, Saldo).
//
// All minutes are integers. "dayOfWeek" uses ISO convention: 0=Mon, 6=Sun.
// Sat/Sun are intentionally excluded from the Soll when they are 0 in the
// schedule, the typical Mon-Fri use case.

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "./staff-api";

const DAY_MS = 24 * 60 * 60 * 1000;

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
function parseSessionDate(input: string): Date | null {
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
 *
 * Tage VOR schedule.validFrom werden ignoriert. Ohne diese Klammer würde der
 * Saldo Soll-Stunden aus einer Zeit anrechnen, in der noch gar kein
 * Dienstplan galt (führt zu künstlich riesigen Minus-Salden, sobald eine
 * Schule mitten im Jahr live geht).
 */
function computeSollForRange(
  schedule: StaffSchedule,
  from: Date,
  to: Date,
): number {
  const start = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const end = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  const dayMs = 24 * 60 * 60 * 1000;
  const totalDays = Math.floor((end.getTime() - start.getTime()) / dayMs) + 1;
  if (totalDays <= 0) return 0;
  const validFrom = parseSessionDate(schedule.validFrom);
  let sum = 0;
  for (let i = 0; i < totalDays; i++) {
    const day = new Date(start);
    day.setDate(day.getDate() + i);
    if (validFrom && day < validFrom) continue;
    sum += resolveTargetForDate(schedule, day);
  }
  return sum;
}

/**
 * Sum the Soll-Minutes that count as "erfüllt durch Abwesenheit" in
 * [from, to] (inclusive). Krank/Urlaub/Fortbildung sind bezahlte Ausfallzeit
 * (ArbZG §3, BUrlG) und füllen das Soll-Konto auf, damit der Saldo am
 * Monatsende nicht durch eine Erkrankung ins Minus rutscht.
 *
 * Half-day absences zählen mit dem halben Tagessoll. Tage vor validFrom
 * werden ignoriert (analog zu computeSollForRange).
 */
function computeAbsenceCreditForRange(
  schedule: StaffSchedule,
  absences: readonly StaffAbsenceRow[] | undefined,
  from: Date,
  to: Date,
): number {
  if (!absences || absences.length === 0) return 0;
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  const validFrom = parseSessionDate(schedule.validFrom);
  let credit = 0;
  const seen = new Set<string>();
  for (const absence of absences) {
    const range = parseEffectiveAbsenceRange(absence);
    if (!range) continue;
    for (let i = 0; i < range.totalDays; i++) {
      const day = new Date(range.start);
      day.setDate(day.getDate() + i);
      const key = toDateKey(day);
      if (!shouldCreditAbsenceDay(key, day, fromKey, toKey, validFrom, seen)) {
        continue;
      }
      seen.add(key);
      credit += absenceCreditForDay(schedule, absence, day, key, range);
    }
  }
  return credit;
}

interface EffectiveAbsenceRange {
  readonly startKey: string;
  readonly endKey: string;
  readonly start: Date;
  readonly totalDays: number;
}

function parseEffectiveAbsenceRange(
  absence: StaffAbsenceRow,
): EffectiveAbsenceRange | null {
  if (!isEffectiveAbsenceStatus(absence.status)) return null;
  const startKey = absence.date_start.slice(0, 10);
  const endKey = absence.date_end.slice(0, 10);
  const start = parseSessionDate(startKey);
  const end = parseSessionDate(endKey);
  if (!start || !end) return null;

  return {
    startKey,
    endKey,
    start,
    totalDays: Math.floor((end.getTime() - start.getTime()) / DAY_MS) + 1,
  };
}

function shouldCreditAbsenceDay(
  key: string,
  day: Date,
  fromKey: string,
  toKey: string,
  validFrom: Date | null,
  seen: ReadonlySet<string>,
): boolean {
  if (key < fromKey || key > toKey) return false;
  if (seen.has(key)) return false;
  return !(validFrom && day < validFrom);
}

function absenceCreditForDay(
  schedule: StaffSchedule,
  absence: StaffAbsenceRow,
  day: Date,
  key: string,
  range: EffectiveAbsenceRange,
): number {
  const target = resolveTargetForDate(schedule, day);
  return isHalfAbsenceBoundary(absence, key, range.startKey, range.endKey)
    ? Math.floor(target / 2)
    : target;
}

function isEffectiveAbsenceStatus(status: string): boolean {
  return status === "reported" || status === "approved";
}

function isHalfAbsenceBoundary(
  absence: StaffAbsenceRow,
  key: string,
  startKey: string,
  endKey: string,
): boolean {
  const hasBoundaryFields =
    absence.start_half_day !== undefined || absence.end_half_day !== undefined;
  const startHalf = hasBoundaryFields
    ? Boolean(absence.start_half_day)
    : absence.half_day;
  const endHalf = hasBoundaryFields
    ? Boolean(absence.end_half_day)
    : absence.half_day;
  if (startKey === endKey) {
    return key === startKey && (startHalf || endHalf);
  }
  return (key === startKey && startHalf) || (key === endKey && endHalf);
}

/**
 * Sum the actual net minutes of all sessions falling into [from, to]
 * (inclusive).
 */
function computeIstForRange(
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

interface RangeMetrics {
  readonly fullSoll: number;
  readonly ist: number;
  readonly delta: number;
}

function computeRangeMetrics(
  schedule: StaffSchedule,
  sessions: StaffHistorySession[],
  absences: readonly StaffAbsenceRow[] | undefined,
  start: Date,
  fullEnd: Date,
  effectiveEnd: Date,
): RangeMetrics {
  const fullSoll = computeSollForRange(schedule, start, fullEnd);
  const effectiveSoll = computeSollForRange(schedule, start, effectiveEnd);
  const ist =
    computeIstForRange(sessions, start, fullEnd) +
    computeAbsenceCreditForRange(schedule, absences, start, effectiveEnd);

  return {
    fullSoll,
    ist,
    delta: ist - effectiveSoll,
  };
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

export interface StaffMetrics {
  // Current week
  weekSoll: number;
  weekIst: number;
  weekDelta: number;
  // Current month
  monthSoll: number;
  monthIst: number;
  monthDelta: number;
  // Cumulative balance since the schedule's validFrom (or today's year as fallback).
  // accountStart is the anchor date used for the cumulative cards.
  accountStart: Date;
  accountSoll: number;
  accountIst: number;
  accountBalance: number;
}

/**
 * Compute all KPI metrics for a staff member given their schedule, the
 * sessions covering the relevant range, the absences covering the same
 * range, and a reference "now" date.
 *
 * Future days (after `now`) are excluded from Soll so the balance only
 * counts days that have already happened.
 *
 * Krank/Urlaub/Fortbildung zählen als Soll-erfüllt (Ansatz B): Ist erhält
 * für jeden Absence-Tag das tagesbezogene Soll als Credit. Damit fällt der
 * Saldo nicht ins Minus, nur weil jemand legitim ausfällt.
 */
export function computeStaffMetrics(
  schedule: StaffSchedule,
  sessions: StaffHistorySession[],
  absences: readonly StaffAbsenceRow[] | undefined,
  now: Date,
): StaffMetrics {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

  const weekStart = startOfWeek(today);
  const weekEnd = endOfWeek(today);
  // weekSoll = full contracted week (filtered by validFrom). weekSollProrated
  // = only days up to today, used for the delta so a Tuesday afternoon does
  // not read "-30h Minusstunden" just because Wed-Fri have not happened yet.
  const weekSollEnd = today < weekEnd ? today : weekEnd;
  const weekMetrics = computeRangeMetrics(
    schedule,
    sessions,
    absences,
    weekStart,
    weekEnd,
    weekSollEnd,
  );

  const monthStart = startOfMonth(today);
  const monthEnd = endOfMonth(today);
  const monthSollEnd = today < monthEnd ? today : monthEnd;
  const monthMetrics = computeRangeMetrics(
    schedule,
    sessions,
    absences,
    monthStart,
    monthEnd,
    monthSollEnd,
  );

  // Stundenkonto: starts at the schedule's validFrom (or Jan 1 of the current
  // year as fallback when no schedule exists yet). Anything before that date
  // doesn't count, see computeSollForRange for the per-day guard.
  const accountStart =
    parseSessionDate(schedule.validFrom) ?? startOfYear(today);
  const accountSoll = computeSollForRange(schedule, accountStart, today);
  const accountIstSessions = computeIstForRange(sessions, accountStart, today);
  const accountIstAbsence = computeAbsenceCreditForRange(
    schedule,
    absences,
    accountStart,
    today,
  );
  const accountIst = accountIstSessions + accountIstAbsence;
  const accountBalance = accountIst - accountSoll;

  return {
    weekSoll: weekMetrics.fullSoll,
    weekIst: weekMetrics.ist,
    weekDelta: weekMetrics.delta,
    monthSoll: monthMetrics.fullSoll,
    monthIst: monthMetrics.ist,
    monthDelta: monthMetrics.delta,
    accountStart,
    accountSoll,
    accountIst,
    accountBalance,
  };
}

// Ampel color for a delta. Fixed 15-min tolerance, target-agnostic.
export function getDeltaStatus(
  delta: number,
  _target?: number,
): "green" | "amber" | "gray" {
  const threshold = 15;
  if (Math.abs(delta) <= threshold) return "green";
  if (delta > 0) return "amber";
  return "gray";
}
