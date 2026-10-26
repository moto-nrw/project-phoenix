// Belegungsplan eines Raums (#4768): die Blöcke des Betreuungsplans in diesem
// Raum und die Zeiten dazwischen, in denen der Raum laut Plan frei ist.
// Reine Ableitung ohne Abruf; die Blöcke liefert GET /api/timetable/instances
// mit `room_id`.

import type { EnrichedInstance } from "~/lib/timetable-types";

/** Eine Zeile im Tagesplan des Raums: ein Block oder eine freie Zeit. */
export type RoomPlanRow =
  | { readonly kind: "block"; readonly instance: EnrichedInstance }
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
function occupiesRoom(instance: EnrichedInstance): boolean {
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
  instances: readonly EnrichedInstance[],
  dayStartHour: number,
  dayEndHour: number,
): RoomPlanRow[] {
  const blocks = [...instances].sort(
    (a, b) =>
      toMinutes(a.startTime) - toMinutes(b.startTime) ||
      toMinutes(a.endTime) - toMinutes(b.endTime),
  );
  const dayEnd = dayEndHour * 60;
  const free: { start: number; end: number }[] = [];
  let cursor = dayStartHour * 60;
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
