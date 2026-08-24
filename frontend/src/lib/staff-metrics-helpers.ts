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
import {
  indexWorkSessionMinutesByBerlinDate,
  type StaffAbsence,
  type WorkSessionHistory,
} from "./time-tracking-helpers";

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * Soll-Minuten for a single calendar day. The two implementations below are
 * the whole difference between the two Soll sources in this file:
 * `scheduleTargetResolver` prices every day at the CURRENT schedule, while
 * `dailyTargetResolver` reads the backend's date-valid answer. Everything
 * downstream (Soll sums, absence credits, deltas) is shared, so the two
 * sources can never drift apart in the arithmetic itself.
 */
type TargetResolver = (day: Date) => number;

/**
 * Prices days at the CURRENT schedule. Days before `schedule.validFrom` are
 * worth 0: without that clamp the balance would charge Soll for a time when no
 * schedule existed yet (huge artificial minus as soon as a school goes live
 * mid-year). `honorValidFrom: false` lifts the clamp for the cumulative
 * account range, which anchors on its own configured start date.
 *
 * NOTE: this resolver is only correct for a range in which the schedule did
 * not change. For anything user-visible prefer `dailyTargetResolver` over the
 * backend's date-valid targets — see `computePeriodTotalsFromTargets`.
 */
function scheduleTargetResolver(
  schedule: StaffSchedule,
  options?: { readonly honorValidFrom?: boolean },
): TargetResolver {
  const validFrom =
    options?.honorValidFrom === false
      ? null
      : parseSessionDate(schedule.validFrom);
  return (day) =>
    validFrom && day < validFrom ? 0 : resolveTargetForDate(schedule, day);
}

/**
 * Prices days from the backend's date-valid Soll map (`schedule-targets`),
 * which resolves each day against the schedule that was valid ON that day.
 * A day outside the fetched range is worth 0.
 */
function dailyTargetResolver(
  targets: ReadonlyMap<string, number>,
): TargetResolver {
  return (day) => targets.get(toDateKey(day)) ?? 0;
}

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
 * Sum the target minutes for all calendar days in [from, to] (inclusive).
 */
function sumTargets(targetFor: TargetResolver, from: Date, to: Date): number {
  const start = new Date(from.getFullYear(), from.getMonth(), from.getDate());
  const end = new Date(to.getFullYear(), to.getMonth(), to.getDate());
  const totalDays = Math.floor((end.getTime() - start.getTime()) / DAY_MS) + 1;
  if (totalDays <= 0) return 0;
  let sum = 0;
  for (let i = 0; i < totalDays; i++) {
    const day = new Date(start);
    day.setDate(day.getDate() + i);
    sum += targetFor(day);
  }
  return sum;
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

/**
 * Sum the Soll-Minutes that count as "erfüllt durch Abwesenheit" in
 * [from, to] (inclusive). Krank/Urlaub/Fortbildung sind bezahlte Ausfallzeit
 * (ArbZG §3, BUrlG) und füllen das Soll-Konto auf, damit der Saldo am
 * Monatsende nicht durch eine Erkrankung ins Minus rutscht.
 *
 * Half-day absences zählen mit dem halben Tagessoll. Ein Tag ohne Soll
 * (vor validFrom, freier Tag, außerhalb des Target-Fensters) ist 0 wert und
 * damit automatisch nicht gutschriftsfähig.
 */
function computeAbsenceCreditForRange(
  targetFor: TargetResolver,
  absences: readonly StaffAbsenceRow[] | undefined,
  from: Date,
  to: Date,
): number {
  if (!absences || absences.length === 0) return 0;
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  let credit = 0;
  const seen = new Set<string>();
  for (const absence of sortAbsencesById(absences)) {
    const range = parseEffectiveAbsenceRange(absence);
    if (!range) continue;
    for (let i = 0; i < range.totalDays; i++) {
      const day = new Date(range.start);
      day.setDate(day.getDate() + i);
      const key = toDateKey(day);
      if (!shouldCreditAbsenceDay(key, fromKey, toKey, seen)) {
        continue;
      }
      seen.add(key);
      credit += absenceCreditForDay(targetFor, absence, day, key, range);
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
  fromKey: string,
  toKey: string,
  seen: ReadonlySet<string>,
): boolean {
  if (key < fromKey || key > toKey) return false;
  return !seen.has(key);
}

function absenceCreditForDay(
  targetFor: TargetResolver,
  absence: StaffAbsenceRow,
  day: Date,
  key: string,
  range: EffectiveAbsenceRange,
): number {
  // Freizeitausgleich (#1420 5b) wird bewusst NICHT gutgeschrieben: der Tag
  // behält sein Soll, der Saldo sinkt um das Tagessoll. Der Tag gilt trotzdem
  // als verbraucht (seen), gespiegelt zum Server (creditAbsenceDays).
  if (absence.absence_type === "comp_time") return 0;
  const target = targetFor(day);
  return isHalfAbsenceBoundary(absence, key, range.startKey, range.endKey)
    ? Math.floor(target / 2)
    : target;
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
 * Effektiv gutgeschriebene Abwesenheits-Minuten je Kalendertag, gespiegelt zum
 * Server (`addAbsenceCredits`): nur reported/approved zählen, ein Tag wird nur
 * einmal gutgeschrieben (niedrigste ID gewinnt), Halbtags-Ränder schreiben das
 * halbe Tagessoll gut, und ein Tag ohne Soll ist 0 wert.
 *
 * Per-Tag/-Woche-Serien lesen diese Map, statt die Gutschrift inline
 * herzuleiten. Die Inline-Variante schrieb jeder Abwesenheit — auch noch
 * nicht reported/approved (requested/declined/canceled) und Halbtagen — das
 * volle Tagessoll gut und widersprach damit Monatskarte und Stundenkonto
 * (#1842).
 */
export function indexAbsenceCreditByDay(
  targets: ReadonlyMap<string, number>,
  absences: readonly StaffAbsenceRow[] | undefined,
): Map<string, number> {
  const byDay = new Map<string, number>();
  if (!absences || absences.length === 0) return byDay;
  const targetFor = dailyTargetResolver(targets);
  const seen = new Set<string>();
  for (const absence of sortAbsencesById(absences)) {
    const range = parseEffectiveAbsenceRange(absence);
    if (!range) continue;
    for (let i = 0; i < range.totalDays; i++) {
      const day = new Date(range.start);
      day.setDate(day.getDate() + i);
      const key = toDateKey(day);
      if (seen.has(key)) continue;
      seen.add(key);
      byDay.set(key, absenceCreditForDay(targetFor, absence, day, key, range));
    }
  }
  return byDay;
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
 * Sum the actual net minutes of all sessions falling into [from, to]
 * (inclusive).
 *
 * `net_minutes` is authoritative as delivered: /history computes it at request
 * time and already deducts a running break (netMinutesWithBreaks in
 * labor_time_policy.go), the same math the Monatskarte uses. Deducting the
 * running break again here would count it twice and make the week card
 * understate Ist and Saldo against both the month card and the day row.
 */
function computeIstForRange(
  sessions: readonly StaffHistorySession[],
  from: Date,
  to: Date,
): number {
  const fromKey = toDateKey(from);
  const toKey = toDateKey(to);
  let sum = 0;
  const timestampedSessions = sessions.filter(
    (session) => !Number.isNaN(new Date(session.check_in_time).getTime()),
  );
  const minutesByDate = indexWorkSessionMinutesByBerlinDate(
    timestampedSessions.map((session) => staffSessionToHistorySession(session)),
  );
  for (const [key, minutes] of minutesByDate) {
    if (key >= fromKey && key <= toKey) {
      sum += Math.max(0, minutes.netMinutes);
    }
  }
  // Older fixtures and imported legacy rows can lack a parseable interval.
  // They have no usable day boundary to split, so preserve their stored date.
  for (const session of sessions) {
    if (!timestampedSessions.includes(session)) {
      const key = session.date.slice(0, 10);
      if (key >= fromKey && key <= toKey) {
        sum += Math.max(0, session.net_minutes ?? 0);
      }
    }
  }
  return sum;
}

function staffSessionToHistorySession(
  session: StaffHistorySession,
): WorkSessionHistory {
  return {
    id: session.id ?? "",
    staffId: "",
    date: session.date,
    status: session.status ?? "present",
    source: session.source,
    checkInTime: session.check_in_time,
    checkOutTime: session.check_out_time,
    breakMinutes: session.break_minutes,
    notes: session.notes ?? "",
    autoCheckedOut: session.auto_checked_out ?? false,
    createdBy: "",
    updatedBy: null,
    createdAt: "",
    updatedAt: "",
    netMinutes: session.net_minutes,
    isOvertime: false,
    isBreakCompliant: true,
    restPeriodWarning: null,
    breaks: (session.breaks ?? []).map((workBreak, index) => ({
      id: index.toString(),
      sessionId: session.id ?? "",
      startedAt: workBreak.started_at,
      endedAt: workBreak.ended_at,
      durationMinutes: 0,
      plannedEndTime: null,
    })),
    editCount: session.edit_count ?? 0,
    auditCount: session.audit_count ?? 0,
  };
}

/**
 * Soll/Ist/Saldo for one period (a week, a month).
 *
 * `soll` is the FULL period target (so "12h von 40h" reads as the whole
 * contracted week), while `ist` and `delta` stop at `effectiveEnd` —
 * otherwise a Tuesday afternoon would read "-30h Minusstunden" merely
 * because Wed–Fri have not happened yet.
 */
export interface PeriodTotals {
  readonly soll: number;
  readonly ist: number;
  readonly delta: number;
}

function computeRangeMetrics(
  targetFor: TargetResolver,
  sessions: readonly StaffHistorySession[],
  absences: readonly StaffAbsenceRow[] | undefined,
  start: Date,
  fullEnd: Date,
  effectiveEnd: Date,
): PeriodTotals {
  const soll = sumTargets(targetFor, start, fullEnd);
  const effectiveSoll = sumTargets(targetFor, start, effectiveEnd);
  // Ist stops at effectiveEnd, like the Soll it is priced against: the admin
  // API allows future-dated sessions, and counting one against a Soll that
  // deliberately stops at today would invent Überstunden the Monatskarte,
  // which excludes future sessions server-side, does not show.
  const ist =
    computeIstForRange(sessions, start, effectiveEnd) +
    computeAbsenceCreditForRange(targetFor, absences, start, effectiveEnd);

  return { soll, ist, delta: ist - effectiveSoll };
}

/**
 * Soll/Ist/Saldo for a period, priced against the backend's DATE-VALID Soll
 * (`schedule-targets`) instead of the current schedule (#1842).
 *
 * This is what every period KPI must use. `computeStaffMetrics` applies the
 * one schedule it is handed to every day in the range, so after a contract
 * change (8h -> 4h) it re-prices the days before the change at today's hours —
 * and its week/month cards then contradict the Monatskarte and the daily rows
 * on the same screen, which are both date-valid.
 *
 * `targets` must cover [start, fullEnd]; days outside it count as 0 Soll.
 */
export function computePeriodTotalsFromTargets(
  targets: ReadonlyMap<string, number>,
  sessions: readonly StaffHistorySession[],
  absences: readonly StaffAbsenceRow[] | undefined,
  start: Date,
  fullEnd: Date,
  effectiveEnd: Date,
): PeriodTotals {
  return computeRangeMetrics(
    dailyTargetResolver(targets),
    sessions,
    absences,
    start,
    fullEnd,
    effectiveEnd,
  );
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

export interface StaffMetrics {
  // Current week
  weekSoll: number;
  weekIst: number;
  weekDelta: number;
  // Current month
  monthSoll: number;
  monthIst: number;
  monthDelta: number;
  // Cumulative balance since the configured account start date or Jan 1.
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
 *
 * NOT date-valid: every Soll here is priced at the ONE schedule passed in, so
 * a range spanning a contract change is wrong by construction. User-visible
 * KPIs must come from the backend model instead — `usePeriodMetrics` for the
 * week/month cards, `useAccountBalance` for the Stundenkonto (#1842).
 */
export function computeStaffMetrics(
  schedule: StaffSchedule,
  sessions: StaffHistorySession[],
  absences: readonly StaffAbsenceRow[] | undefined,
  now: Date,
  accountStartDate?: string | null,
): StaffMetrics {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

  const periodTargets = scheduleTargetResolver(schedule);

  const weekStart = startOfWeek(today);
  const weekEnd = endOfWeek(today);
  const weekSollEnd = today < weekEnd ? today : weekEnd;
  const weekMetrics = computeRangeMetrics(
    periodTargets,
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
    periodTargets,
    sessions,
    absences,
    monthStart,
    monthEnd,
    monthSollEnd,
  );

  // The cumulative range anchors on its own configured start date, so the
  // schedule's validFrom must not clamp it a second time.
  const accountTargets = scheduleTargetResolver(schedule, {
    honorValidFrom: false,
  });
  const accountStart = resolveAccountStartDate(today, accountStartDate);
  const accountSoll = sumTargets(accountTargets, accountStart, today);
  const accountIstSessions = computeIstForRange(sessions, accountStart, today);
  const accountIstAbsence = computeAbsenceCreditForRange(
    accountTargets,
    absences,
    accountStart,
    today,
  );
  const accountIst = accountIstSessions + accountIstAbsence;
  const accountBalance = accountIst - accountSoll;

  return {
    weekSoll: weekMetrics.soll,
    weekIst: weekMetrics.ist,
    weekDelta: weekMetrics.delta,
    monthSoll: monthMetrics.soll,
    monthIst: monthMetrics.ist,
    monthDelta: monthMetrics.delta,
    accountStart,
    accountSoll,
    accountIst,
    accountBalance,
  };
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
