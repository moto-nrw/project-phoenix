// Helper functions for time tracking data transformation and calculations

import { expandClosingDaysToMap } from "~/lib/closing-day-helpers";

// Backend response types (snake_case, numbers for IDs)
export interface BackendWorkSession {
  id: number;
  staff_id: number;
  date: string;
  status: "present" | "home_office";
  // Channel the row was created on (active.work_sessions.source, Issue #1368).
  // Optional because pre-#1368 payloads predate the column.
  source?: "app" | "nfc" | "unknown";
  check_in_time: string;
  check_out_time: string | null;
  break_minutes: number;
  notes: string;
  auto_checked_out: boolean;
  created_by: number;
  updated_by: number | null;
  created_at: string;
  updated_at: string;
}

export interface BackendWorkSessionBreak {
  id: number;
  session_id: number;
  started_at: string;
  ended_at: string | null;
  duration_minutes: number;
  planned_end_time: string | null;
  created_at: string;
  updated_at: string;
}

export interface BackendWorkSessionHistory extends BackendWorkSession {
  net_minutes: number;
  is_overtime: boolean;
  is_break_compliant: boolean;
  rest_period_warning: string | null;
  breaks: BackendWorkSessionBreak[] | null;
  edit_count: number;
  audit_count?: number;
}

interface BackendWeeklySummary {
  week_number: number;
  year: number;
  total_net_minutes: number;
  target_minutes: number | null;
  delta_minutes: number | null;
  session_count: number;
  is_over_weekly_max: boolean;
}

export interface BackendHistoryResponse {
  sessions: BackendWorkSessionHistory[];
  weekly_summaries: BackendWeeklySummary[];
}

export interface BackendWorkSessionEdit {
  id: number;
  session_id: number;
  staff_id: number;
  edited_by: number;
  field_name: string;
  old_value: string | null;
  new_value: string | null;
  notes: string | null;
  created_at: string;
  // Decorated by the service layer (WorkSessionEditView). Older clients of
  // the audit endpoint may still see absent fields; we tolerate missing
  // values for backwards compatibility.
  editor_name?: string;
  is_self_edit?: boolean;
}

// Backend absence response type (snake_case)
export interface BackendStaffAbsence {
  id: number;
  staff_id: number;
  absence_type: string;
  date_start: string;
  date_end: string;
  half_day: boolean;
  start_half_day?: boolean;
  end_half_day?: boolean;
  note: string;
  status: string;
  approved_by: number | null;
  approved_at: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
  duration_days: number;
  working_days?: number | null;
  decision_note?: string;
  requested_at?: string;
  substitute_staff_id?: number | null;
}

// Frontend absence type
export type AbsenceType =
  "sick" | "vacation" | "training" | "other" | "comp_time";

export interface StaffAbsence {
  id: string;
  staffId: string;
  absenceType: AbsenceType;
  dateStart: string;
  dateEnd: string;
  halfDay: boolean;
  startHalfDay: boolean;
  endHalfDay: boolean;
  note: string;
  status: string;
  approvedBy: string | null;
  approvedAt: string | null;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  durationDays: number;
  workingDays: number | null;
  decisionNote: string;
  requestedAt: string;
  substituteStaffId: string | null;
}

export const absenceTypeLabels: Record<AbsenceType, string> = {
  sick: "Krank",
  vacation: "Urlaub",
  training: "Fortbildung",
  other: "Sonstige",
  comp_time: "Freizeitausgleich",
};

export function mapStaffAbsenceResponse(
  data: BackendStaffAbsence,
): StaffAbsence {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    absenceType: data.absence_type as AbsenceType,
    dateStart: data.date_start.split("T")[0] ?? data.date_start,
    dateEnd: data.date_end.split("T")[0] ?? data.date_end,
    halfDay: data.half_day,
    startHalfDay: data.start_half_day ?? data.half_day,
    endHalfDay: data.end_half_day ?? data.half_day,
    note: data.note ?? "",
    status: data.status,
    approvedBy: data.approved_by?.toString() ?? null,
    approvedAt: data.approved_at ?? null,
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    durationDays: data.duration_days,
    workingDays: data.working_days ?? null,
    decisionNote: data.decision_note ?? "",
    requestedAt: data.requested_at ?? data.created_at,
    substituteStaffId: data.substitute_staff_id?.toString() ?? null,
  };
}

// Frontend types (camelCase, string IDs)
export interface WorkSession {
  id: string;
  staffId: string;
  date: string;
  status: "present" | "home_office";
  source?: "app" | "nfc" | "unknown";
  checkInTime: string;
  checkOutTime: string | null;
  breakMinutes: number;
  notes: string;
  autoCheckedOut: boolean;
  createdBy: string;
  updatedBy: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface WorkSessionBreak {
  id: string;
  sessionId: string;
  startedAt: string;
  endedAt: string | null;
  durationMinutes: number;
  plannedEndTime: string | null;
}

export interface WorkSessionHistory extends WorkSession {
  netMinutes: number;
  isOvertime: boolean;
  isBreakCompliant: boolean;
  restPeriodWarning: string | null;
  breaks: WorkSessionBreak[];
  editCount: number;
  auditCount?: number;
}

export interface WeeklySummary {
  weekNumber: number;
  year: number;
  totalNetMinutes: number;
  targetMinutes: number | null;
  deltaMinutes: number | null;
  sessionCount: number;
  isOverWeeklyMax: boolean;
}

export interface WorkSessionEdit {
  id: string;
  sessionId: string;
  staffId: string;
  editedBy: string;
  fieldName: string;
  oldValue: string | null;
  newValue: string | null;
  notes: string | null;
  createdAt: string;
  // Decorated server-side (WorkSessionEditView). Both fields are optional so
  // existing test fixtures and older API responses keep type-checking; the
  // mapper fills them in from the backend response and falls back to a
  // self-edit when the backend didn't decorate.
  editorName?: string;
  isSelfEdit?: boolean;
}

/**
 * Maps backend work session response to frontend type
 */
export function mapWorkSessionResponse(data: BackendWorkSession): WorkSession {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    date: data.date.split("T")[0] ?? data.date,
    status: data.status,
    source: data.source,
    checkInTime: data.check_in_time,
    checkOutTime: data.check_out_time ?? null,
    breakMinutes: data.break_minutes,
    notes: data.notes ?? "",
    autoCheckedOut: data.auto_checked_out,
    createdBy: data.created_by.toString(),
    updatedBy: data.updated_by == null ? null : data.updated_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

/**
 * Maps backend break response to frontend type
 */
export function mapWorkSessionBreakResponse(
  data: BackendWorkSessionBreak,
): WorkSessionBreak {
  return {
    id: data.id.toString(),
    sessionId: data.session_id.toString(),
    startedAt: data.started_at,
    endedAt: data.ended_at ?? null,
    durationMinutes: data.duration_minutes,
    plannedEndTime: data.planned_end_time ?? null,
  };
}

/**
 * Maps backend work session history response to frontend type
 */
export function mapWorkSessionHistoryResponse(
  data: BackendWorkSessionHistory,
): WorkSessionHistory {
  return {
    ...mapWorkSessionResponse(data),
    netMinutes: data.net_minutes,
    isOvertime: data.is_overtime,
    isBreakCompliant: data.is_break_compliant,
    restPeriodWarning: data.rest_period_warning ?? null,
    breaks: (data.breaks ?? []).map(mapWorkSessionBreakResponse),
    editCount: data.edit_count ?? 0,
    auditCount: data.audit_count ?? 0,
  };
}

/**
 * Maps backend weekly summary to frontend type
 */
function mapWeeklySummaryResponse(data: BackendWeeklySummary): WeeklySummary {
  return {
    weekNumber: data.week_number,
    year: data.year,
    totalNetMinutes: data.total_net_minutes,
    targetMinutes: data.target_minutes ?? null,
    deltaMinutes: data.delta_minutes ?? null,
    sessionCount: data.session_count,
    isOverWeeklyMax: data.is_over_weekly_max,
  };
}

/**
 * Maps the full history response (sessions + weekly summaries)
 */
export function mapHistoryResponse(data: BackendHistoryResponse): {
  sessions: WorkSessionHistory[];
  weeklySummaries: WeeklySummary[];
} {
  return {
    sessions: (data.sessions ?? []).map(mapWorkSessionHistoryResponse),
    weeklySummaries: (data.weekly_summaries ?? []).map(
      mapWeeklySummaryResponse,
    ),
  };
}

/**
 * Maps backend work session edit response to frontend type
 */
export function mapWorkSessionEditResponse(
  data: BackendWorkSessionEdit,
): WorkSessionEdit {
  return {
    id: data.id.toString(),
    sessionId: data.session_id.toString(),
    staffId: data.staff_id.toString(),
    editedBy: data.edited_by.toString(),
    fieldName: data.field_name,
    oldValue: data.old_value ?? null,
    newValue: data.new_value ?? null,
    notes: data.notes ?? null,
    createdAt: data.created_at,
    editorName: data.editor_name ?? "",
    // Older responses without is_self_edit are treated as self-edits to keep
    // legacy audit rows from being mislabeled as "vom Admin geändert".
    isSelfEdit: data.is_self_edit ?? data.edited_by === data.staff_id,
  };
}

/**
 * Formats duration in minutes to human-readable string
 * @param minutes - Duration in minutes
 * @returns Formatted string like "6h 30min" or "0min" or "--"
 */
export function formatDuration(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined || Number.isNaN(minutes)) {
    return "--";
  }

  if (minutes === 0) {
    return "0min";
  }

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  if (hours === 0) {
    return `${mins}min`;
  }

  if (mins === 0) {
    return `${hours}h`;
  }

  return `${hours}h ${mins}min`;
}

/**
 * Formats ISO timestamp to time string
 * @param isoString - ISO 8601 timestamp
 * @returns Formatted time like "08:15" or "--:--"
 */
export function formatTime(isoString: string | null | undefined): string {
  if (!isoString) {
    return "--:--";
  }

  try {
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) {
      return "--:--";
    }

    const hours = date.getHours().toString().padStart(2, "0");
    const minutes = date.getMinutes().toString().padStart(2, "0");
    return `${hours}:${minutes}`;
  } catch {
    return "--:--";
  }
}

/**
 * Gets array of dates for a week (Monday to Sunday)
 * @param date - Any date within the week
 * @returns Array of 7 dates starting from Monday
 */
export function getWeekDays(date: Date): Date[] {
  const days: Date[] = [];
  const currentDay = date.getDay();
  const mondayOffset = currentDay === 0 ? -6 : 1 - currentDay;

  for (let i = 0; i < 7; i++) {
    const day = new Date(date);
    day.setDate(date.getDate() + mondayOffset + i);
    day.setHours(0, 0, 0, 0);
    days.push(day);
  }

  return days;
}

/**
 * Gets ISO week number for a date
 * @param date - Date to get week number for
 * @returns ISO week number (1-53)
 */
export function getWeekNumber(date: Date): number {
  const target = new Date(date.valueOf());
  const dayNr = (date.getDay() + 6) % 7;
  target.setDate(target.getDate() - dayNr + 3);
  const firstThursday = target.valueOf();
  target.setMonth(0, 1);
  if (target.getDay() !== 4) {
    target.setMonth(0, 1 + ((4 - target.getDay() + 7) % 7));
  }
  return 1 + Math.ceil((firstThursday - target.valueOf()) / 604800000);
}

/**
 * Returns array of compliance warnings for a work session
 * @param session - Work session with calculated fields
 * @returns Array of warning messages
 */
export function getComplianceWarnings(session: WorkSessionHistory): string[] {
  const warnings: string[] = [];

  if (session.restPeriodWarning) {
    warnings.push(session.restPeriodWarning);
  }

  if (!session.isBreakCompliant && session.netMinutes > 0) {
    if (session.netMinutes > 540 && session.breakMinutes < 45) {
      warnings.push("Pausenzeit < 45min bei >9h Arbeitszeit");
    } else if (session.netMinutes > 360 && session.breakMinutes < 30) {
      warnings.push("Pausenzeit < 30min bei >6h Arbeitszeit");
    }
  }

  if (session.autoCheckedOut) {
    warnings.push("Automatisch ausgestempelt");
  }

  return warnings;
}

/**
 * Calculates net working minutes from check-in/out times and break
 * @param checkIn - Check-in ISO timestamp
 * @param checkOut - Check-out ISO timestamp (null if still active)
 * @param breakMinutes - Break duration in minutes
 * @returns Net working minutes, or null if still active
 */
export function calculateNetMinutes(
  checkIn: string,
  checkOut: string | null,
  breakMinutes: number,
): number | null {
  if (!checkOut) {
    return null;
  }

  try {
    const checkInDate = new Date(checkIn);
    const checkOutDate = new Date(checkOut);

    if (
      Number.isNaN(checkInDate.getTime()) ||
      Number.isNaN(checkOutDate.getTime())
    ) {
      return null;
    }

    const totalMinutes = Math.floor(
      (checkOutDate.getTime() - checkInDate.getTime()) / 60000,
    );
    return Math.max(0, totalMinutes - breakMinutes);
  } catch {
    return null;
  }
}

/**
 * Backend shape of the Monatskarte aggregate (#1842).
 */
// Poll interval for the current month's Monatskarte (admin tab and own
// view). A running session keeps growing the Ist server-side, so a card
// fetched once at mount would freeze at the check-in minute.
export const OPEN_MONTH_REFRESH_MS = 60_000;

/**
 * Largest [from, to] window the `schedule-targets` endpoint accepts, mirroring
 * `maxDailyTargetRangeDays` in `services/active/work_time_month_service.go` —
 * asking for more is a 400, not a truncated answer. Callers whose range is
 * user-controlled (the Übersicht charts reach back to the account start) must
 * split it into windows of at most this many days.
 */
export const MAX_TARGET_RANGE_DAYS = 366;

export interface BackendDailyTarget {
  date: string;
  target_minutes: number;
}

/**
 * Soll-Minuten je Kalendertag, aufgelöst gegen die Plan-Version, die AN
 * diesem Tag galt (#1842). Key ist der ISO-Tag (YYYY-MM-DD).
 *
 * Die Tagestabelle darf das Soll nicht aus dem AKTUELLEN Dienstplan ableiten:
 * Nach einer Vertragsänderung (z. B. 8h -> 4h) stünden sonst in jeder
 * vergangenen Zeile 4h, während die Monatskarte darüber die tatsächlich
 * gültigen 8h summiert — Karte und Tabelle widersprächen sich.
 */
export function mapDailyTargetsResponse(
  data: BackendDailyTarget[] | null | undefined,
): ReadonlyMap<string, number> {
  const targets = new Map<string, number>();
  for (const entry of data ?? []) {
    targets.set(entry.date.slice(0, 10), entry.target_minutes);
  }
  return targets;
}

export interface BackendHoliday {
  date: string;
  name: string;
}

/**
 * Gesetzliche Feiertage im Zeitraum, keyed nach ISO-Tag (YYYY-MM-DD),
 * Wert ist der Feiertagsname (#1418 3a). Quelle ist das Bundesland-Setting
 * des Tenants; an diesen Tagen liefert das Backend Soll = 0.
 */
export function mapHolidaysResponse(
  data: BackendHoliday[] | null | undefined,
): ReadonlyMap<string, string> {
  const holidays = new Map<string, string>();
  for (const entry of data ?? []) {
    holidays.set(entry.date.slice(0, 10), entry.name);
  }
  return holidays;
}

export interface BackendClosingDayRange {
  start_date: string;
  end_date: string;
  reason: string;
}

/**
 * OGS-Schließtage im Zeitraum, keyed nach ISO-Tag (YYYY-MM-DD), Wert ist
 * der Grund (#1418 3b). Das Backend liefert die gespeicherten Zeiträume;
 * die tageweise Expansion teilt sich diese Funktion mit den Planungsrastern
 * (#2032) über expandClosingDaysToMap. An diesen Tagen liefert das Backend
 * Soll = 0 — die Map ist reine Anzeige, keine Rechengrundlage.
 */
export function mapClosingDaysResponse(
  data: BackendClosingDayRange[] | null | undefined,
  from: string,
  to: string,
): ReadonlyMap<string, string> {
  return expandClosingDaysToMap(
    (data ?? []).map((entry) => ({
      startDate: entry.start_date,
      endDate: entry.end_date,
      reason: entry.reason,
    })),
    from,
    to,
  );
}

export interface BackendMonthSummary {
  staff_id: number;
  year: number;
  month: number;
  carry_in_minutes: number;
  target_minutes: number;
  target_minutes_to_date: number;
  actual_minutes: number;
  credited_sick_minutes: number;
  credited_vacation_minutes: number;
  credited_training_minutes: number;
  credited_other_minutes: number;
  sick_days: number;
  vacation_days: number;
  training_days: number;
  planned_shift_minutes?: number | null;
  adjustment_minutes: number;
  adjustments?: BackendBalanceAdjustment[] | null;
  balance_minutes: number;
  closing_balance_minutes: number;
  is_closed?: boolean;
  closed_at?: string | null;
  closed_by?: number | null;
  close_reason?: string | null;
  frozen_closing_balance_minutes?: number | null;
  drift_minutes?: number | null;
  carry_in_frozen?: boolean;
  carry_in_frozen_from_month?: string | null;
}

/** One Stundenkonto transaction (#1420): payout / comp-time grant / reset. */
export interface BackendBalanceAdjustment {
  id: number;
  type: string;
  minutes_delta: number;
  effective_date: string;
  note: string;
  decided_by: number;
  decided_at: string;
}

export type BalanceAdjustmentType =
  "payout" | "comp_time" | "reset" | "opening";

export interface BalanceAdjustment {
  id: string;
  type: BalanceAdjustmentType;
  minutesDelta: number;
  effectiveDate: string;
  note: string;
  decidedBy: string;
  decidedAt: string;
}

const balanceAdjustmentTypeLabels: Record<BalanceAdjustmentType, string> = {
  payout: "Auszahlung",
  comp_time: "Freizeitausgleich",
  reset: "Reset",
  opening: "Eröffnungssaldo",
};

// Defensive lookup: mapBalanceAdjustmentResponse casts the wire type without
// validation, so an unknown future type renders as its raw name instead of
// "undefined".
export function balanceAdjustmentTypeLabel(
  type: BalanceAdjustmentType,
): string {
  const labels: Record<string, string> = balanceAdjustmentTypeLabels;
  return labels[type] ?? type;
}

export function mapBalanceAdjustmentResponse(
  data: BackendBalanceAdjustment,
): BalanceAdjustment {
  return {
    id: data.id.toString(),
    type: data.type as BalanceAdjustmentType,
    minutesDelta: data.minutes_delta,
    effectiveDate: data.effective_date,
    note: data.note,
    decidedBy: data.decided_by.toString(),
    decidedAt: data.decided_at,
  };
}

/**
 * Monatskarte aggregate for one staff member and month (#1842), computed
 * live by the backend — the Übertrag updates automatically when past months
 * are corrected. plannedShiftMinutes is null when no Dienstplan rows exist
 * for the month ("kein Dienstplan gepflegt").
 */
export interface MonthSummary {
  staffId: string;
  year: number;
  month: number;
  carryInMinutes: number;
  targetMinutes: number;
  targetMinutesToDate: number;
  actualMinutes: number;
  creditedSickMinutes: number;
  creditedVacationMinutes: number;
  creditedTrainingMinutes: number;
  creditedOtherMinutes: number;
  sickDays: number;
  vacationDays: number;
  trainingDays: number;
  plannedShiftMinutes: number | null;
  adjustmentMinutes: number;
  adjustments: BalanceAdjustment[];
  balanceMinutes: number;
  closingBalanceMinutes: number;
  /**
   * Monatsabschluss state (#1417). A closed month keeps being computed live;
   * frozenClosingBalanceMinutes is the value the following month's Übertrag
   * actually uses, and driftMinutes = live closing − frozen value. Non-zero
   * drift means someone edited times inside the month after it was closed —
   * shown, never silently reconciled.
   */
  isClosed: boolean;
  closedAt: string | null;
  closedBy: string | null;
  closeReason: string | null;
  frozenClosingBalanceMinutes: number | null;
  driftMinutes: number;
  carryInFrozen: boolean;
  carryInFrozenFromMonth: string | null;
}

/**
 * One frozen Monatsabschluss row (#1417) as returned by
 * GET/POST /api/staff/time-tracking/month-close. The frozen values are what
 * the following month's Übertrag actually uses; the month itself keeps being
 * computed live and any divergence surfaces as drift on the MonthSummary.
 */
export interface BackendMonthCloseSnapshot {
  staff_id: number;
  year: number;
  month: number;
  closing_balance_minutes: number;
  carry_in_minutes: number;
  target_minutes: number;
  actual_minutes: number;
  credited_minutes: number;
  adjustment_minutes: number;
  closed_at: string;
  closed_by: number;
  close_reason?: string;
  source: string;
  reopened_at?: string;
  reopened_by?: number;
  reopen_reason?: string;
}

export interface MonthCloseSnapshot {
  staffId: string;
  year: number;
  month: number;
  closingBalanceMinutes: number;
  closedAt: string;
  closedBy: string;
  closeReason: string;
}

export function mapMonthCloseSnapshotResponse(
  data: BackendMonthCloseSnapshot,
): MonthCloseSnapshot {
  return {
    staffId: data.staff_id.toString(),
    year: data.year,
    month: data.month,
    closingBalanceMinutes: data.closing_balance_minutes,
    closedAt: data.closed_at,
    closedBy: data.closed_by.toString(),
    closeReason: data.close_reason ?? "",
  };
}

export interface BackendMonthCloseResult {
  year: number;
  month: number;
  closed_staff: number;
  skipped_staff: number;
  snapshots?: BackendMonthCloseSnapshot[] | null;
}

export interface MonthCloseResult {
  year: number;
  month: number;
  closedStaff: number;
  skippedStaff: number;
}

export function mapMonthCloseResultResponse(
  data: BackendMonthCloseResult,
): MonthCloseResult {
  return {
    year: data.year,
    month: data.month,
    closedStaff: data.closed_staff,
    skippedStaff: data.skipped_staff,
  };
}

export function mapMonthSummaryResponse(
  data: BackendMonthSummary,
): MonthSummary {
  return {
    staffId: data.staff_id.toString(),
    year: data.year,
    month: data.month,
    carryInMinutes: data.carry_in_minutes,
    targetMinutes: data.target_minutes,
    targetMinutesToDate: data.target_minutes_to_date,
    actualMinutes: data.actual_minutes,
    creditedSickMinutes: data.credited_sick_minutes,
    creditedVacationMinutes: data.credited_vacation_minutes,
    creditedTrainingMinutes: data.credited_training_minutes,
    creditedOtherMinutes: data.credited_other_minutes,
    sickDays: data.sick_days,
    vacationDays: data.vacation_days,
    trainingDays: data.training_days,
    plannedShiftMinutes: data.planned_shift_minutes ?? null,
    adjustmentMinutes: data.adjustment_minutes ?? 0,
    adjustments: (data.adjustments ?? []).map(mapBalanceAdjustmentResponse),
    balanceMinutes: data.balance_minutes,
    closingBalanceMinutes: data.closing_balance_minutes,
    isClosed: data.is_closed ?? false,
    closedAt: data.closed_at ?? null,
    closedBy: data.closed_by != null ? data.closed_by.toString() : null,
    closeReason: data.close_reason ?? null,
    frozenClosingBalanceMinutes: data.frozen_closing_balance_minutes ?? null,
    driftMinutes: data.drift_minutes ?? 0,
    carryInFrozen: data.carry_in_frozen ?? false,
    carryInFrozenFromMonth: data.carry_in_frozen_from_month ?? null,
  };
}
