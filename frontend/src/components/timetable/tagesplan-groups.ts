// Zeitgleiche Blöcke im Tagesplan (#5020): Blöcke, deren Zeiten sich
// überschneiden, bilden eine Gruppe und stehen nebeneinander. Die Spalten
// rechnet derselbe Algorithmus wie im Wochenraster (`assignBlockLanes`);
// hier kommt nur hinzu, in welcher Zeile einer Spalte ein Block steht.

import { hasOpenEnd } from "~/lib/block-children";
import { assignBlockLanes, parseTimeToMinutes } from "~/lib/timetable-helpers";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

export interface TagesplanCell {
  instance: PlannedTimetableInstance;
  /** Erste Spalte, 0-basiert. */
  lane: number;
  /** Spalten, die der Block nach rechts mitnimmt. */
  laneSpan: number;
  /** Zeile innerhalb der Gruppe, 0-basiert. */
  row: number;
}

export interface TagesplanGroup {
  key: string;
  /** Spalten der Gruppe; 1 heißt: ein Block allein in voller Breite. */
  laneCount: number;
  /** Chronologisch (Start, Ende, Titel) — auch die Vorlesereihenfolge. */
  cells: TagesplanCell[];
  startTime: string;
  /** Spätestes Ende der Gruppe, für die Jetzt-Linie. */
  endTime: string;
  /** Ein Block der Gruppe läuft spontan ohne festes Ende. */
  openEnd: boolean;
}

interface Window {
  startTime: string;
  endTime: string;
  endMinute: number;
  instance: PlannedTimetableInstance;
}

const MINUTES_PER_DAY = 24 * 60;

function formatMinutes(total: number): string {
  const clamped = Math.min(total, MINUTES_PER_DAY);
  const h = Math.floor(clamped / 60);
  const m = clamped % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

function endTimeToMinutes(endTime: string): number {
  return endTime === "24:00" ? MINUTES_PER_DAY : parseTimeToMinutes(endTime);
}

// Ende, mit dem ein Block in die Überlappung eingeht. Ein Block ohne Dauer
// zählt bis zu seinem Start. Ein spontaner Block hat bis zum Beenden nur ein
// Platzhalter-Ende: er läuft mindestens bis jetzt, sonst stünde er als
// "vorbei" neben Blöcken, die erst nach dem Platzhalter beginnen.
function effectiveEnd(
  instance: PlannedTimetableInstance,
  nowHHMM: string | null,
): Pick<Window, "endTime" | "endMinute"> {
  const end =
    instance.endTime > instance.startTime
      ? instance.endTime
      : instance.startTime;
  const endMinute = endTimeToMinutes(end);
  if (nowHHMM == null || !hasOpenEnd(instance)) {
    return { endTime: end, endMinute };
  }

  // `24:00` is a valid end-of-day boundary, but not a valid HH:MM input.
  // Keep its minute value so lane assignment never parses that display value.
  const runningUntil = Math.min(
    parseTimeToMinutes(nowHHMM) + 1,
    MINUTES_PER_DAY,
  );
  const effectiveEndMinute = Math.max(endMinute, runningUntil);
  return {
    endTime: formatMinutes(effectiveEndMinute),
    endMinute: effectiveEndMinute,
  };
}

function compareWindows(a: Window, b: Window): number {
  return (
    a.startTime.localeCompare(b.startTime) ||
    a.instance.endTime.localeCompare(b.instance.endTime) ||
    a.instance.title.localeCompare(b.instance.title)
  );
}

// Zeile je Block: ein Block steht unter dem letzten Block, der eine seiner
// Spalten belegt. So bleibt jede Spalte von oben nach unten chronologisch.
function placeRows(
  members: { instance: Window; lane: number; laneSpan: number }[],
): TagesplanCell[] {
  const nextFreeRow: number[] = [];
  return members.map(({ instance, lane, laneSpan }) => {
    let row = 0;
    for (let col = lane; col < lane + laneSpan; col++) {
      row = Math.max(row, nextFreeRow[col] ?? 0);
    }
    for (let col = lane; col < lane + laneSpan; col++) {
      nextFreeRow[col] = row + 1;
    }
    return { instance: instance.instance, lane, laneSpan, row };
  });
}

/**
 * Gruppiert die Blöcke eines Tages nach Überlappung. `nowHHMM` ist nur für
 * den heutigen Tag gesetzt: dann zählt ein spontaner, laufender Block
 * mindestens bis jetzt.
 */
export function groupParallelBlocks(
  instances: readonly PlannedTimetableInstance[],
  nowHHMM: string | null,
): TagesplanGroup[] {
  const windows: Window[] = instances.map((instance) => ({
    startTime: instance.startTime,
    ...effectiveEnd(instance, nowHHMM),
    instance,
  }));
  const laned = assignBlockLanes(windows, compareWindows);

  const groups: TagesplanGroup[] = [];
  let members: typeof laned = [];
  const flush = () => {
    const first = members[0];
    if (!first) return;
    const endMinute = members.reduce(
      (max, member) => Math.max(max, member.instance.endMinute),
      first.instance.endMinute,
    );
    groups.push({
      key: first.instance.instance.id,
      laneCount: first.laneCount,
      cells: placeRows(members),
      startTime: first.instance.startTime,
      endTime: formatMinutes(endMinute),
      openEnd: members.some((m) => hasOpenEnd(m.instance.instance)),
    });
    members = [];
  };
  for (const entry of laned) {
    if (members.length > 0 && members[0]!.cluster !== entry.cluster) flush();
    members.push(entry);
  }
  flush();
  return groups;
}
