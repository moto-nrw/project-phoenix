// Rechenteil der eigenen Dienstplan-Woche (#3821). Rein und ohne React,
// damit die Ansicht nur noch zusammensetzt.

import { parseISODate, toISODate } from "~/lib/date-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";
import type { StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";
import { startOfWeek } from "~/lib/staff-metrics-helpers";

const WORKWEEK_DAYS = 5;
const WEEK_DAYS = 7;

/** Die sieben Tage (Mo–So) der Woche, die `dayISO` enthält. */
export function calendarWeekDays(dayISO: string): string[] {
  const monday = startOfWeek(parseISODate(dayISO));
  return Array.from({ length: WEEK_DAYS }, (_, index) => {
    const day = new Date(monday);
    day.setDate(day.getDate() + index);
    return toISODate(day);
  });
}

/**
 * Die Spalten des Rasters: Mo–Fr, dazu Sa und So nur, wenn dort eine Schicht
 * liegt. Sonst fehlten Wochenend-Dienste im Raster, obwohl sie in der
 * Wochensumme zählen. Ein Sonntagsdienst zieht den Samstag mit, damit die
 * Tage lückenlos bleiben.
 */
export function visibleWeekDays(
  weekDays: readonly string[],
  shifts: readonly StaffShift[],
): string[] {
  const dates = new Set(shifts.map((shift) => shift.date));
  let lastIndex = WORKWEEK_DAYS - 1;
  for (let index = WORKWEEK_DAYS; index < weekDays.length; index += 1) {
    const day = weekDays[index];
    if (day !== undefined && dates.has(day)) lastIndex = index;
  }
  return weekDays.slice(0, lastIndex + 1);
}

/** "08:15" → 495; ungültige Werte ergeben null. */
function clockMinutes(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  return Number(match[1]) * 60 + Number(match[2]);
}

/**
 * Geplante Netto-Minuten der Woche: Spanne minus Pause, ausgefallene
 * Schichten zählen nicht. Dieselbe Regel wie die Wochensumme im Dienstplan.
 */
export function plannedWeekMinutes(shifts: readonly StaffShift[]): number {
  let total = 0;
  for (const shift of shifts) {
    if (shift.cancelled) continue;
    const start = clockMinutes(shift.startTime);
    const end = clockMinutes(shift.endTime);
    if (start === null || end === null) continue;
    total += Math.max(0, end - start - shift.breakMinutes);
  }
  return total;
}

/**
 * Soll der Woche als Summe der Tages-Solls (Mo–So) aus der Zeiterfassung.
 * So stimmt die Zahl mit „Diese Woche … Soll" auf der Zeiterfassung überein:
 * Feiertage, Schließtage und Sonderarbeitszeiten sind schon eingerechnet.
 * Null, solange für keinen Tag der Woche ein Soll vorliegt.
 */
export function targetWeekMinutes(
  weekDays: readonly string[],
  projection: ReadonlyMap<string, DayProjection> | undefined,
): number | null {
  if (!projection) return null;
  let total = 0;
  let found = false;
  for (const day of weekDays) {
    const entry = projection.get(day);
    if (!entry) continue;
    found = true;
    total += entry.targetMinutes;
  }
  return found ? total : null;
}

/**
 * Schichtarten, wie die eigenen Schichten sie mitbringen. Die Liste aller
 * Schichtarten der Schule dürfen Mitarbeitende nicht lesen; Name und Farbe
 * liefert der Server aber an jeder Schicht mit. Reihenfolge nach Namen.
 */
export function shiftTypesFromShifts(
  shifts: readonly StaffShift[],
): ShiftType[] {
  const byId = new Map<string, ShiftType>();
  for (const shift of shifts) {
    if (!shift.shiftTypeId || byId.has(shift.shiftTypeId)) continue;
    byId.set(shift.shiftTypeId, {
      id: shift.shiftTypeId,
      name: shift.shiftTypeName ?? "Schicht",
      color: shift.shiftTypeColor ?? "",
      description: "",
      isActive: true,
    });
  }
  return [...byId.values()].sort((a, b) => a.name.localeCompare(b.name, "de"));
}

/**
 * Wie weit der Leerzustand nach der nächsten Schicht sucht: acht ganze
 * Wochen. Der Server erlaubt höchstens 62 Tage je Abfrage.
 */
export const LOOKAHEAD_WEEKS = 8;

/**
 * Der Suchzeitraum nach einer leeren Woche: vom Montag nach `weekTo` (einem
 * Sonntag) über {@link LOOKAHEAD_WEEKS} ganze Wochen bis zu deren Sonntag.
 */
export function lookaheadRange(weekTo: string): { from: string; to: string } {
  const from = parseISODate(weekTo);
  from.setDate(from.getDate() + 1);
  const to = new Date(from);
  to.setDate(to.getDate() + LOOKAHEAD_WEEKS * WEEK_DAYS - 1);
  return { from: toISODate(from), to: toISODate(to) };
}

/**
 * Tag der frühesten Schicht, die stattfindet. Ausgefallene Schichten zählen
 * nicht: ein Sprung zu einer Woche, in der nur ein Ausfall steht, führt ins
 * Leere. Null, wenn keine Schicht stattfindet.
 */
export function earliestShiftDate(
  shifts: readonly StaffShift[],
): string | null {
  let earliest: string | null = null;
  for (const shift of shifts) {
    if (shift.cancelled) continue;
    if (earliest === null || shift.date < earliest) earliest = shift.date;
  }
  return earliest;
}
