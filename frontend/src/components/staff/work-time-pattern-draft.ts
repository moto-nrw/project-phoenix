// Der Entwurf eines Wochenmusters (Soll-Stunden Mo–Fr je Rotationswoche).
// Ihn teilen der Reiter „Arbeitszeitmodell“ einer Person und die Katalogseite
// der Vorlagen (#6891), damit beide dieselben Eingaben gleich lesen und
// dieselben Fehler melden.

import { formatDecimalHours, parseDecimalHours } from "./arbeitszeit-hours";

export const WORK_DAYS: readonly number[] = [0, 1, 2, 3, 4];
export const WEEK_BADGE_LETTERS = ["A", "B", "C", "D"] as const;

export interface PatternEntry {
  readonly weekIndex: number;
  readonly dayOfWeek: number;
  readonly targetMinutes: number;
  readonly startTime?: string;
}

export interface PatternDraft {
  readonly rotationLength: number;
  /** Genau ein Eintrag je Rotationswoche und Werktag. */
  readonly entries: readonly PatternEntry[];
  /** Was im Stundenfeld steht, auch wenn es (noch) ungültig ist. */
  readonly hourInputs: Readonly<Record<string, string>>;
  readonly invalidKeys: ReadonlySet<string>;
}

export function entryKey(weekIndex: number, dayOfWeek: number): string {
  return `${weekIndex}:${dayOfWeek}`;
}

const weekOfKey = (key: string) => Number(key.split(":")[0]);

/** Ordnet die Einträge auf Rotation × Werktage; fehlende Tage haben 0. */
function shapeEntries(
  rotationLength: number,
  source: readonly PatternEntry[],
): PatternEntry[] {
  const out: PatternEntry[] = [];
  for (let w = 0; w < rotationLength; w++) {
    for (const d of WORK_DAYS) {
      const existing = source.find(
        (e) => e.weekIndex === w && e.dayOfWeek === d,
      );
      out.push({
        weekIndex: w,
        dayOfWeek: d,
        targetMinutes: existing?.targetMinutes ?? 0,
        startTime: existing?.startTime,
      });
    }
  }
  return out;
}

export function draftFromPattern(
  rotationLength: number,
  entries: readonly PatternEntry[],
): PatternDraft {
  const shaped = shapeEntries(Math.max(1, rotationLength), entries);
  return {
    rotationLength: Math.max(1, rotationLength),
    entries: shaped,
    hourInputs: Object.fromEntries(
      shaped.map((e) => [
        entryKey(e.weekIndex, e.dayOfWeek),
        e.targetMinutes > 0 ? formatDecimalHours(e.targetMinutes) : "",
      ]),
    ),
    invalidKeys: new Set(),
  };
}

export function withRotationLength(
  draft: PatternDraft,
  rotationLength: number,
): PatternDraft {
  return {
    rotationLength,
    entries: shapeEntries(rotationLength, draft.entries),
    hourInputs: Object.fromEntries(
      Object.entries(draft.hourInputs).filter(
        ([key]) => weekOfKey(key) < rotationLength,
      ),
    ),
    invalidKeys: new Set(
      [...draft.invalidKeys].filter((key) => weekOfKey(key) < rotationLength),
    ),
  };
}

function updateEntry(
  draft: PatternDraft,
  weekIndex: number,
  dayOfWeek: number,
  change: (entry: PatternEntry) => PatternEntry,
): readonly PatternEntry[] {
  return draft.entries.map((e) =>
    e.weekIndex === weekIndex && e.dayOfWeek === dayOfWeek ? change(e) : e,
  );
}

/** Ein ungültiger Wert bleibt sichtbar, zählt aber 0 Minuten. */
export function withDecimalHours(
  draft: PatternDraft,
  weekIndex: number,
  dayOfWeek: number,
  value: string,
): PatternDraft {
  const key = entryKey(weekIndex, dayOfWeek);
  const result = parseDecimalHours(value);
  const minutes = result.status === "valid" ? result.minutes : 0;
  const invalidKeys = new Set(draft.invalidKeys);
  if (result.status === "invalid") invalidKeys.add(key);
  else invalidKeys.delete(key);
  return {
    ...draft,
    entries: updateEntry(draft, weekIndex, dayOfWeek, (e) => ({
      ...e,
      targetMinutes: Math.max(0, Math.min(720, minutes)),
    })),
    hourInputs: { ...draft.hourInputs, [key]: value },
    invalidKeys,
  };
}

export function withStartTime(
  draft: PatternDraft,
  weekIndex: number,
  dayOfWeek: number,
  startTime: string,
): PatternDraft {
  return {
    ...draft,
    entries: updateEntry(draft, weekIndex, dayOfWeek, (e) => ({
      ...e,
      startTime: startTime || undefined,
    })),
  };
}

export function weekTotal(draft: PatternDraft, weekIndex: number): number {
  return draft.entries
    .filter((e) => e.weekIndex === weekIndex)
    .reduce((sum, e) => sum + e.targetMinutes, 0);
}

export function rotationTotal(draft: PatternDraft): number {
  return draft.entries.reduce((sum, e) => sum + e.targetMinutes, 0);
}

export function weekLabel(weekIndex: number): string {
  return `Woche ${WEEK_BADGE_LETTERS[weekIndex] ?? weekIndex + 1}`;
}

/**
 * Der erste Grund, warum der Entwurf nicht gespeichert werden kann, oder
 * `null`. Ein Muster ohne Stunden oder mit einer leeren Rotationswoche sähe
 * nach dem Speichern kaputt aus und erzeugte jede n-te Woche Minusstunden.
 */
export function validatePatternDraft(draft: PatternDraft): string | null {
  if (draft.invalidKeys.size > 0) {
    return "Bitte korrigieren Sie die markierten Stunden.";
  }
  if (rotationTotal(draft) === 0) {
    return "Das Modell hat noch keine Stunden. Bitte tragen Sie mindestens einen Tag ein.";
  }
  for (let w = 0; w < draft.rotationLength; w++) {
    if (weekTotal(draft, w) === 0) {
      return `${weekLabel(w)} hat noch keine Stunden. Bitte tragen Sie mindestens einen Tag ein.`;
    }
  }
  return null;
}

/** Was das Backend bekommt: nur Tage mit Stunden. */
export function patternEntries(draft: PatternDraft): PatternEntry[] {
  return draft.entries
    .filter((e) => e.targetMinutes > 0)
    .map((e) => ({
      weekIndex: e.weekIndex,
      dayOfWeek: e.dayOfWeek,
      targetMinutes: e.targetMinutes,
      ...(e.startTime ? { startTime: e.startTime } : {}),
    }));
}
