/**
 * Helpers for the timetable feature: backend ↔ frontend mapping, week
 * date math, and brand-colour resolution.
 *
 * All week math is anchored to Monday (OGS-Schultage Mo–Fr), uses Berlin
 * local time, and emits YYYY-MM-DD strings ready for the backend
 * /api/timetable/instances endpoint.
 */

import { LOCATION_COLORS } from "./location-helper";
import type {
  ActivityType,
  BackendCreateTemplateResult,
  BackendEnrichedInstance,
  BackendInstanceStatusResult,
  BackendMaterializeResult,
  BackendTemplatesResponse,
  BackendStartInstanceResult,
  BackendWeeklyInstancesResponse,
  CreateTemplateResult,
  EnrichedInstance,
  InstanceStaffSummary,
  InstanceStatusResult,
  MaterializeResult,
  StartInstanceResult,
  TemplatesResponse,
  WeeklyInstancesResponse,
} from "./timetable-types";

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
 * Strong text colour matching the brand-colour bar — used on titles inside
 * coloured cards.
 */
export function getActivityTextColor(type: ActivityType): string {
  switch (type) {
    case "care":
      return "#1E3A8A";
    case "activity":
      return "#365314";
    case "external":
      return "#7C2D12";
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
export function getMondayOfWeek(ref: Date, weekOffset = 0): Date {
  const monday = new Date(ref);
  const day = monday.getDay(); // 0 = Sunday, 1 = Monday, …
  // Sunday (0) needs to walk back 6 days; otherwise day - 1.
  const diff = day === 0 ? -6 : 1 - day;
  monday.setDate(monday.getDate() + diff + weekOffset * 7);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

/**
 * Returns Monday and Friday (00:00 local) for the requested week. The
 * backend /instances endpoint accepts an inclusive range, so Friday is the
 * "to" boundary — Saturday and Sunday are not OGS days.
 */
export function getWeekRange(
  ref: Date,
  weekOffset = 0,
): { from: Date; to: Date } {
  const from = getMondayOfWeek(ref, weekOffset);
  const to = new Date(from);
  to.setDate(to.getDate() + 4); // Mo + 4 = Fr
  return { from, to };
}

/**
 * Formats a Date as YYYY-MM-DD using local-time fields. Avoids
 * `toISOString().slice(0,10)` which round-trips through UTC and rolls back
 * a day for late-evening Berlin times in winter.
 */
export function toISODate(d: Date): string {
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

/**
 * Returns the ISO 8601 week number (1–53) of the given Date. Used for the
 * "KW 38" label in the week header.
 *
 * Reference algorithm: ISO 8601 defines the week containing the year's first
 * Thursday as week 1.
 */
export function getISOWeekNumber(d: Date): number {
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

export function getGermanWeekdayShort(d: Date): string {
  return GERMAN_WEEKDAY_SHORT[d.getDay()] ?? "";
}

/**
 * "KW 38 • Mo 22.09 – Fr 26.09.2026" — the week-navigator header.
 */
export function formatWeekLabel(from: Date, to: Date): string {
  const kw = getISOWeekNumber(from);
  const fromDay = String(from.getDate()).padStart(2, "0");
  const fromMonth = String(from.getMonth() + 1).padStart(2, "0");
  const toDay = String(to.getDate()).padStart(2, "0");
  const toMonth = String(to.getMonth() + 1).padStart(2, "0");
  const year = to.getFullYear();
  return `KW ${kw} • Mo ${fromDay}.${fromMonth} – Fr ${toDay}.${toMonth}.${year}`;
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
 * Five Monday→Friday Date objects for the week containing `from`. Used to
 * label the grid columns even on weeks with no instances.
 */
export function getWeekdays(from: Date): Date[] {
  const days: Date[] = [];
  for (let i = 0; i < 5; i++) {
    const d = new Date(from);
    d.setDate(d.getDate() + i);
    days.push(d);
  }
  return days;
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
  }));

  return {
    id: String(raw.id),
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    title: raw.title,
    description: raw.description,
    notes: raw.notes,
    status: raw.status,
    isSpontaneous: raw.is_spontaneous,
    isLive: raw.is_live,
    activityGroupId:
      raw.activity_group_id !== undefined && raw.activity_group_id !== null
        ? String(raw.activity_group_id)
        : undefined,
    activityType: raw.activity_type,
    roomId: String(raw.room_id),
    roomName: raw.room_name,
    staff,
    studentIds: (raw.student_ids ?? []).map(String),
    staffCount: raw.staff_count,
    absentStaffCount: raw.absent_staff_count,
    expectedStudentsCount: raw.expected_students_count,
    presentStudentsCount: raw.present_students_count,
    // Backend GET /instances does not yet embed conflict warnings.
    // Always return [] so consumers can iterate without nil guards.
    conflictWarnings: [],
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

export function mapTemplates(raw: BackendTemplatesResponse): TemplatesResponse {
  return {
    templates: (raw.templates ?? []).map((template) => ({
      id: String(template.id),
      name: template.name,
      type: template.type,
      categoryId: String(template.category_id),
      categoryName: template.category_name,
      roomId:
        template.room_id !== undefined && template.room_id !== null
          ? String(template.room_id)
          : undefined,
      roomName: template.room_name,
      isOpen: template.is_open,
      maxParticipants: template.max_participants,
      enrollmentCount: template.enrollment_count,
      supervisorCount: template.supervisor_count,
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
      })),
    })),
  };
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
