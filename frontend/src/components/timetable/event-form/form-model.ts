import { fetchStudents } from "~/lib/student-api";
import { resolveTemplateCalendarPeriodId } from "~/lib/timetable-helpers";
import type {
  ActivityType,
  EnrichedInstance,
  TargetGroupType,
  TimetableListKind,
  TimetableTemplate,
} from "~/lib/timetable-types";

const STUDENT_PAGE_SIZE = 500;

export type RepeatMode = "none" | "weekly" | "biweekly";

export interface EventFormState {
  title: string;
  date: string;
  startTime: string;
  endTime: string;
  roomId: string;
  type: ActivityType;
  categoryId: string;
  /**
   * Listenart (#1565) — classifies the slot for printable daily lists
   * (Randstunden, Lernzeit, AG-Angebote, Mensa). "" = no list kind.
   */
  listKind: "" | TimetableListKind;
  educationGroupId: string;
  /**
   * Tagesnotiz — the one-off note on a single occurrence
   * (schedule.activity_instances.notes). Editable only in the single-instance
   * scope; series-wide scopes never write it.
   */
  notes: string;
  /**
   * Wochennotiz — the durable series note on the Regeltermin
   * (activities.groups.notes, #1837 follow-up). Shows on every occurrence and
   * survives Re-Plan/Split. Editable in the series-create/edit flow; shown
   * read-only on a single occurrence that belongs to a series.
   */
  seriesNotes: string;
  repeat: RepeatMode;
  weekPattern: 0 | 1 | 2;
  weekdays: number[];
  calendarPeriodId: string;
  studentIds: string[];
  staffIds: string[];
  primaryStaffId: string;
  /**
   * Manual Personalbedarf override (issue #1839). Empty = automatic: a single
   * occurrence inherits the series value, otherwise the Betreuungsschlüssel
   * derivation applies. A non-negative integer overrides both (on a single
   * occurrence it becomes a pin that survives series edits). Held as a string
   * because it is an <input> value; parsed via parseRequiredStaffOverride.
   */
  requiredStaff: string;
  /**
   * Zielgruppe (target group, issue #1838). "gruppe" reuses educationGroupId
   * above as its value rather than a separate field — switching away from
   * "gruppe" clears educationGroupId so the two never disagree.
   */
  targetGroupType: TargetGroupType;
  targetGradeLevel: string; // "" | configured grade, only meaningful for "jahrgang"
  targetSchoolClass: string; // "", only meaningful for "klasse"
}

export interface PersonOption {
  id: string;
  name: string;
  schoolClass?: string;
  groupId?: string;
  groupName?: string;
}

export function isoWeekday(dateISO: string): number {
  const d = new Date(`${dateISO}T00:00:00`);
  const day = d.getDay();
  if (day === 0) return 7;
  return day;
}

export function emptyForm(
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
  defaultRepeat: RepeatMode = "none",
  defaultStartTime = "12:00",
  defaultEndTime = "13:00",
): EventFormState {
  const weekday = isoWeekday(defaultDate);
  return {
    title: "",
    date: defaultDate,
    startTime: defaultStartTime,
    endTime: defaultEndTime,
    roomId: "",
    type: "care",
    categoryId: "",
    listKind: "",
    educationGroupId: "",
    notes: "",
    seriesNotes: "",
    repeat: defaultRepeat,
    weekPattern: defaultRepeat === "biweekly" ? 2 : 0,
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: [],
    staffIds: [],
    primaryStaffId: "",
    requiredStaff: "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
  };
}

export function formFromInstance(
  instance: EnrichedInstance,
  defaultCalendarPeriodId?: string | null,
  repeat: RepeatMode = "none",
): EventFormState {
  const weekday = isoWeekday(instance.date);
  return {
    title: instance.title,
    date: instance.date,
    startTime: instance.startTime,
    endTime: instance.endTime,
    roomId: instance.roomId,
    type: instance.activityType,
    categoryId: "",
    listKind: instance.listKind ?? "",
    educationGroupId: "",
    notes: instance.notes ?? "",
    // Read-only on a single occurrence: the series note is edited via the
    // Regeltermin, not here. Prefilled so it can be shown as a fixed hint.
    seriesNotes: instance.seriesNotes ?? "",
    repeat,
    weekPattern: repeat === "biweekly" ? 2 : 0,
    weekdays: weekday >= 1 && weekday <= 5 ? [weekday] : [1],
    calendarPeriodId: defaultCalendarPeriodId ?? "",
    studentIds: instance.studentIds,
    staffIds: instance.staff.map((item) => item.staffId),
    primaryStaffId:
      instance.staff.find((item) => item.isPrimary)?.staffId ?? "",
    requiredStaff:
      instance.requiredStaffOverride !== undefined
        ? String(instance.requiredStaffOverride)
        : "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
  };
}

export function formFromSeries(
  series: TimetableTemplate,
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
): EventFormState {
  const firstSchedule = series.schedules[0];
  const weekdays = series.schedules.map((schedule) => schedule.weekday);
  const weekPattern =
    firstSchedule?.weekPattern === 1 || firstSchedule?.weekPattern === 2
      ? firstSchedule.weekPattern
      : 0;
  // Both A (1) and B (2) are biweekly series. Mapping A to "weekly" here used
  // to hide the A-ness and silently reset it on the next repeat-tab touch.
  const repeat: RepeatMode = weekPattern === 0 ? "weekly" : "biweekly";
  return {
    title: series.name,
    date: defaultDate,
    startTime: firstSchedule?.startTime ?? "12:00",
    endTime: firstSchedule?.endTime ?? "13:00",
    roomId: series.roomId ?? "",
    type: series.type,
    categoryId: series.categoryId,
    listKind: series.listKind ?? "",
    educationGroupId: series.educationGroupId ?? "",
    notes: "",
    seriesNotes: series.notes ?? "",
    repeat,
    weekPattern,
    weekdays: weekdays.length > 0 ? weekdays : [1],
    calendarPeriodId:
      resolveTemplateCalendarPeriodId(series) ?? defaultCalendarPeriodId ?? "",
    studentIds: series.studentIds,
    staffIds: series.staffIds,
    primaryStaffId: series.primaryStaffId ?? "",
    requiredStaff:
      series.requiredStaffOverride !== undefined
        ? String(series.requiredStaffOverride)
        : "",
    targetGroupType: series.targetGroupType,
    targetGradeLevel:
      series.targetGradeLevel !== undefined && series.targetGradeLevel !== null
        ? String(series.targetGradeLevel)
        : "",
    targetSchoolClass: series.targetSchoolClass?.trim() ?? "",
  };
}

// parseRequiredStaffOverride maps the "Benötigtes Personal" input to the
// override wire value (#1839): empty/invalid → null (clear the override, derive
// from the Betreuungsschlüssel); a whole non-negative integer → manual
// override. Only plain digit strings are accepted — Number.parseInt would
// silently coerce "1e2" → 1 or "2.5" → 2 and persist the wrong requirement.
export function parseRequiredStaffOverride(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number(trimmed);
  return Number.isSafeInteger(parsed) ? parsed : null;
}

export function sortPeople<T extends PersonOption>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const classCompare = (a.schoolClass ?? "").localeCompare(
      b.schoolClass ?? "",
      "de",
    );
    if (classCompare !== 0) return classCompare;
    const groupCompare = (a.groupName ?? "").localeCompare(
      b.groupName ?? "",
      "de",
    );
    if (groupCompare !== 0) return groupCompare;
    return a.name.localeCompare(b.name, "de");
  });
}

export function schoolClassLabel(schoolClass: string): string {
  const trimmed = schoolClass.trim();
  return /^klasse(?:\s|$)/i.test(trimmed) ? trimmed : `Klasse ${trimmed}`;
}

export function targetCohortActionLabel(
  label: string,
  memberCount: number,
  missingMemberCount: number,
): string {
  if (memberCount === 0) return `Keine Kinder aus ${label} gefunden`;
  if (missingMemberCount === 0) {
    return `Alle Kinder aus ${label} übernommen`;
  }
  const childLabel = memberCount === 1 ? "Kind" : "Kinder";
  return `Alle ${memberCount} ${childLabel} aus ${label} übernehmen`;
}

export function initialStudentIDs(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string[] {
  if (initialSeries) return initialSeries.studentIds;
  if (convertInstance) return convertInstance.studentIds;
  if (initialInstance) return initialInstance.studentIds;
  return [];
}

export function initialStaffIDs(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string[] {
  if (initialSeries) return initialSeries.staffIds;
  const instance = convertInstance ?? initialInstance;
  return instance?.staff.map((item) => item.staffId) ?? [];
}

export function initialPrimaryStaffID(
  initialInstance: EnrichedInstance | null,
  initialSeries: TimetableTemplate | null,
  convertInstance: EnrichedInstance | null,
): string {
  if (initialSeries) return initialSeries.primaryStaffId ?? "";
  const instance = convertInstance ?? initialInstance;
  return instance?.staff.find((item) => item.isPrimary)?.staffId ?? "";
}

export async function fetchAllStudentOptions(): Promise<PersonOption[]> {
  const firstPage = await fetchStudents({
    page: 1,
    page_size: STUDENT_PAGE_SIZE,
  });
  const totalPages = Math.max(1, firstPage.pagination?.total_pages ?? 1);
  const remainingPages =
    totalPages > 1
      ? await Promise.all(
          Array.from({ length: totalPages - 1 }, (_, index) =>
            fetchStudents({
              page: index + 2,
              page_size: STUDENT_PAGE_SIZE,
            }),
          ),
        )
      : [];

  const byID = new Map<string, PersonOption>();
  for (const page of [firstPage, ...remainingPages]) {
    for (const student of page.students) {
      byID.set(student.id, {
        id: student.id,
        name: student.name,
        schoolClass: student.school_class,
        groupId: student.group_id,
        groupName: student.group_name,
      });
    }
  }
  return [...byID.values()];
}
