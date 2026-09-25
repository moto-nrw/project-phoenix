import { isNotCheckedInLocation, LOCATION_COLORS } from "./location-helper";

const APPROACHING_THRESHOLD_MINUTES = 30;

type TimeStatusState =
  | "none"
  | "planned"
  | "approaching"
  | "slightly-overdue"
  | "very-overdue"
  | "done-on-time"
  | "done-late"
  | "absent-excused"
  | "awaiting-arrival"
  | "only-if-lesson-cancelled";

type TimeStatusIcon = "clock" | "warning" | "check";

export interface StudentTimeStatus {
  state: TimeStatusState;
  displayTime?: string;
  icon: TimeStatusIcon;
  iconColor: string;
  textColor?: string;
  isResolved: boolean;
  detailAnnotation?: string;
  diffMinutes?: number;
}

/**
 * Both ends of one child's day as far as a surface knows them. The arrival and
 * pickup rows each need the other end for the rules of #3373.
 */
export interface StudentDayTimes {
  plannedArrival?: string | null;
  actualArrival?: string | null;
  plannedPickup?: string | null;
  actualPickup?: string | null;
  /**
   * The child is here right now by its live location. Counts like a recorded
   * check-in, so a missing arrival time never hides a due pickup.
   */
  checkedIn?: boolean;
}

/**
 * Reads the day from the flat student shape most surfaces carry. For a date
 * other than today the live location says nothing about that day.
 */
export function getStudentDayTimes(
  student: {
    arrival_time?: string | null;
    actual_arrival_time?: string | null;
    pickup_time?: string | null;
    actual_pickup_time?: string | null;
    current_location?: string | null;
  },
  { ignoreCurrentAttendance = false } = {},
): StudentDayTimes {
  return {
    plannedArrival: student.arrival_time,
    actualArrival: student.actual_arrival_time,
    plannedPickup: student.pickup_time,
    actualPickup: student.actual_pickup_time,
    checkedIn:
      !ignoreCurrentAttendance &&
      !isNotCheckedInLocation(student.current_location),
  };
}

/** A recorded check-in or check-out, or the child is here right now. */
function hasArrived(day: StudentDayTimes): boolean {
  return Boolean(day.actualArrival || day.actualPickup || day.checkedIn);
}

interface StudentTimeStatusInput {
  plannedTime?: string;
  actualTime?: string;
  now: Date;
  /**
   * Which end of the day this row shows. Together with `day` it enables the
   * whole-day rules (#3373); without both, the row is judged on its own time.
   */
  kind?: "arrival" | "pickup";
  day?: StudentDayTimes;
  /** Student is reported sick today — suppresses overdue urgency. */
  sick?: boolean;
  /** Student is excused today (not attending) — suppresses overdue urgency. */
  excused?: boolean;
  /** Student is on a class trip today — suppresses overdue urgency. */
  classTrip?: boolean;
}

/** Why a student is not expected today. Drives the neutral "absent" rendering. */
export interface StudentAbsence {
  reason: "sick" | "class_trip" | "excused";
  /** German UI label, e.g. "krank gemeldet" or "entschuldigt". */
  label: string;
}

interface StudentAbsenceInput {
  sick?: boolean;
  classTrip?: boolean;
  excused?: boolean;
}

/**
 * Determines whether a student is absent today and why. Single source of truth
 * for the sick/excused rule, consumed by every surface (group cards, search,
 * active supervisions, student detail) and the sort ranking so they can never
 * disagree. Sick takes priority over excused.
 *
 * The label is intentionally date-free: a "seit <weekday>" qualifier reads
 * wrong once an absence spans more than a week. We only state the status.
 *
 * Note: resolved actual times may still win at the row level. Callers that
 * collapse a whole day into one absence line should only do so while pickup is
 * unresolved, otherwise an already-arrived sick/excused child can still fall
 * through to an overdue pickup state.
 */
export function getStudentAbsence({
  sick,
  classTrip,
  excused,
}: StudentAbsenceInput): StudentAbsence | null {
  if (sick) {
    return { reason: "sick", label: "krank gemeldet" };
  }

  if (classTrip) {
    return { reason: "class_trip", label: "Klassenfahrt" };
  }

  if (excused) {
    return { reason: "excused", label: "entschuldigt" };
  }

  return null;
}

interface TimeParts {
  hours: number;
  minutes: number;
}

function parseTimeParts(time: string): TimeParts | null {
  const [hoursRaw, minutesRaw] = time.split(":");
  const hours = Number.parseInt(hoursRaw ?? "", 10);
  const minutes = Number.parseInt(minutesRaw ?? "", 10);

  if (Number.isNaN(hours) || Number.isNaN(minutes)) {
    return null;
  }

  return { hours, minutes };
}

/** How a day without care time reads on the narrow card and home rows (#3373). */
export const ONLY_IF_LESSON_CANCELLED_LABEL = "Nur bei Unterrichtsausfall";

function toMinutes(time?: string | null): number | null {
  const parts = time ? parseTimeParts(time) : null;
  return parts ? parts.hours * 60 + parts.minutes : null;
}

/**
 * The child is booked for the day, but the planned arrival (usually the end of
 * lessons) is not before the planned pickup: the regular day has no care time.
 * Such a child only comes when a lesson is cancelled, so the missing check-in
 * is not an alarm (#3373). Once the child is here (see `hasArrived`), the
 * special case ends and the ordinary rules apply.
 */
export function comesOnlyIfLessonCancelled(day: StudentDayTimes): boolean {
  if (hasArrived(day)) {
    return false;
  }
  const arrival = toMinutes(day.plannedArrival);
  const pickup = toMinutes(day.plannedPickup);
  return arrival !== null && pickup !== null && arrival >= pickup;
}

function buildTimeDate(time: string, baseDate: Date): Date | null {
  const parts = parseTimeParts(time);
  if (!parts) {
    return null;
  }

  const result = new Date(baseDate);
  result.setHours(parts.hours, parts.minutes, 0, 0);
  return result;
}

function formatTimeDisplay(time?: string): string | undefined {
  if (!time) {
    return undefined;
  }

  return time.length > 5 ? time.slice(0, 5) : time;
}

function formatActualDiff(diffMinutes: number): string | undefined {
  if (diffMinutes === 0) {
    return undefined;
  }

  if (diffMinutes > 0) {
    return `+${diffMinutes} min`;
  }

  return `-${Math.abs(diffMinutes)} min früh`;
}

function formatOverdueDiff(diffMinutes: number): string | undefined {
  if (diffMinutes >= 0) {
    return undefined;
  }

  return `Überfällig seit ${Math.abs(diffMinutes)} min`;
}

export function getStudentTimeStatus({
  plannedTime,
  actualTime,
  now,
  kind,
  day,
  sick,
  classTrip,
  excused,
}: StudentTimeStatusInput): StudentTimeStatus {
  const displayActualTime = formatTimeDisplay(actualTime);
  const displayPlannedTime = formatTimeDisplay(plannedTime);

  const actualDate = actualTime ? buildTimeDate(actualTime, now) : null;
  const plannedDate = plannedTime ? buildTimeDate(plannedTime, now) : null;

  if (displayActualTime) {
    const diffMinutes =
      actualDate && plannedDate
        ? Math.round((actualDate.getTime() - plannedDate.getTime()) / 60000)
        : undefined;

    const doneLate = (diffMinutes ?? 0) > 0;
    return {
      state: doneLate ? "done-late" : "done-on-time",
      displayTime: displayActualTime,
      icon: "check",
      iconColor: doneLate
        ? LOCATION_COLORS.SCHOOLYARD
        : LOCATION_COLORS.GROUP_ROOM,
      textColor: undefined,
      isResolved: true,
      detailAnnotation:
        diffMinutes === undefined ? undefined : formatActualDiff(diffMinutes),
      diffMinutes,
    };
  }

  // Absence wins over the temporal (overdue) path but never over a recorded
  // actual time above — a sick/excused child is not "overdue", they are simply
  // not expected today. Neutral coloring, sorted out of the urgency band.
  const absence = getStudentAbsence({ sick, classTrip, excused });
  if (absence) {
    return {
      state: "absent-excused",
      displayTime: undefined,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: true,
      detailAnnotation: absence.label,
    };
  }

  if (!displayPlannedTime || !plannedDate) {
    return {
      state: "none",
      displayTime: undefined,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: false,
    };
  }

  const diffMinutes = Math.round(
    (plannedDate.getTime() - now.getTime()) / 60000,
  );

  if (kind === "arrival" && day && comesOnlyIfLessonCancelled(day)) {
    return {
      state: "only-if-lesson-cancelled",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: true,
      detailAnnotation: "nur bei Unterrichtsausfall",
      diffMinutes,
    };
  }

  // A pickup can only be late for a child who is here. Without a check-in
  // there is nobody to pick up, whatever the clock says (#3373).
  if (kind === "pickup" && day && !hasArrived(day)) {
    return {
      state: "awaiting-arrival",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: false,
      diffMinutes,
    };
  }

  if (diffMinutes > APPROACHING_THRESHOLD_MINUTES) {
    return {
      state: "planned",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: false,
      diffMinutes,
    };
  }

  if (diffMinutes >= 0) {
    return {
      state: "approaching",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.SCHOOLYARD,
      textColor: undefined,
      isResolved: false,
      diffMinutes,
    };
  }

  if (diffMinutes > -APPROACHING_THRESHOLD_MINUTES) {
    return {
      state: "slightly-overdue",
      displayTime: displayPlannedTime,
      icon: "warning",
      iconColor: LOCATION_COLORS.SCHOOLYARD,
      textColor: LOCATION_COLORS.SCHOOLYARD,
      isResolved: false,
      detailAnnotation: formatOverdueDiff(diffMinutes),
      diffMinutes,
    };
  }

  return {
    state: "very-overdue",
    displayTime: displayPlannedTime,
    icon: "warning",
    iconColor: LOCATION_COLORS.DANGER,
    textColor: LOCATION_COLORS.DANGER,
    isResolved: false,
    detailAnnotation: formatOverdueDiff(diffMinutes),
    diffMinutes,
  };
}

export function areStudentDayTimesResolved(
  arrivalStatus: StudentTimeStatus,
  pickupStatus: StudentTimeStatus,
): boolean {
  return arrivalStatus.isResolved && pickupStatus.isResolved;
}

export function combineTimeNotes(
  notes?: string,
  dayNotes?: ReadonlyArray<{ content: string }>,
): string | undefined {
  const combined = [notes, ...(dayNotes?.map((note) => note.content) ?? [])]
    .filter(Boolean)
    .join(", ");

  return combined || undefined;
}

export function getTimeStatusSortRank(status: StudentTimeStatus): number {
  switch (status.state) {
    case "very-overdue":
      return 0;
    case "slightly-overdue":
      return 1;
    case "approaching":
      return 2;
    case "planned":
    case "awaiting-arrival":
      return 3;
    case "done-late":
      return 4;
    case "done-on-time":
      return 5;
    // Out of the urgency band, alongside resolved students — but above "none"
    // so a known-absent child still outranks one with no schedule at all.
    case "absent-excused":
    case "only-if-lesson-cancelled":
      return 5;
    case "none":
    default:
      return 6;
  }
}
