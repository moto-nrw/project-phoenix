import { fetchStudents } from "~/lib/student-api";
import { resolveTemplateCalendarPeriodId } from "~/lib/timetable-helpers";
import type {
  ActivityType,
  EnrichedInstance,
  TargetGroupType,
  TemplateProtectedStudentAssignment,
  TimetableListKind,
  TimetableTemplate,
} from "~/lib/timetable-types";

const STUDENT_PAGE_SIZE = 500;

export type RepeatMode = "none" | "weekly" | "biweekly";

/** One weekday's roster inside the per-weekday editing mode (#2129). */
export interface WeekdayRosterState {
  staffIds: string[];
  primaryStaffId: string;
  studentIds: string[];
}

/** German weekday labels for the per-weekday roster tabs, ISO-indexed. */
export const WEEKDAY_LABELS: Record<number, string> = {
  1: "Montag",
  2: "Dienstag",
  3: "Mittwoch",
  4: "Donnerstag",
  5: "Freitag",
  6: "Samstag",
  7: "Sonntag",
};

export const WEEKDAY_SHORT_LABELS: Record<number, string> = {
  1: "Mo",
  2: "Di",
  3: "Mi",
  4: "Do",
  5: "Fr",
  6: "Sa",
  7: "So",
};

/** The roster that applies on `weekday`, honouring the per-weekday mode. */
export function rosterForWeekday(
  form: EventFormState,
  weekday: number,
): WeekdayRosterState {
  if (!form.perWeekdayRoster) return sharedRoster(form);
  return form.weekdayRosters[weekday] ?? sharedRoster(form);
}

export function sharedRoster(form: EventFormState): WeekdayRosterState {
  return {
    staffIds: form.staffIds,
    primaryStaffId: form.primaryStaffId,
    studentIds: form.studentIds,
  };
}

/**
 * Seeds every weekday of the series, keeping any weekday that already has a
 * roster. When per-weekday editing is already active, a newly added weekday
 * copies the first existing scheduled day instead of the top-level aggregate,
 * which may be the union of several different weekday rosters.
 */
export function seedWeekdayRosters(
  form: EventFormState,
  weekdays: number[],
): Record<number, WeekdayRosterState> {
  const shared = sharedRoster(form);
  const existing = form.weekdays
    .map((weekday) => form.weekdayRosters[weekday])
    .find((roster): roster is WeekdayRosterState => roster !== undefined);
  const seed = form.perWeekdayRoster && existing ? existing : shared;
  const seeded: Record<number, WeekdayRosterState> = {};
  for (const weekday of weekdays) {
    const existingRoster = form.weekdayRosters[weekday];
    const roster = existingRoster ?? rosterSeedForWeekday(form, seed, weekday);
    seeded[weekday] = {
      staffIds: [...roster.staffIds],
      primaryStaffId: roster.primaryStaffId,
      studentIds: [...roster.studentIds],
    };
  }
  return seeded;
}

/**
 * Switches between the shared and per-weekday roster representations.
 * Enabling always rebuilds every day from the current shared controls, so a
 * previous per-weekday edit cannot reappear after a shared-mode change.
 * Enrollment-owned children are then restored only on their covered days;
 * their selected weekdays are not editable through this form.
 */
export function changePerWeekdayRosterMode(
  form: EventFormState,
  enabled: boolean,
  activeWeekday: number,
): EventFormState {
  if (!enabled) {
    const source = rosterForWeekday(
      form,
      form.weekdays.includes(activeWeekday)
        ? activeWeekday
        : (form.weekdays[0] ?? activeWeekday),
    );
    return {
      ...form,
      staffIds: [...source.staffIds],
      primaryStaffId: source.primaryStaffId,
      studentIds: [...source.studentIds],
      perWeekdayRoster: false,
      weekdayRosters: {},
    };
  }

  const protectedIDs = new Set(
    form.protectedStudentAssignments.flatMap(
      (assignment) => assignment.studentIds,
    ),
  );
  const protectedByWeekday = new Map(
    form.protectedStudentAssignments.map((assignment) => [
      assignment.weekday,
      assignment.studentIds,
    ]),
  );
  const weekdayRosters: Record<number, WeekdayRosterState> = {};
  for (const weekday of form.weekdays) {
    const covered = protectedByWeekday.get(weekday) ?? [];
    const coveredIDs = new Set(covered);
    const studentIds = form.studentIds.filter(
      (id) => !protectedIDs.has(id) || coveredIDs.has(id),
    );
    const included = new Set(studentIds);
    for (const id of covered) {
      if (!included.has(id)) {
        studentIds.push(id);
        included.add(id);
      }
    }
    weekdayRosters[weekday] = {
      staffIds: [...form.staffIds],
      primaryStaffId: form.primaryStaffId,
      studentIds,
    };
  }
  return { ...form, perWeekdayRoster: true, weekdayRosters };
}

/**
 * Replaces the recurrence weekdays without losing the roster state rules.
 * New days copy one concrete existing day, never the aggregate read model;
 * collapsing to one day turns the surviving concrete roster into the shared
 * controls that are serialized by the single-day flow.
 */
export function changeRosterWeekdays(
  form: EventFormState,
  weekdays: number[],
  fallbackWeekday: number,
): EventFormState {
  const weekdayRosters = form.perWeekdayRoster
    ? seedWeekdayRosters(form, weekdays)
    : form.weekdayRosters;
  if (form.perWeekdayRoster && weekdays.length < 2) {
    const source =
      weekdayRosters[weekdays[0] ?? fallbackWeekday] ??
      rosterForWeekday(form, weekdays[0] ?? fallbackWeekday);
    return {
      ...form,
      weekdays,
      staffIds: [...source.staffIds],
      primaryStaffId: source.primaryStaffId,
      studentIds: [...source.studentIds],
      perWeekdayRoster: false,
      weekdayRosters,
    };
  }
  return { ...form, weekdays, weekdayRosters };
}

/** Copies a roster to a newly introduced day without widening protected rows. */
export function rosterSeedForWeekday(
  form: EventFormState,
  roster: WeekdayRosterState,
  weekday: number,
): WeekdayRosterState {
  const protectedIDs = new Set(
    form.protectedStudentAssignments.flatMap(
      (assignment) => assignment.studentIds,
    ),
  );
  const coveredIDs = new Set(
    form.protectedStudentAssignments.find(
      (assignment) => assignment.weekday === weekday,
    )?.studentIds ?? [],
  );
  return {
    staffIds: [...roster.staffIds],
    primaryStaffId: roster.primaryStaffId,
    studentIds: roster.studentIds.filter(
      (id) => !protectedIDs.has(id) || coveredIDs.has(id),
    ),
  };
}

/**
 * True when `weekday` staffs differently from the series baseline — the signal
 * behind the "abweichend" badge. The baseline is the first weekday's roster:
 * with no stored notion of "the shared default" (the writer expands it away),
 * deviation is a comparison between days, which is also what a planner means
 * by it.
 */
export function weekdayDeviatesFromBaseline(
  form: EventFormState,
  weekday: number,
  weekdays: number[],
): boolean {
  if (!form.perWeekdayRoster) return false;
  const baselineWeekday = weekdays[0];
  if (baselineWeekday === undefined || baselineWeekday === weekday) {
    return false;
  }
  const baseline = rosterForWeekday(form, baselineWeekday);
  const current = rosterForWeekday(form, weekday);
  return (
    !sameIdSet(baseline.staffIds, current.staffIds) ||
    !sameIdSet(baseline.studentIds, current.studentIds) ||
    baseline.primaryStaffId !== current.primaryStaffId
  );
}

function sameIdSet(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false;
  const seen = new Set(left);
  return right.every((id) => seen.has(id));
}

/**
 * True when at least one weekday staffs differently from the others. This is
 * the state an offering source destroys: with a source set, the payload
 * carries only the shared staff list (the backend rejects
 * `weekday_assignments` next to a source), so activating one must ask for
 * explicit confirmation first (#2147 review). Confirming then empties the
 * shared staffing instead of adopting an aggregated all-weekdays union, so
 * the replacement Besetzung is picked explicitly (#2147 review round 13).
 * Child lists are not compared; a sourced roster replaces them by design.
 */
export function hasPerWeekdayStaffDeviation(form: EventFormState): boolean {
  if (!form.perWeekdayRoster || form.weekdays.length < 2) return false;
  const [first, ...rest] = form.weekdays;
  if (first === undefined) return false;
  const baseline = rosterForWeekday(form, first);
  return rest.some((weekday) => {
    const current = rosterForWeekday(form, weekday);
    return (
      !sameIdSet(baseline.staffIds, current.staffIds) ||
      baseline.primaryStaffId !== current.primaryStaffId
    );
  });
}

export interface EventFormState {
  title: string;
  date: string;
  /**
   * Serienbeginn of a stored, not-yet-started series (#2226). Prefilled with
   * the schedules' validFrom when editing such a series; the user may move it
   * to an earlier date (never later, never into the past). "" everywhere
   * else — creates and instance edits use `date` instead.
   */
  seriesStartDate: string;
  startTime: string;
  endTime: string;
  roomId: string;
  type: ActivityType;
  categoryId: string;
  planningTrackId: string;
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
   * Per-weekday staffing (#2129). When false the series uses ONE roster on
   * every weekday and `weekdayRosters` is ignored. When true each weekday of
   * the series carries its own staff, zuständige Person and child list, and
   * `studentIds`/`staffIds`/`primaryStaffId` above seed the mode initially;
   * weekdays added later copy an existing scheduled day's roster.
   */
  perWeekdayRoster: boolean;
  /** Roster per ISO weekday (1 = Mo … 5 = Fr); only read when perWeekdayRoster. */
  weekdayRosters: Record<number, WeekdayRosterState>;
  /** Enrollment-owned children and the weekdays on which they are immutable. */
  protectedStudentAssignments: TemplateProtectedStudentAssignment[];
  /**
   * Manual Personalbedarf override (issue #1839). Empty = automatic: a single
   * occurrence inherits the series value, otherwise the Betreuungsschlüssel
   * derivation applies. A non-negative integer overrides both (on a single
   * occurrence it becomes a pin that survives series edits). Held as a string
   * because it is an <input> value; parsed via parseRequiredStaffOverride.
   */
  requiredStaff: string;
  /**
   * Maximale Teilnehmerzahl of the series (#2233), stored on
   * activities.groups.max_participants. Empty = unbegrenzt (NULL, #2236).
   * Held as a string because it is an <input> value; parsed via
   * parseMaxParticipants. Series flows only — a single occurrence has no
   * capacity of its own, so occurrence-scope edits keep echoing the stored
   * series value.
   */
  maxParticipants: string;
  /**
   * Zielgruppe (target group, issue #1838). "gruppe" reuses educationGroupId
   * above as its value rather than a separate field — switching away from
   * "gruppe" clears educationGroupId so the two never disagree.
   */
  targetGroupType: TargetGroupType;
  targetGradeLevel: string; // "" | configured grade, only meaningful for "jahrgang"
  targetSchoolClass: string; // "", only meaningful for "klasse"
  targetGradeLevels: string[];
  targetSchoolClasses: string[];
  educationGroupIds: string[];
  /**
   * Offering-source rule (#2137), only meaningful for "angebot": the
   * Betreuungsangebot whose approved children feed this Regeltermin, plus the
   * Jahrgang filter (empty = alle Kinder des Angebots). With a source set the
   * manual student picker is hidden and studentIds stays empty — the roster
   * is server-managed.
   */
  sourceCareOfferingIds: string[];
  sourceGradeLevels: number[];
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
    seriesStartDate: "",
    startTime: defaultStartTime,
    endTime: defaultEndTime,
    roomId: "",
    type: "care",
    categoryId: "",
    planningTrackId: "",
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
    perWeekdayRoster: false,
    weekdayRosters: {},
    protectedStudentAssignments: [],
    requiredStaff: "",
    maxParticipants: "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
    targetGradeLevels: [],
    targetSchoolClasses: [],
    educationGroupIds: [],
    sourceCareOfferingIds: [],
    sourceGradeLevels: [],
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
    seriesStartDate: "",
    startTime: instance.startTime,
    endTime: instance.endTime,
    roomId: instance.roomId,
    type: instance.activityType,
    categoryId: "",
    planningTrackId: instance.planningTrackId ?? "",
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
    // A single occurrence has exactly one date, so per-weekday staffing is a
    // series-only concept here.
    perWeekdayRoster: false,
    weekdayRosters: {},
    protectedStudentAssignments: [],
    requiredStaff:
      instance.requiredStaffOverride !== undefined
        ? String(instance.requiredStaffOverride)
        : "",
    // Capacity lives on the series, not the occurrence; the occurrence editor
    // never shows or writes it (#2233).
    maxParticipants: "",
    targetGroupType: "none",
    targetGradeLevel: "",
    targetSchoolClass: "",
    targetGradeLevels: [],
    targetSchoolClasses: [],
    educationGroupIds: [],
    sourceCareOfferingIds: [],
    sourceGradeLevels: [],
  };
}

export function formFromSeries(
  series: TimetableTemplate,
  defaultDate: string,
  defaultCalendarPeriodId?: string | null,
): EventFormState {
  const targets = series.targets ?? [];
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
    seriesStartDate: firstSchedule?.validFrom ?? "",
    startTime: firstSchedule?.startTime ?? "12:00",
    endTime: firstSchedule?.endTime ?? "13:00",
    roomId: series.roomId ?? "",
    type: series.type,
    categoryId: series.categoryId,
    planningTrackId: series.planningTrackId ?? "",
    listKind: series.listKind ?? "",
    educationGroupId:
      series.targetGroupType === "gruppe"
        ? ""
        : (series.educationGroupId ?? ""),
    notes: "",
    seriesNotes: series.notes ?? "",
    repeat,
    weekPattern,
    weekdays: weekdays.length > 0 ? weekdays : [1],
    calendarPeriodId:
      resolveTemplateCalendarPeriodId(series) ?? defaultCalendarPeriodId ?? "",
    // A sourced roster is server-managed: the backend rejects student_ids
    // next to a source, and the list's studentIds are exactly the sourced
    // rows — prefilling them would turn the rule into a snapshot.
    studentIds:
      (series.sourceCareOfferingIds?.length ?? 0) > 0 ? [] : series.studentIds,
    staffIds: series.staffIds,
    primaryStaffId: series.primaryStaffId ?? "",
    // A series that carries weekday assignments was saved in per-weekday mode
    // and must reopen in it, or saving again would silently flatten it back to
    // one shared roster.
    perWeekdayRoster: series.weekdayAssignments.length > 0,
    weekdayRosters: weekdayRostersFromSeries(series),
    protectedStudentAssignments: series.protectedStudentAssignments ?? [],
    requiredStaff:
      series.requiredStaffOverride !== undefined
        ? String(series.requiredStaffOverride)
        : "",
    maxParticipants:
      series.maxParticipants !== null ? String(series.maxParticipants) : "",
    targetGroupType: series.targetGroupType,
    targetGradeLevel:
      series.targetGradeLevel !== undefined && series.targetGradeLevel !== null
        ? String(series.targetGradeLevel)
        : "",
    targetSchoolClass: series.targetSchoolClass?.trim() ?? "",
    sourceCareOfferingIds: series.sourceCareOfferingIds ?? [],
    sourceGradeLevels: series.sourceGradeLevels ?? [],
    targetGradeLevels:
      targets.length > 0
        ? targets.flatMap((target) =>
            target.gradeLevel === undefined ? [] : [String(target.gradeLevel)],
          )
        : series.targetGradeLevel === undefined
          ? []
          : [String(series.targetGradeLevel)],
    targetSchoolClasses:
      targets.length > 0
        ? targets.flatMap((target) =>
            target.schoolClass ? [target.schoolClass] : [],
          )
        : series.targetSchoolClass
          ? [series.targetSchoolClass.trim()]
          : [],
    educationGroupIds:
      targets.length > 0
        ? targets.flatMap((target) =>
            target.educationGroupId ? [target.educationGroupId] : [],
          )
        : series.targetGroupType === "gruppe" && series.educationGroupId
          ? [series.educationGroupId]
          : [],
  };
}

function weekdayRostersFromSeries(
  series: TimetableTemplate,
): Record<number, WeekdayRosterState> {
  const rosters: Record<number, WeekdayRosterState> = {};
  for (const assignment of series.weekdayAssignments) {
    rosters[assignment.weekday] = {
      staffIds: assignment.staffIds,
      primaryStaffId: assignment.primaryStaffId ?? "",
      studentIds: assignment.studentIds,
    };
  }
  return rosters;
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

// parseMaxParticipants maps the "Maximale Teilnehmerzahl" input to the wire
// value (#2233): empty → null (unbegrenzt, stored as NULL since #2236); a
// whole positive integer → the capacity. Only plain digit strings are
// accepted, mirroring parseRequiredStaffOverride. "0" and junk both parse to
// null here — validateForm rejects them before any save runs, so null never
// silently clears a limit the user meant to keep.
export function parseMaxParticipants(value: string): number | null {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) return null;
  const parsed = Number(trimmed);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null;
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
  if (missingMemberCount === 0) return `Alle Kinder aus ${label} übernommen`;
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
