// Belegungsplan eines Raums (#4768): die Blöcke des Betreuungsplans in diesem
// Raum und die Zeiten dazwischen, in denen der Raum laut Plan frei ist.
// Reine Ableitung ohne Abruf; die Blöcke liefert `fetchRoomPlanBlocks`
// (room-plan-api.ts). Bewusst ohne die Planner-Module (timetable-api,
// -helpers, -types): die Raumseite trägt sonst deren ganzes Bundle (#4768).

import {
  berlinTodayISO,
  careWeekDayCount,
  isCareWeekendDay,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";

/** Ein Block im Raumplan, nur mit den Feldern, die der Reiter zeigt. */
export interface RoomPlanBlock {
  readonly id: string;
  readonly date: string; // YYYY-MM-DD
  readonly startTime: string; // HH:MM
  readonly endTime: string; // HH:MM
  readonly title: string;
  readonly status: string;
  readonly groupName?: string;
  readonly cancelReason?: string;
}

/** Eine Zeile im Tagesplan des Raums: ein Block oder eine freie Zeit. */
export type RoomPlanRow =
  | { readonly kind: "block"; readonly instance: RoomPlanBlock }
  | { readonly kind: "free"; readonly start: string; readonly end: string };

/** "HH:MM" (auch "HH:MM:SS") in Minuten seit Mitternacht. */
function toMinutes(time: string): number {
  const [hours = "0", minutes = "0"] = time.split(":");
  return Number(hours) * 60 + Number(minutes);
}

function toClock(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  return `${String(hours).padStart(2, "0")}:${String(minutes % 60).padStart(2, "0")}`;
}

/** Ein abgesagter Block belegt den Raum nicht. */
function occupiesRoom(instance: RoomPlanBlock): boolean {
  return instance.status !== "cancelled";
}

/**
 * Ordnet die Blöcke eines Tages nach Beginn und setzt freie Zeiten innerhalb
 * der Planungszeit des Tages (`timetable.day_start_time` bis
 * `timetable.day_end_time`) dazwischen. Überlappende Blöcke belegen den Raum
 * gemeinsam; ein abgesagter Block steht in der Liste, zählt aber nicht als
 * Belegung. Eine freie Zeit steht vor einem Block, der zur selben Zeit
 * beginnt.
 */
export function roomDayPlan(
  instances: readonly RoomPlanBlock[],
  dayStartTime: string,
  dayEndTime: string,
): RoomPlanRow[] {
  const blocks = [...instances].sort(
    (a, b) =>
      toMinutes(a.startTime) - toMinutes(b.startTime) ||
      toMinutes(a.endTime) - toMinutes(b.endTime),
  );
  const dayEnd = toMinutes(dayEndTime);
  const free: { start: number; end: number }[] = [];
  let cursor = toMinutes(dayStartTime);
  for (const block of blocks.filter(occupiesRoom)) {
    const start = toMinutes(block.startTime);
    if (start > cursor && cursor < dayEnd) {
      free.push({ start: cursor, end: Math.min(start, dayEnd) });
    }
    cursor = Math.max(cursor, toMinutes(block.endTime));
  }
  if (cursor < dayEnd) free.push({ start: cursor, end: dayEnd });

  const rows: { at: number; row: RoomPlanRow }[] = [
    ...free.map((slot) => ({
      at: slot.start,
      row: {
        kind: "free" as const,
        start: toClock(slot.start),
        end: toClock(slot.end),
      },
    })),
    ...blocks.map((instance) => ({
      at: toMinutes(instance.startTime),
      row: { kind: "block" as const, instance },
    })),
  ];
  // Stabil: bei gleichem Beginn bleibt die freie Zeit vorn, die Blöcke in
  // ihrer Reihenfolge.
  return rows.sort((a, b) => a.at - b.at).map((entry) => entry.row);
}

const WEEKDAY_NAMES = [
  "Sonntag",
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
] as const;

function shiftDays(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

/**
 * Der Betreuungstag zu `iso`, wie im Betreuungsplan: am Wochenende der
 * folgende Montag, außer die Schule betreut das Wochenende nach dem
 * Freitagsplan (#3921) und es ist das heutige Wochenende (#5028).
 */
export function roomPlanCareDay(
  iso: string,
  weekendOpen: boolean,
  todayISO = berlinTodayISO(),
): string {
  if (isCareWeekendDay(iso, weekendOpen, todayISO)) return iso;
  const weekday = parseISODate(iso).getDay();
  if (weekday === 6) return shiftDays(iso, 2);
  if (weekday === 0) return shiftDays(iso, 1);
  return iso;
}

/** Der nächste (1) oder vorige (-1) Betreuungstag. */
export function roomPlanStepDay(
  iso: string,
  direction: 1 | -1,
  weekendOpen: boolean,
  todayISO = berlinTodayISO(),
): string {
  let next = shiftDays(iso, direction);
  while (
    [0, 6].includes(parseISODate(next).getDay()) &&
    !isCareWeekendDay(next, weekendOpen, todayISO)
  ) {
    next = shiftDays(next, direction);
  }
  return next;
}

/** Der Montag der Woche davor (-1) oder danach (1). */
export function roomPlanStepWeek(mondayISO: string, direction: 1 | -1): string {
  return shiftDays(mondayISO, 7 * direction);
}

/**
 * Montag bis Freitag der Woche von `iso`; bis Sonntag nur in der Woche mit
 * dem heutigen Wochenende (`careWeekDayCount`, #5028).
 */
export function roomPlanWeekDays(
  iso: string,
  weekendOpen: boolean,
  todayISO?: string,
): string[] {
  const monday = shiftDays(iso, -((parseISODate(iso).getDay() + 6) % 7));
  return Array.from(
    { length: careWeekDayCount(iso, weekendOpen, todayISO) },
    (_, index) => shiftDays(monday, index),
  );
}

/** „Dienstag, 27.10.2026“. */
export function roomPlanDayLabel(iso: string): string {
  const date = parseISODate(iso);
  const day = String(date.getDate()).padStart(2, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  return `${WEEKDAY_NAMES[date.getDay()]}, ${day}.${month}.${date.getFullYear()}`;
}
