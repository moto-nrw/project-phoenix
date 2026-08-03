/**
 * Helpers for the timetable feature: backend ↔ frontend mapping, week
 * date math, and brand-colour resolution.
 *
 * All week math is anchored to Monday (OGS-Schultage Mo–Fr), uses Berlin
 * local time, and emits YYYY-MM-DD strings ready for the backend
 * /api/timetable/instances endpoint.
 */

import { parseISODate, toISODate } from "./date-helpers";
import { LOCATION_COLORS } from "./location-helper";
import {
  shouldMaterializeWeekPattern,
  type CalendarPeriod,
} from "./calendar-period-helpers";
import type {
  ActivityType,
  BackendConflictCheckResult,
  BackendShiftCoverageCheckResult,
  BackendCreateTemplateResult,
  BackendAttendanceResponse,
  BackendEndTemplateResult,
  BackendEnrichedInstance,
  BackendGapInstance,
  BackendGapsResponse,
  BackendMoveStaffResponse,
  BackendStaffPoolResponse,
  MoveStaffResponse,
  StaffPoolResponse,
  BackendDeviationHistoryResponse,
  BackendApplyDeviationsResponse,
  BackendInstanceStatusResult,
  BackendMaterializeResult,
  BackendReplanWeekResult,
  BackendEditedInWindowResult,
  EditedInWindowResult,
  BackendOfferingSourcesResponse,
  BackendSplitTemplateResult,
  BackendTemplatesResponse,
  OfferingSourceOption,
  BackendStartInstanceResult,
  BackendWeeklyInstancesResponse,
  AttendanceResponse,
  ConflictCheckResult,
  CreateTemplateResult,
  EndTemplateResult,
  EnrichedInstance,
  GapInstance,
  GapsResponse,
  DeviationHistoryEvent,
  DeviationHistoryResponse,
  ApplyDeviationsResponse,
  InstanceStaffSummary,
  InstanceStudentSummary,
  InstanceStatusResult,
  MaterializeResult,
  ReplanWeekResult,
  ShiftCoverageCheckResult,
  SplitTemplateResult,
  StartInstanceResult,
  TemplatesResponse,
  TimetableTemplate,
  WeeklyInstancesResponse,
} from "./timetable-types";

/**
 * SWR-Key-Präfixe der Vertretungs-Ansicht. Producer (vertretung-view) und
 * Invalidatoren (Krankmeldungs-Kaskade #1843 in dienstplan-view und
 * abwesenheiten-tab) MÜSSEN dieselben Konstanten verwenden:
 * useTenantMutateMatching ist ein stiller No-op, wenn kein Key matcht — ein
 * umbenannter String-Literal fällt in keinem Test auf.
 */
export const VERTRETUNG_WEEK_KEY_PREFIX = "vertretung-week-";
export const VERTRETUNG_GAPS_KEY_PREFIX = "vertretung-gaps-";

/**
 * Brand-colour key per activity type. Mirrors the timetable RFC §5.5
 * ("Color-coded by type using MOTO brand colors from LOCATION_COLORS").
 *
 * - care     → blue  (#5080D8) — Mensa, Lernzeit, Freispiel
 * - activity → green (#83CD2D) — AGs (Yoga, Bouldern, …)
 * - external → orange (#F78C10) — DAZ, Musikschule, externe Förderung
 */
export function getActivityColor(type: ActivityType): string {
  switch (type) {
    case "care":
      return LOCATION_COLORS.OTHER_ROOM; // #5080D8
    case "activity":
      return LOCATION_COLORS.GROUP_ROOM; // #83CD2D
    case "external":
      return LOCATION_COLORS.SCHOOLYARD; // #F78C10
  }
}

/**
 * Light tint for card backgrounds. Hardcoded to maintain contrast with the
 * coloured left-bar; computing them at runtime would invite drift.
 */
export function getActivityLightTint(type: ActivityType): string {
  switch (type) {
    case "care":
      return "#EBF0FB";
    case "activity":
      return "#ECF7DA";
    case "external":
      return "#FCEFD9";
  }
}

/**
 * German label for the activity type, surfaced as a small badge on cards
 * when the type is non-default.
 */
export function getActivityTypeBadge(
  type: ActivityType,
): { label: string; bg: string } | null {
  switch (type) {
    case "activity":
      return { label: "AG", bg: LOCATION_COLORS.GROUP_ROOM };
    case "external":
      return { label: "EXTERN", bg: LOCATION_COLORS.SCHOOLYARD };
    case "care":
      return null;
  }
}

/**
 * Returns the Monday (00:00 local) of the week containing `ref`, then offset
 * by `weekOffset * 7` days. weekOffset=0 returns the current Monday.
 *
 * The OGS-Schultage are Mon–Fri; this helper anchors to Monday so the grid
 * always shows the same five columns regardless of the picker date.
 */
function getMondayOfWeek(ref: Date, weekOffset = 0): Date {
  const monday = new Date(ref);
  const day = monday.getDay(); // 0 = Sunday, 1 = Monday, …
  // Sunday (0) needs to walk back 6 days; otherwise day - 1.
  const diff = day === 0 ? -6 : 1 - day;
  monday.setDate(monday.getDate() + diff + weekOffset * 7);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

/**
 * Returns Monday and Sunday (00:00 local) for the requested week. The
 * backend /instances endpoint accepts an inclusive range, so Sunday is the
 * "to" boundary for the full calendar week.
 */
export function getWeekRange(
  ref: Date,
  weekOffset = 0,
): { from: Date; to: Date } {
  const from = getMondayOfWeek(ref, weekOffset);
  const to = new Date(from);
  to.setDate(to.getDate() + 6); // Mo + 6 = So
  return { from, to };
}

/**
 * Splits an inclusive date range into windows of at most `maxDays` civil
 * days each. Used to apply a template across a multi-month calendar
 * period when the backend caps a single materialize call at 56 days.
 *
 * Returns ISO date strings (YYYY-MM-DD), suitable to pass straight into
 * `timetableService.materialize(from, to)`. The window is inclusive on
 * both ends, so a 56-day chunk runs from day N through day N+55.
 */
export function chunkDateRange(
  fromISO: string,
  toISO: string,
  maxDays: number,
): Array<{ from: string; to: string }> {
  const start = new Date(`${fromISO}T00:00:00`);
  const end = new Date(`${toISO}T00:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return [];
  if (end < start) return [];

  const chunks: Array<{ from: string; to: string }> = [];
  const totalDays =
    Math.round((end.getTime() - start.getTime()) / 86400000) + 1;
  for (let dayOffset = 0; dayOffset < totalDays; dayOffset += maxDays) {
    const chunkStart = new Date(start);
    chunkStart.setDate(start.getDate() + dayOffset);
    const chunkEnd = new Date(start);
    const lastOffset = Math.min(dayOffset + maxDays - 1, totalDays - 1);
    chunkEnd.setDate(start.getDate() + lastOffset);
    chunks.push({ from: toISODate(chunkStart), to: toISODate(chunkEnd) });
  }
  return chunks;
}

/**
 * Returns every concrete calendar date in an inclusive range whose ISO
 * weekday is selected (Monday=1 … Sunday=7). This only expands weekdays;
 * A/B-week eligibility stays in the backend's existing materialization
 * predicate so the frontend never becomes a second recurrence engine.
 */
export function weekdayDatesInRange(
  fromISO: string,
  toISO: string,
  weekdays: number[],
): string[] {
  const start = new Date(`${fromISO}T00:00:00`);
  const end = new Date(`${toISO}T00:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return [];
  if (end < start) return [];

  const selected = new Set(
    weekdays.filter((weekday) => weekday >= 1 && weekday <= 7),
  );
  if (selected.size === 0) return [];

  const dates: string[] = [];
  for (let date = new Date(start); date <= end;) {
    const weekday = date.getDay() === 0 ? 7 : date.getDay();
    if (selected.has(weekday)) dates.push(toISODate(date));
    const next = new Date(date);
    next.setDate(next.getDate() + 1);
    date = next;
  }
  return dates;
}

export function latestISODate(first: string, ...dates: string[]): string {
  let latest = first;
  for (const candidate of dates) {
    if (candidate > latest) latest = candidate;
  }
  return latest;
}

/**
 * Concrete recurrence dates inside a calendar period and schedule segment.
 * `validFrom` is inclusive, `validUntil` exclusive, matching the backend
 * materializer and split-series contract.
 */
export function materializedRecurrenceDates({
  period,
  fromISO,
  weekdays,
  weekPattern,
  validFrom,
  validUntil,
}: {
  period: CalendarPeriod;
  fromISO: string;
  weekdays: number[];
  weekPattern: number;
  validFrom?: string;
  validUntil?: string;
}): string[] {
  const from = latestISODate(period.startDate, fromISO, validFrom ?? "");
  return weekdayDatesInRange(from, period.endDate, weekdays).filter(
    (dateISO) =>
      (validUntil === undefined || dateISO < validUntil) &&
      shouldMaterializeWeekPattern(period, dateISO, weekPattern),
  );
}

export { toISODate };

/**
 * Returns the ISO 8601 week number (1–53) of the given Date. Used for the
 * "KW 38" label in the week header.
 *
 * Reference algorithm: ISO 8601 defines the week containing the year's first
 * Thursday as week 1.
 */
function getISOWeekNumber(d: Date): number {
  const target = new Date(d.valueOf());
  const dayNum = (d.getDay() + 6) % 7; // Mon=0 .. Sun=6
  target.setDate(target.getDate() - dayNum + 3);
  const firstThursday = target.valueOf();
  target.setMonth(0, 1);
  if (target.getDay() !== 4) {
    target.setMonth(0, 1 + ((4 - target.getDay() + 7) % 7));
  }
  return 1 + Math.ceil((firstThursday - target.valueOf()) / 604800000);
}

const GERMAN_WEEKDAY_LONG = [
  "Sonntag",
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
];

const GERMAN_WEEKDAY_SHORT = ["So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"];

export function getGermanWeekdayLong(d: Date): string {
  return GERMAN_WEEKDAY_LONG[d.getDay()] ?? "";
}

/** Converts a German weekday name to its recurring adverb ("Montag" → "montags"). */
export function getGermanWeekdayAdverb(weekday: string): string {
  return weekday ? `${weekday.toLowerCase()}s` : "";
}

export function getGermanWeekdayShort(d: Date): string {
  return GERMAN_WEEKDAY_SHORT[d.getDay()] ?? "";
}

/**
 * "KW 38 · 22.09.–28.09.2026" — the week-navigator header. Weekday names are
 * omitted on purpose: the grid columns directly below already show Mo–So, so
 * repeating them here only bloated the label and pushed the toolbar onto a
 * second row.
 */
export function formatWeekLabel(from: Date, to: Date): string {
  const kw = getISOWeekNumber(from);
  const fromDay = String(from.getDate()).padStart(2, "0");
  const fromMonth = String(from.getMonth() + 1).padStart(2, "0");
  const toDay = String(to.getDate()).padStart(2, "0");
  const toMonth = String(to.getMonth() + 1).padStart(2, "0");
  const year = to.getFullYear();
  return `KW ${kw} · ${fromDay}.${fromMonth}.–${toDay}.${toMonth}.${year}`;
}

/**
 * "Mo 22.09." — short label used as a column header on the grid and as a
 * day label in the agenda list.
 */
export function formatDayHeader(d: Date): string {
  const day = String(d.getDate()).padStart(2, "0");
  const month = String(d.getMonth() + 1).padStart(2, "0");
  return `${getGermanWeekdayShort(d)} ${day}.${month}.`;
}

/**
 * Seven Monday→Sunday Date objects for the week containing `from`. Used to
 * label the grid columns even on weeks with no instances.
 */
export function getWeekdays(from: Date): Date[] {
  const days: Date[] = [];
  for (let i = 0; i < 7; i++) {
    const d = new Date(from);
    d.setDate(d.getDate() + i);
    days.push(d);
  }
  return days;
}

/**
 * Snappt ein Wochenend-Datum auf den folgenden Montag; Mo–Fr unverändert.
 * OGS-Betrieb ist Mo–Fr — die Vertretung zeigt nie einen Wochenendtag an.
 */
export function nextWorkdayISO(iso: string): string {
  const d = parseISODate(iso);
  const day = d.getDay(); // 0 = So, 6 = Sa
  if (day === 6) d.setDate(d.getDate() + 2);
  else if (day === 0) d.setDate(d.getDate() + 1);
  return toISODate(d);
}

/**
 * Snappt ein Wochenend-Sprungziel innerhalb eines Planungszeitraums auf den
 * nächsten Schultag: Sa/So heben auf den folgenden Montag, sofern der noch im
 * Zeitraum liegt, sonst zurück auf den vorangehenden Freitag, sofern der im
 * Zeitraum liegt. Mo–Fr (oder ein Zeitraum, der nur das Wochenende umfasst)
 * bleibt unverändert. Verhindert, dass der Zeitraumsprung bei einem
 * Samstag-Start eine Woche fast vollständig VOR dem Zeitraum anzeigt.
 */
export function firstSchoolDayInPeriod(
  periodStartISO: string,
  periodEndISO: string,
  targetISO: string,
): string {
  const target = parseISODate(targetISO);
  const day = target.getDay(); // 0 = So, 6 = Sa
  if (day !== 6 && day !== 0) return targetISO;

  const nextMonday = new Date(target);
  nextMonday.setDate(target.getDate() + (day === 6 ? 2 : 1));
  const nextMondayISO = toISODate(nextMonday);
  if (nextMondayISO <= periodEndISO) return nextMondayISO;

  const previousFriday = new Date(target);
  previousFriday.setDate(target.getDate() - (day === 6 ? 1 : 2));
  const previousFridayISO = toISODate(previousFriday);
  if (previousFridayISO >= periodStartISO) return previousFridayISO;

  return targetISO;
}

export function getMonthRange(ref: Date): { from: Date; to: Date } {
  const first = new Date(ref.getFullYear(), ref.getMonth(), 1);
  first.setHours(0, 0, 0, 0);
  const from = getMondayOfWeek(first, 0);

  const last = new Date(ref.getFullYear(), ref.getMonth() + 1, 0);
  last.setHours(0, 0, 0, 0);
  const to = new Date(getMondayOfWeek(last, 0));
  to.setDate(to.getDate() + 6);
  return { from, to };
}

export function getMonthDays(ref: Date): Date[] {
  const { from, to } = getMonthRange(ref);
  const dayCount = Math.round((to.getTime() - from.getTime()) / 86400000) + 1;
  const days: Date[] = [];
  for (let offset = 0; offset < dayCount; offset++) {
    const cursor = new Date(from);
    cursor.setDate(cursor.getDate() + offset);
    days.push(new Date(cursor));
  }
  return days;
}

export function formatMonthLabel(ref: Date): string {
  return ref.toLocaleDateString("de-DE", {
    month: "long",
    year: "numeric",
  });
}

export function getYearRange(ref: Date): { from: Date; to: Date } {
  const from = new Date(ref.getFullYear(), 0, 1);
  from.setHours(0, 0, 0, 0);
  const to = new Date(ref.getFullYear(), 11, 31);
  to.setHours(0, 0, 0, 0);
  return { from, to };
}

export function getYearMonths(ref: Date): Date[] {
  const months: Date[] = [];
  for (let month = 0; month < 12; month++) {
    months.push(new Date(ref.getFullYear(), month, 1));
  }
  return months;
}

export function formatYearLabel(ref: Date): string {
  return String(ref.getFullYear());
}

/**
 * Status-to-German UI label. Surfaced as the badge on cards.
 */
export function getStatusLabel(status: EnrichedInstance["status"]): string {
  switch (status) {
    case "planned":
      return "Geplant";
    case "active":
      return "LÄUFT";
    case "completed":
      return "Abgeschlossen";
    case "cancelled":
      return "Abgesagt";
  }
}

/**
 * Backend → frontend mapping. snake_case → camelCase, int64 IDs → strings,
 * conflict_warnings array always present (defaults to []).
 */
export function mapInstance(raw: BackendEnrichedInstance): EnrichedInstance {
  const staff: InstanceStaffSummary[] = (raw.staff ?? []).map((s) => ({
    staffId: String(s.staff_id),
    isPrimary: s.is_primary,
    isAbsent: s.is_absent,
    isSubstitute: s.is_substitute,
    absenceReason: s.absence_reason ?? undefined,
  }));
  const students: InstanceStudentSummary[] = (raw.students ?? []).map((s) => ({
    studentId: String(s.student_id),
    status: s.status,
    substatus: s.substatus,
    note: s.note,
    checkedInAt: s.checked_in_at,
    careDayStatus: s.care_day_status ?? "unknown",
  }));
  const studentIds =
    students.length > 0
      ? students.map((student) => student.studentId)
      : (raw.student_ids ?? []).map(String);

  return {
    id: String(raw.id),
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    title: raw.title,
    description: raw.description,
    notes: raw.notes,
    seriesNotes: raw.series_notes,
    status: raw.status,
    isSpontaneous: raw.is_spontaneous,
    isLive: raw.is_live,
    activityGroupId:
      raw.activity_group_id !== undefined && raw.activity_group_id !== null
        ? String(raw.activity_group_id)
        : undefined,
    listKind: raw.list_kind,
    activityType: raw.activity_type,
    roomId: String(raw.room_id),
    roomName: raw.room_name,
    staff,
    studentIds,
    students,
    staffCount: raw.staff_count,
    absentStaffCount: raw.absent_staff_count,
    understaffedAck: raw.understaffed_ack ?? false,
    understaffedNote: raw.understaffed_note ?? undefined,
    cancelReason: raw.cancel_reason ?? undefined,
    expectedStudentsCount: raw.expected_students_count,
    notScheduledStudentsCount: raw.not_scheduled_students_count ?? 0,
    presentStudentsCount: raw.present_students_count,
    requiredStaffCount: raw.required_staff_count,
    assignedStaffCount: raw.assigned_staff_count,
    requiredStaffOverride: raw.required_staff_override ?? undefined,
    conflictWarnings: (raw.conflict_warnings ?? []).map((warning) => ({
      kind: warning.kind,
      resourceId: String(warning.resource_id),
      message: warning.message,
      canOverride: warning.can_override,
    })),
  };
}

export function mapWeeklyInstances(
  raw: BackendWeeklyInstancesResponse,
): WeeklyInstancesResponse {
  return {
    from: raw.from,
    to: raw.to,
    instances: (raw.instances ?? []).map(mapInstance),
  };
}

export function mapMaterializeResult(
  raw: BackendMaterializeResult,
): MaterializeResult {
  return {
    from: raw.from,
    to: raw.to,
    instancesCreated: raw.instances_created,
    candidatesSkippedExisting: raw.candidates_skipped_existing,
    warnings: (raw.warnings ?? []).map((w) => ({
      // The codes the backend emits today are bounded; widening to string
      // here keeps the mapper forward-compatible if a new code lands without
      // requiring a frontend release.
      code: w.code as MaterializeResult["warnings"][number]["code"],
      message: w.message,
    })),
    durationMs: raw.duration_ms,
  };
}

export function mapReplanWeekResult(
  raw: BackendReplanWeekResult,
): ReplanWeekResult {
  return {
    from: raw.from,
    to: raw.to,
    deletedInstances: raw.deleted_instances,
    candidatesSkippedExisting: raw.candidates_skipped_existing,
    instancesCreated: raw.instances_created,
    instanceStudentsCreated: raw.instance_students_created,
    instanceStaffCreated: raw.instance_staff_created,
    warnings: (raw.warnings ?? []).map((w) => ({
      code: w.code as ReplanWeekResult["warnings"][number]["code"],
      message: w.message,
    })),
    durationMs: raw.duration_ms,
  };
}

export function mapEditedInWindowResult(
  raw: BackendEditedInWindowResult,
): EditedInWindowResult {
  return {
    count: raw.count,
    occurrences: (raw.occurrences ?? []).map((o) => ({
      instanceId: String(o.instance_id),
      date: o.date,
      startTime: o.start_time,
      title: o.title,
      changes:
        o.changes as EditedInWindowResult["occurrences"][number]["changes"],
    })),
  };
}

function mapGapInstance(gap: BackendGapInstance): GapInstance {
  return {
    instanceId: String(gap.instance_id),
    date: gap.date,
    title: gap.title,
    startTime: gap.start_time,
    endTime: gap.end_time,
    roomId: String(gap.room_id),
    status: gap.status,
    assignedStaffCount: gap.assigned_staff_count,
    absentStaffCount: gap.absent_staff_count,
    presentStaffCount: gap.present_staff_count,
    plannedStaffCount: gap.planned_staff_count,
    understaffedNote: gap.understaffed_note ?? undefined,
  };
}

/** Deutsche Labels für die Ereignistypen des Änderungsprotokolls (#1886). */
const DEVIATION_EVENT_LABELS: Record<string, string> = {
  absence: "Abwesenheit eingetragen",
  return_to_presence: "Rückkehr eingetragen",
  substitution: "Vertretung zugewiesen",
  substitute_removed: "Vertretung entfernt",
  cancellation: "Block abgesagt",
  understaffed_ack: "Lücke bewusst offen gelassen",
  understaffed_unack: "Lücke wieder als offen markiert",
  deviation_dropped_by_replan: "Abweichung durch Neuplanung entfernt",
  deviation_dropped_by_edit: "Abweichung durch Bearbeitung entfernt",
  sick_reported: "Krankmeldung",
  sick_cleared: "Krankmeldung zurückgenommen",
  staff_moved: "Person verschoben",
  shift_moved: "Schicht verschoben",
};

export function deviationEventLabel(eventType: string): string {
  return DEVIATION_EVENT_LABELS[eventType] ?? eventType;
}

export function mapDeviationHistory(
  raw: BackendDeviationHistoryResponse,
): DeviationHistoryResponse {
  const events: DeviationHistoryEvent[] = (raw.events ?? []).map((ev) => ({
    id: String(ev.id),
    activityGroupId:
      ev.activity_group_id != null ? String(ev.activity_group_id) : undefined,
    occurrenceDate: ev.occurrence_date,
    startTime: ev.start_time,
    instanceId: ev.instance_id != null ? String(ev.instance_id) : undefined,
    eventType: ev.event_type,
    subjectStaffId:
      ev.subject_staff_id != null ? String(ev.subject_staff_id) : undefined,
    subjectStaffName: ev.subject_staff_name ?? undefined,
    relatedStaffId:
      ev.related_staff_id != null ? String(ev.related_staff_id) : undefined,
    relatedStaffName: ev.related_staff_name ?? undefined,
    actorAccountId:
      ev.actor_account_id != null ? String(ev.actor_account_id) : undefined,
    actorName: ev.actor_name ?? undefined,
    oldValue: ev.old_value ?? undefined,
    newValue: ev.new_value ?? undefined,
    reason: ev.reason ?? undefined,
    occurredAt: ev.occurred_at,
  }));
  return { events };
}

export function mapGaps(raw: BackendGapsResponse): GapsResponse {
  return {
    from: raw.from,
    to: raw.to,
    gaps: (raw.gaps ?? []).map(mapGapInstance),
    acknowledged: (raw.acknowledged ?? []).map(mapGapInstance),
  };
}

/**
 * Anzeigename einer Personalzeile mit einheitlichem Fallback, wenn die Person
 * nicht (mehr) in der Namensauflösung steht — Liste und Editor des
 * Vertretungsbereichs zeigen denselben Text für denselben Fehlfall.
 */
export function staffLabel(
  staffNames: Map<string, string>,
  staffId: string,
): string {
  return staffNames.get(staffId) ?? `Personal #${staffId}`;
}

export function mapAttendance(
  raw: BackendAttendanceResponse,
): AttendanceResponse {
  return {
    id: String(raw.id),
    instanceId: String(raw.instance_id),
    studentId: String(raw.student_id),
    status: raw.status,
    substatus: raw.substatus,
    note: raw.note,
    checkedInAt: raw.checked_in_at,
  };
}

export function mapApplyDeviations(
  raw: BackendApplyDeviationsResponse,
): ApplyDeviationsResponse {
  return {
    instanceId: String(raw.instance_id),
    cancelled: raw.cancelled,
    understaffedAck: raw.understaffed_ack,
    affectedInstances: (raw.affected_instances ?? []).map((item) => ({
      instanceId: String(item.instance_id),
      title: item.title,
      startTime: item.start_time,
      action: item.action,
    })),
    warnings: (raw.warnings ?? []).map((warning) => ({
      instanceId: String(warning.instance_id),
      title: warning.title,
      date: warning.date,
      startTime: warning.start_time,
      endTime: warning.end_time,
    })),
  };
}

export function mapStartInstanceResult(
  raw: BackendStartInstanceResult,
): StartInstanceResult {
  return {
    instanceId: String(raw.instance_id),
    status: raw.status,
    activeGroupId: String(raw.active_group_id),
    startedAt: raw.started_at,
    warnings: (raw.warnings ?? []).map((w) => ({
      kind: w.kind,
      resourceId: String(w.resource_id),
      message: w.message,
      canOverride: w.can_override,
    })),
  };
}

export function mapInstanceStatusResult(
  raw: BackendInstanceStatusResult,
): InstanceStatusResult {
  return {
    instanceId: String(raw.instance_id),
    status: raw.status,
    completedAt: raw.completed_at,
  };
}

export function mapCreateTemplateResult(
  raw: BackendCreateTemplateResult,
): CreateTemplateResult {
  return {
    templateId: String(raw.template_id),
    timeframeId: String(raw.timeframe_id),
    scheduleIds: (raw.schedule_ids ?? []).map(String),
    instancesCreated: raw.instances_created,
    materializedFrom: raw.materialized_from,
    materializedTo: raw.materialized_to,
  };
}

export function mapSplitTemplateResult(
  raw: BackendSplitTemplateResult,
): SplitTemplateResult {
  return {
    oldTemplateId: String(raw.old_template_id),
    newTemplateId: String(raw.new_template_id),
    scheduleIds: (raw.schedule_ids ?? []).map(String),
    deletedInstances: raw.deleted_instances,
    instancesCreated: raw.instances_created,
  };
}

export function mapEndTemplateResult(
  raw: BackendEndTemplateResult,
): EndTemplateResult {
  return {
    templateId: String(raw.template_id),
    effectiveDate: raw.effective_date,
    deletedInstances: raw.deleted_instances,
  };
}

export function mapConflictCheckResult(
  raw: BackendConflictCheckResult,
): ConflictCheckResult {
  return {
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    warnings: (raw.warnings ?? []).map((warning) => ({
      kind: warning.kind,
      resourceId: String(warning.resource_id),
      message: warning.message,
      conflictingInstanceId: String(warning.conflicting_instance_id),
      conflictingTitle: warning.conflicting_title,
    })),
  };
}

export function mapShiftCoverageCheckResult(
  raw: BackendShiftCoverageCheckResult,
): ShiftCoverageCheckResult {
  return {
    coverageWarnings: (raw.coverage_warnings ?? []).map((warning) => ({
      staffId: String(warning.staff_id),
      staffName: warning.staff_name,
      date: warning.date,
      startTime: warning.start_time,
      endTime: warning.end_time,
      uncoveredStartTime: warning.uncovered_start_time,
      uncoveredEndTime: warning.uncovered_end_time,
      message: warning.message,
    })),
    coverageWarningCount:
      raw.coverage_warning_count ?? raw.coverage_warnings?.length ?? 0,
  };
}

/** #1884 Personalpool: Backend-Snake-Case auf das Frontend-Modell abbilden. */
export function mapStaffPool(raw: BackendStaffPoolResponse): StaffPoolResponse {
  return {
    instanceId: String(raw.instance_id),
    title: raw.title,
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    dienstplanInUse: raw.dienstplan_in_use,
    entries: (raw.entries ?? []).map((entry) => ({
      staffId: String(entry.staff_id),
      displayName: entry.display_name,
      category: entry.category,
      onShift: entry.on_shift,
      coversWindow: entry.covers_window,
      shiftWindows: entry.shift_windows ?? [],
      absenceReason: entry.absence_reason ?? undefined,
      assignments: (entry.assignments ?? []).map((assignment) => ({
        instanceId: String(assignment.instance_id),
        title: assignment.title,
        startTime: assignment.start_time,
        endTime: assignment.end_time,
        isSubstitute: assignment.is_substitute,
      })),
    })),
  };
}

/** #1884: Antwort des atomaren Personal-Moves abbilden. */
export function mapMoveStaff(raw: BackendMoveStaffResponse): MoveStaffResponse {
  return {
    targetInstanceId: String(raw.target_instance_id),
    sourceInstanceId:
      raw.source_instance_id != null
        ? String(raw.source_instance_id)
        : undefined,
    action: raw.action,
    timeConflicts: (raw.time_conflicts ?? []).map((conflict) => ({
      instanceId: String(conflict.instance_id),
      title: conflict.title,
      date: conflict.date,
      startTime: conflict.start_time,
      endTime: conflict.end_time,
    })),
    coverageWarnings: (raw.coverage_warnings ?? []).map((warning) => ({
      staffId: String(warning.staff_id),
      staffName: warning.staff_name,
      date: warning.date,
      startTime: warning.start_time,
      endTime: warning.end_time,
      uncoveredStartTime: warning.uncovered_start_time,
      uncoveredEndTime: warning.uncovered_end_time,
      message: warning.message,
    })),
  };
}

export function mapTemplates(raw: BackendTemplatesResponse): TemplatesResponse {
  return {
    templates: (raw.templates ?? []).map((template) => ({
      id: String(template.id),
      name: template.name,
      type: template.type,
      listKind: template.list_kind,
      categoryId: String(template.category_id),
      categoryName: template.category_name,
      roomId:
        template.room_id !== undefined && template.room_id !== null
          ? String(template.room_id)
          : undefined,
      roomName: template.room_name,
      educationGroupId:
        template.education_group_id !== undefined &&
        template.education_group_id !== null
          ? String(template.education_group_id)
          : undefined,
      educationGroupName: template.education_group_name,
      isOpen: template.is_open,
      maxParticipants: template.max_participants,
      notes: template.notes,
      shiftTypeName: template.shift_type_name,
      shiftTypeColor: template.shift_type_color,
      calendarPeriodId:
        template.calendar_period_id !== undefined &&
        template.calendar_period_id !== null
          ? String(template.calendar_period_id)
          : undefined,
      targetGroupType: template.target_group_type,
      targetGradeLevel: template.target_grade_level,
      targetSchoolClass: template.target_school_class,
      sourceCareOfferingId:
        template.source_care_offering_id !== undefined &&
        template.source_care_offering_id !== null
          ? String(template.source_care_offering_id)
          : undefined,
      sourceGradeLevels: template.source_grade_levels ?? undefined,
      enrollmentCount: template.enrollment_count,
      supervisorCount: template.supervisor_count,
      requiredStaffCount: template.required_staff_count,
      assignedStaffCount: template.assigned_staff_count,
      requiredStaffOverride: template.required_staff_override ?? undefined,
      studentIds: (template.student_ids ?? []).map(String),
      staffIds: (template.staff_ids ?? []).map(String),
      primaryStaffId:
        template.primary_staff_id !== undefined &&
        template.primary_staff_id !== null
          ? String(template.primary_staff_id)
          : undefined,
      schedules: (template.schedules ?? []).map((schedule) => ({
        id: String(schedule.id),
        weekday: schedule.weekday,
        startTime: schedule.start_time,
        endTime: schedule.end_time,
        weekPattern: schedule.week_pattern,
        calendarPeriodId:
          schedule.calendar_period_id !== undefined &&
          schedule.calendar_period_id !== null
            ? String(schedule.calendar_period_id)
            : undefined,
        validFrom: schedule.valid_from,
        validUntil: schedule.valid_until,
      })),
    })),
  };
}

/**
 * Maps the offering-source editor support payload (#2137). Grade-count keys
 * arrive as JSON object strings and become numbers (0 = ohne Jahrgang).
 */
export function mapOfferingSourceOptions(
  raw: BackendOfferingSourcesResponse,
): OfferingSourceOption[] {
  return (raw.offerings ?? []).map((offering) => {
    const gradeCounts: Record<number, number> = {};
    for (const [grade, count] of Object.entries(offering.grade_counts ?? {})) {
      const parsed = Number.parseInt(grade, 10);
      if (!Number.isNaN(parsed)) gradeCounts[parsed] = count;
    }
    return {
      id: String(offering.id),
      name: offering.name,
      phaseId: String(offering.phase_id),
      phaseName: offering.phase_name,
      totalCount: offering.total_count,
      gradeCounts,
      sourcedTemplates: (offering.sourced_templates ?? []).map((template) => ({
        id: String(template.id),
        name: template.name,
        gradeLevels: template.grade_levels ?? [],
      })),
      legacyLinkedTemplateId:
        offering.legacy_linked_template_id !== undefined &&
        offering.legacy_linked_template_id !== null
          ? String(offering.legacy_linked_template_id)
          : undefined,
    };
  });
}

/**
 * Resolves the calendar-period pin that governs a template's first schedule.
 * A schedule pin is more specific than the template-level fallback, matching
 * the backend materialization rule in schedulePinnedPeriodID.
 */
export function resolveTemplateCalendarPeriodId(
  template: TimetableTemplate,
): string | undefined {
  return template.schedules[0]?.calendarPeriodId ?? template.calendarPeriodId;
}

/**
 * Groups a flat list of instances by date. Returns a stable ordering: dates
 * are sorted, instances within a date keep their incoming order (the
 * backend already returns them sorted by start_time).
 */
export function groupInstancesByDate(
  instances: EnrichedInstance[],
): Map<string, EnrichedInstance[]> {
  const map = new Map<string, EnrichedInstance[]>();
  for (const inst of instances) {
    const list = map.get(inst.date) ?? [];
    list.push(inst);
    map.set(inst.date, list);
  }
  return map;
}

// ---------------------------------------------------------------------------
// Apple-Calendar-style grid math.
//
// The Wochenansicht renders events as absolutely-positioned blocks within
// hour rows. These helpers convert the backend "HH:MM" strings into the
// pixel coordinates the grid component reads.
// ---------------------------------------------------------------------------

const MIN_BLOCK_HEIGHT_PX = 24;

/**
 * "09:30" → 570. Returns NaN for malformed input — callers must guard.
 */
export function parseTimeToMinutes(time: string): number {
  const [h, m] = time.split(":").map(Number);
  if (
    time.split(":").length !== 2 ||
    typeof h !== "number" ||
    typeof m !== "number" ||
    Number.isNaN(h) ||
    Number.isNaN(m) ||
    h < 0 ||
    h > 23 ||
    m < 0 ||
    m > 59
  ) {
    return NaN;
  }
  return h * 60 + m;
}

/**
 * Pixel coordinates for a single event block within a day column.
 *
 * - `top` is offset from the grid's first rendered hour line. Callers should
 *   pass a dayStartHour that includes any off-hours events they want reachable.
 * - `height` is clamped to `MIN_BLOCK_HEIGHT_PX` so 5-minute events stay
 *   tappable.
 */
export function getEventBlockPosition(
  startTime: string,
  endTime: string,
  hourHeightPx: number,
  dayStartHour: number,
): { top: number; height: number } {
  const startMin = parseTimeToMinutes(startTime);
  const endMin = parseTimeToMinutes(endTime);
  const startOffsetMin = startMin - dayStartHour * 60;
  const durationMin = Math.max(endMin - startMin, 0);
  return {
    top: (startOffsetMin / 60) * hourHeightPx,
    height: Math.max((durationMin / 60) * hourHeightPx, MIN_BLOCK_HEIGHT_PX),
  };
}

/**
 * Y offset (px) for the "now" indicator line. Returns null if the current
 * wall-clock time is outside the visible window — the caller hides the
 * line. Pure-time math; no timezone gymnastics: the browser's local time
 * is the source of truth, matching the backend's Berlin-local "HH:MM".
 */
export function getCurrentTimeOffset(
  hourHeightPx: number,
  dayStartHour: number,
  dayEndHour: number,
  now: Date = new Date(),
): number | null {
  const minutesNow = now.getHours() * 60 + now.getMinutes();
  const minutesStart = dayStartHour * 60;
  const minutesEnd = dayEndHour * 60;
  if (minutesNow < minutesStart || minutesNow > minutesEnd) return null;
  return ((minutesNow - minutesStart) / 60) * hourHeightPx;
}

interface LanedInstance {
  instance: EnrichedInstance;
  lane: number; // 0-indexed lane within the overlap cluster
  laneCount: number; // total lanes in this cluster (column-width divisor)
}

/**
 * Lane assignment for overlapping events in a single day column.
 *
 * Strategy:
 * 1. Sort by startTime ascending.
 * 2. Walk events in order; maintain the current "overlap cluster" — the
 *    set of events whose time-windows transitively overlap.
 * 3. For each new event, pick the lowest free lane in the cluster (a lane
 *    is free if its previous event has already ended).
 * 4. When a new event starts after every cluster member has ended, flush
 *    the cluster and start a new one. On flush, every member gets the
 *    cluster's final lane count so they all render at the same width.
 *
 * Result: visually parallel events show side-by-side in equal slices,
 * matching Apple Calendar's behaviour. Non-overlapping events stay
 * full-width.
 */
export function assignBlockLanes(
  instances: EnrichedInstance[],
): LanedInstance[] {
  if (instances.length === 0) return [];

  const sorted = [...instances].sort(
    (a, b) => parseTimeToMinutes(a.startTime) - parseTimeToMinutes(b.startTime),
  );

  const result: LanedInstance[] = [];
  let cluster: LanedInstance[] = [];
  let clusterMaxEnd = -Infinity;

  const flushCluster = (): void => {
    if (cluster.length === 0) return;
    const lanesUsed = cluster.reduce((m, c) => Math.max(m, c.lane), 0) + 1;
    for (const c of cluster) c.laneCount = lanesUsed;
    cluster = [];
    clusterMaxEnd = -Infinity;
  };

  for (const inst of sorted) {
    const startMin = parseTimeToMinutes(inst.startTime);
    const endMin = parseTimeToMinutes(inst.endTime);

    if (startMin >= clusterMaxEnd) {
      flushCluster();
    }

    const activeLanes = new Set(
      cluster
        .filter((c) => parseTimeToMinutes(c.instance.endTime) > startMin)
        .map((c) => c.lane),
    );
    let lane = 0;
    while (activeLanes.has(lane)) lane++;

    const laned: LanedInstance = { instance: inst, lane, laneCount: 1 };
    cluster.push(laned);
    result.push(laned);
    clusterMaxEnd = Math.max(clusterMaxEnd, endMin);
  }

  flushCluster();
  return result;
}

/**
 * Betreuungsplan-Tageskopfzeile (docs/06-betreuungsplan.md Abschnitt 3.1):
 * eingeplante Personenzahl eines Tages als Vereinigung der zugeordneten,
 * nicht abwesenden Personen über alle Blöcke des Tages — eine Person zählt
 * unabhängig von der Anzahl ihrer Blöcke einmal, abgesagte Instanzen zählen
 * nicht. Erwartet bereits auf einen Kalendertag gefilterte Instanzen (z. B.
 * ein Eintrag von `groupInstancesByDate`).
 */
export function countPlannedStaff(instances: EnrichedInstance[]): number {
  const staffIds = new Set<string>();
  for (const instance of instances) {
    if (instance.status === "cancelled") continue;
    for (const member of instance.staff) {
      if (!member.isAbsent) {
        staffIds.add(member.staffId);
      }
    }
  }
  return staffIds.size;
}

/**
 * Whether any active care offering points at a Betreuungsplan-Regeltermin.
 * "unknown" means the linkage could not be read (no enrollment permission).
 */
export type TimetableCareOfferingLinkStatus = "linked" | "unlinked" | "unknown";

export interface TimetableSetupState {
  periodDone: boolean;
  enrollmentDone: boolean;
  planDone: boolean;
  /** false when the care-offering linkage is unknown (no read access) */
  enrollmentApplicable: boolean;
  completedSteps: number;
  totalSteps: number;
  progressPercent: number;
  /** Required steps (period + first plan) complete — collapses the guide. */
  setupComplete: boolean;
}

/**
 * Status of the three onboarding steps shown in the planner setup guide
 * (Planungszeitraum / Anmeldung verknüpfen / Erste Woche planen). The
 * enrollment step counts as done only when a care offering actually links to
 * a Regeltermin — an active enrollment phase alone proves no linkage. It is
 * optional and is dropped from the progress count when the linkage is unknown
 * (the admin cannot read enrollment data).
 */
export function computeTimetableSetup(input: {
  hasActivePeriod: boolean;
  careOfferingLink: TimetableCareOfferingLinkStatus;
  hasPlan: boolean;
}): TimetableSetupState {
  const periodDone = input.hasActivePeriod;
  const planDone = input.hasPlan;
  const enrollmentApplicable = input.careOfferingLink !== "unknown";
  const enrollmentDone = input.careOfferingLink === "linked";

  const totalSteps = enrollmentApplicable ? 3 : 2;
  const completedSteps =
    (periodDone ? 1 : 0) +
    (planDone ? 1 : 0) +
    (enrollmentApplicable && enrollmentDone ? 1 : 0);
  const progressPercent = Math.round((completedSteps / totalSteps) * 100);

  return {
    periodDone,
    enrollmentDone,
    planDone,
    enrollmentApplicable,
    completedSteps,
    totalSteps,
    progressPercent,
    setupComplete: periodDone && planDone,
  };
}

/**
 * Ansichts-Typ und Dichte-Konstanten des Betreuungsplans. Wohnten früher in
 * `components/timetable/timetable-toolbar.tsx`; die Toolbar wurde mit dem
 * Chrome-Abbau (Planung-Redesign Inkrement 4, Chunk 8) entfernt, deshalb
 * leben die noch gebrauchten Typen/Konstanten hier im Helper-Modul.
 */
export type TimetableView = "week" | "month" | "series";

/**
 * Drei diskrete Zoomstufen des Wochenrasters. Die Pixel-pro-Stunde-Werte
 * werden daraus abgeleitet — nie rohe Pixel in der UI zeigen (semantische
 * Labels nach Apple/Google/Outlook-Konvention: Zoom nach Absicht, nicht
 * Betrag).
 */
export type WeekDensity = "compact" | "normal" | "comfortable";

export const DENSITY_TO_HOUR_HEIGHT_PX: Record<WeekDensity, number> = {
  compact: 60,
  normal: 90,
  comfortable: 120,
};
