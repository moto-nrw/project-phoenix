/**
 * Kinderzahl und Uhrzeit eines Blocks, wie Tagesplan, „Mein Tag“ und
 * „Ablauf des Tages“ sie in einer Zeile zeigen (#3921).
 *
 * „X von Y da“ heißt: X Kinder sind gerade da, Y Kinder gehören heute zu
 * diesem Block. Wer gegangen ist, steht getrennt daneben. Ungeplant
 * dazugekommene Kinder zählen in X, aber nicht in Y. Ein Block ohne eigene
 * Kinder (spontan gestartet) zeigt nur „X da“, nie „X von 0 da“. Sind mehr
 * Kinder da als geplant, steht ebenfalls nur „X da“.
 */

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

type BlockCounts = Pick<
  PlannedTimetableInstance,
  | "status"
  | "expectedStudentsCount"
  | "presentStudentsCount"
  | "currentStudentsCount"
  | "plannedStudentsCount"
>;

function kinder(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

/**
 * Laufend: „12 von 18 da · 3 gegangen“. Beendet: wie viele da waren.
 * Davor: wie viele erwartet werden.
 */
export function blockChildrenLabel(block: BlockCounts): string {
  if (block.status === "completed") return kinder(block.presentStudentsCount);
  if (block.status !== "active") return kinder(block.expectedStudentsCount);
  // Older BFF responses do not have the #3921 fields yet. Their established
  // counts are the closest compatible values; treating omitted fields as zero
  // turns a populated running block into the false label "0 da".
  const current = block.currentStudentsCount ?? block.presentStudentsCount;
  const planned = block.plannedStudentsCount ?? block.expectedStudentsCount;
  const departed = Math.max(0, block.presentStudentsCount - current);
  // Mehr Kinder da als geplant (dazugekommene): „10 von 6“ wäre unlogisch.
  const here =
    planned > 0 && current <= planned
      ? `${current} von ${planned} da`
      : `${current} da`;
  return departed > 0 ? `${here} · ${departed} gegangen` : here;
}

type BlockClock = Pick<
  PlannedTimetableInstance,
  "startTime" | "endTime" | "isSpontaneous"
> & {
  status: string;
};

/**
 * Ein spontan gestarteter Block hat bis zum Beenden kein Ende. Die geplante
 * Endzeit ist dann nur ein Platzhalter und wird nicht gezeigt.
 */
export function hasOpenEnd(block: BlockClock): boolean {
  return block.isSpontaneous === true && block.status === "active";
}

/** Uhrzeit in einer Zeile: „14:00–15:00“ oder „seit 15:16“. */
export function blockTimeRange(block: BlockClock): string {
  return hasOpenEnd(block)
    ? `seit ${block.startTime}`
    : `${block.startTime}–${block.endTime}`;
}

/** Zweite Zeile unter der Startzeit: „bis 15:00“ oder „Ende offen“. */
export function blockEndLine(block: BlockClock): string {
  return hasOpenEnd(block) ? "Ende offen" : `bis ${block.endTime}`;
}
