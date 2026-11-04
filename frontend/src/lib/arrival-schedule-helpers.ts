import {
  berlinTodayISO,
  careWeekDayCount,
  parseISODate,
  toISODate as formatDateISO,
} from "./date-helpers";
import { schoolClassLabel } from "./school-class-label";
import type {
  ArrivalException,
  ArrivalNote,
  ArrivalSchedule,
} from "./student-arrival-api";
import type { StudentStatusKind } from "./student-status-days-api";

export const WEEKDAYS = [
  { value: 1, label: "Montag", shortLabel: "Mo" },
  { value: 2, label: "Dienstag", shortLabel: "Di" },
  { value: 3, label: "Mittwoch", shortLabel: "Mi" },
  { value: 4, label: "Donnerstag", shortLabel: "Do" },
  { value: 5, label: "Freitag", shortLabel: "Fr" },
] as const;

export function getWeekdayLabel(weekday: number): string {
  const found = WEEKDAYS.find((w) => w.value === weekday);
  return found ? found.label : `Tag ${weekday}`;
}

export function formatArrivalTime(time: string): string {
  if (time.length > 5) return time.substring(0, 5);
  return time;
}

export function arrivalScheduleSourceLabel(
  schedule: Pick<ArrivalSchedule, "source" | "source_class"> | null | undefined,
): string | null {
  if (schedule?.source === "class_schedule") {
    return schedule.source_class
      ? `aus ${schoolClassLabel(schedule.source_class)}`
      : "Klassenzeit";
  }
  if (schedule?.source === "staff") return "eigene Zeit";
  return null;
}

export function getWeekStart(
  weekOffset = 0,
  todayISO = berlinTodayISO(),
): Date {
  const today = parseISODate(todayISO);
  const dayOfWeek = today.getDay();
  const daysToMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1;
  const monday = new Date(today);
  monday.setDate(today.getDate() - daysToMonday + weekOffset * 7);
  monday.setHours(0, 0, 0, 0);
  return monday;
}

export function getWeekDays(
  weekOffset = 0,
  weekendFollowsFriday = false,
  todayISO = berlinTodayISO(),
): Date[] {
  const monday = getWeekStart(weekOffset, todayISO);
  return getWeekDaysForStart(
    formatDateISO(monday),
    weekendFollowsFriday,
    todayISO,
  );
}

/** Returns the visible care days for a week identified by its Monday. */
export function getWeekDaysForStart(
  weekStartISO: string,
  weekendFollowsFriday = false,
  todayISO = berlinTodayISO(),
): Date[] {
  const monday = parseISODate(weekStartISO);
  const days: Date[] = [];
  const dayCount = careWeekDayCount(
    formatDateISO(monday),
    weekendFollowsFriday,
    todayISO,
  );
  for (let i = 0; i < dayCount; i++) {
    const day = new Date(monday);
    day.setDate(monday.getDate() + i);
    days.push(day);
  }
  return days;
}

export function formatShortDate(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}.`;
}

function getWeekdayFromDate(date: Date): number | null {
  const jsDay = date.getDay();
  if (jsDay === 0 || jsDay === 6) return null;
  return jsDay;
}

export { formatDateISO };

export interface ArrivalScheduleFormEntry {
  weekday: number;
  /** The child is in care on this weekday. */
  inCare: boolean;
  /** The child's own time. Empty means the class time applies. */
  expected_arrival: string;
  /** The class time that applies when the child has none. Read-only. */
  classTime?: string;
  notes?: string | null;
}

/**
 * Splits what the API returns into what the form edits: whether the child is
 * in care that weekday, its own time if it deviates, and the class time it
 * would otherwise inherit (#2414). An inherited time must never come back as
 * the child's own on save.
 */
export function mergeSchedulesWithTemplate(
  existing: ArrivalSchedule[],
): ArrivalScheduleFormEntry[] {
  return WEEKDAYS.map((w) => {
    const match = existing.find((s) => s.weekday === w.value);
    const inherited = match?.source === "class_schedule";
    const time = match ? formatArrivalTime(match.expected_arrival) : "";
    return {
      weekday: w.value,
      inCare: Boolean(match),
      expected_arrival: inherited ? "" : time,
      classTime: inherited ? time : "",
      notes: match?.notes ?? null,
    };
  });
}

export interface ArrivalDayData {
  date: Date;
  weekday: number;
  isToday: boolean;
  showSick: boolean;
  showClassTrip: boolean;
  showExcused: boolean;
  exception: ArrivalException | undefined;
  baseSchedule: ArrivalSchedule | undefined;
  effectiveTime: string | undefined;
  effectiveReason: string | undefined;
  isException: boolean;
  isAbsent: boolean;
  notes: ArrivalNote[];
}

export function getDayData(
  date: Date,
  schedules: ArrivalSchedule[],
  exceptions: ArrivalException[],
  notes: ArrivalNote[] = [],
  isSickToday = false,
  isExcusedToday = false,
  statusForDate: StudentStatusKind | null = null,
  weekendFollowsFriday = false,
  todayISO = berlinTodayISO(),
): ArrivalDayData {
  const calendarWeekday = getWeekdayFromDate(date);
  const weekday = calendarWeekday ?? 5;
  const dateStr = formatDateISO(date);
  const isToday = dateStr === todayISO;

  if (calendarWeekday === null && !weekendFollowsFriday) {
    return {
      date,
      weekday: 0,
      isToday,
      showSick: false,
      showClassTrip: false,
      showExcused: false,
      exception: undefined,
      baseSchedule: undefined,
      effectiveTime: undefined,
      effectiveReason: undefined,
      isException: false,
      isAbsent: false,
      notes: [],
    };
  }

  const exception = exceptions.find((e) => e.exception_date === dateStr);
  const baseSchedule = schedules.find((s) => s.weekday === weekday);
  const showSick = statusForDate
    ? statusForDate === "sick"
    : isToday && isSickToday;
  const showClassTrip = statusForDate === "class_trip";
  const showExcused = statusForDate
    ? statusForDate === "excused"
    : isToday && !showSick && !showClassTrip && isExcusedToday;

  let effectiveTime: string | undefined;
  let isAbsent = false;
  let effectiveReason: string | undefined =
    exception?.reason ?? baseSchedule?.notes ?? undefined;

  if (showSick) {
    effectiveTime = undefined;
    isAbsent = true;
    effectiveReason = "Krank";
  } else if (showClassTrip) {
    effectiveTime = undefined;
    isAbsent = true;
    effectiveReason = "Klassenfahrt";
  } else if (showExcused) {
    effectiveTime = undefined;
    isAbsent = true;
    effectiveReason = "Entschuldigt";
  } else if (exception) {
    if (exception.expected_arrival) {
      effectiveTime = formatArrivalTime(exception.expected_arrival);
    } else {
      effectiveTime = undefined;
      isAbsent = true;
    }
  } else if (baseSchedule) {
    // A care day whose class carries no time yet has no arrival time. It must
    // stay a care day, so isAbsent is deliberately not set (#2414).
    effectiveTime =
      formatArrivalTime(baseSchedule.expected_arrival) || undefined;
  }

  const dayNotes = notes.filter((n) => n.note_date === dateStr);

  return {
    date,
    weekday,
    isToday,
    showSick,
    showClassTrip,
    showExcused,
    exception,
    baseSchedule,
    effectiveTime,
    effectiveReason,
    isException: !!exception,
    isAbsent,
    notes: dayNotes,
  };
}
