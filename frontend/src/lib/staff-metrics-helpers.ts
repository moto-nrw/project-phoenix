// Helpers for staff work-time metrics (Soll, Ist, Saldo).
//
// All minutes are integers. "dayOfWeek" uses ISO convention: 0=Mon, 6=Sun.
// Soll, absence credit and Saldo of a day come from the server (#1842, #6847);
// nothing here prices a day on its own.

import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "./staff-api";
import type {
  DayProjection,
  StaffAbsence,
  WorkSessionHistory,
} from "./time-tracking-helpers";

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
 * Overlapping absences credit a day only ONCE, so the iteration order decides
 * WHICH of them prices it — and a half day pays half of what a full day pays.
 * The server resolves this by lowest absence ID (`addAbsenceCredits`), the API
 * returns absences by date, so consuming them as they arrive let the weekly
 * KPI credit a day from a different absence than the Monatskarte did and the
 * two cards on the same screen disagreed (#1842). Sorting by ID here is what
 * makes "first one wins" mean the same thing on both sides.
 */
function sortAbsencesById(
  absences: readonly StaffAbsenceRow[],
): readonly StaffAbsenceRow[] {
  return [...absences].sort((a, b) => a.id - b.id);
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

export function isEffectiveAbsenceStatus(status: string): boolean {
  return status === "reported" || status === "approved";
}

export function isHalfAbsenceBoundary(
  absence: StaffAbsenceRow,
  key: string,
  startKey: string,
  endKey: string,
): boolean {
  // Mirrors backend effectiveAbsenceBoundaryHalves: older/admin-created rows
  // store only half_day=true while the newer boundary columns remain false.
  const legacyHalfDay =
    absence.half_day && !absence.start_half_day && !absence.end_half_day;
  const startHalf = legacyHalfDay || Boolean(absence.start_half_day);
  const endHalf = legacyHalfDay || Boolean(absence.end_half_day);
  if (startKey === endKey) {
    return key === startKey && (startHalf || endHalf);
  }
  return (key === startKey && startHalf) || (key === endKey && endHalf);
}

/**
 * Die Abwesenheit, die einen Kalendertag VERBRAUCHT — dieselbe Auswahl, die
 * `indexAbsenceCreditByDay` und der Server (`walkCreditedAbsenceDays`) treffen:
 * nur reported/approved zählen, und bei Überschneidungen gewinnt die niedrigste
 * ID.
 *
 * Die Gutschrift-Spalte zeigt einen vom Server gerechneten Wert; ihr Tooltip
 * muss deshalb dieselbe Abwesenheit nennen, aus der dieser Wert stammt. Eine
 * Karte in API-Reihenfolge (nach Startdatum) nennt bei Überschneidungen die
 * falsche Art.
 *
 * Freizeitausgleich steht hier bewusst mit drin: der Typ verbraucht seinen Tag,
 * schreibt aber nichts gut — die Zelle rendert dann gar keinen Tooltip.
 */
export function indexCreditingAbsenceByDay(
  absences: readonly StaffAbsenceRow[] | undefined,
): Map<string, StaffAbsenceRow> {
  const byDay = new Map<string, StaffAbsenceRow>();
  if (!absences || absences.length === 0) return byDay;
  for (const absence of sortAbsencesById(absences)) {
    const range = parseEffectiveAbsenceRange(absence);
    if (!range) continue;
    for (let i = 0; i < range.totalDays; i++) {
      const day = new Date(range.start);
      day.setDate(day.getDate() + i);
      const key = toDateKey(day);
      if (byDay.has(key)) continue;
      byDay.set(key, absence);
    }
  }
  return byDay;
}

/**
 * Soll/Ist/Saldo for one period (a week, a month).
 *
 * `soll` is the FULL period target (so "12h von 40h" reads as the whole
 * contracted week), while `ist` and `delta` stop at today —
 * otherwise a Tuesday afternoon would read "-30h Minusstunden" merely
 * because Wed–Fri have not happened yet.
 */
export interface PeriodTotals {
  readonly soll: number;
  readonly ist: number;
  readonly delta: number;
}

/**
 * Soll/Ist/Saldo of a period as the sum of the server's per-day projection
 * (`schedule-targets`, #2443). The server prices each day with the helpers
 * the Monatskarte sums: the date-valid Soll, the absence credit and the net
 * work. Future days carry their Soll but no Ist and no Saldo, so `soll` is the
 * full period while `ist` and `delta` stop at today. The week card used to
 * recompute the credit from the absence list and could then disagree with the
 * month card for the same days (#6847).
 */
export function periodTotalsFromProjection(
  projection: ReadonlyMap<string, DayProjection>,
): PeriodTotals {
  let soll = 0;
  let ist = 0;
  let delta = 0;
  for (const day of projection.values()) {
    soll += day.targetMinutes;
    ist += day.actualMinutes + day.creditMinutes;
    delta += day.balanceMinutes;
  }
  return { soll, ist, delta };
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

export function resolveAccountStartDate(
  now: Date,
  accountStartDate?: string | null,
): Date {
  return parseSessionDate(accountStartDate ?? "") ?? startOfYear(now);
}

// ─── Own-portal adapters ─────────────────────────────────────────────────────
//
// The own time-tracking endpoints return camelCase WorkSessionHistory /
// StaffAbsence, the staff-scoped admin endpoints the snake_case wire rows the
// metric helpers above take. Adapting at the edge keeps ONE metric engine for
// both portals instead of a second camelCase copy.

export function adaptHistorySessionForMetrics(
  session: WorkSessionHistory,
): StaffHistorySession {
  return {
    id: session.id || undefined,
    date: session.date,
    status: session.status,
    source: session.source,
    net_minutes: session.netMinutes,
    check_in_time: session.checkInTime,
    check_out_time: session.checkOutTime,
    break_minutes: session.breakMinutes,
    breaks: session.breaks?.map((brk) => ({
      started_at: brk.startedAt,
      ended_at: brk.endedAt,
    })),
    auto_checked_out: session.autoCheckedOut,
    notes: session.notes || undefined,
    edit_count: session.editCount,
    audit_count: session.auditCount,
    pack_up_minutes: session.packUpMinutes,
    rounding_minutes: session.roundingMinutes,
  };
}

export function adaptAbsenceForMetrics(absence: StaffAbsence): StaffAbsenceRow {
  return {
    id: Number(absence.id),
    staff_id: Number(absence.staffId),
    absence_type: absence.absenceType,
    absence_type_id: absence.absenceTypeId,
    absence_type_label: absence.absenceTypeLabel,
    date_start: absence.dateStart,
    date_end: absence.dateEnd,
    half_day: absence.halfDay,
    start_half_day: absence.startHalfDay,
    end_half_day: absence.endHalfDay,
    note: absence.note,
    status: absence.status,
    approved_by: absence.approvedBy ? Number(absence.approvedBy) : null,
    approved_at: absence.approvedAt,
    working_days: absence.workingDays,
    decision_note: absence.decisionNote,
    requested_at: absence.requestedAt,
    duration_days: absence.durationDays,
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
