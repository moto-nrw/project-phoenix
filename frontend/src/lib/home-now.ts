import { blockPhase, minutesBetween } from "~/lib/home-clock";
import type { OwnAssignment } from "~/lib/shift-helpers";
import { canStartPlannedInstance } from "~/lib/timetable-lifecycle";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

/**
 * Was die Jetzt-Zone der Startseite sagt (#2180) — als reine Rechnung, damit
 * sie sich ohne Browser prüfen lässt.
 *
 * Zwei Blickwinkel, je nachdem, wer die Seite öffnet:
 *
 * - DER EIGENE TAG (Betreuung): der Einsatz, der gerade läuft, sonst der
 *   nächste, sonst „alles erledigt". Ausgefallene Einsätze und solche, von
 *   denen die Person abgezogen wurde, zählen nicht als Einsatz — sie stehen
 *   mit Etikett in „Mein Tag".
 * - DIE SCHULE (Leitung): wie viele Blöcke laufen, wie viele nicht gestartet
 *   wurden, was als Nächstes beginnt.
 *
 * Wer beides ist, sieht den eigenen Tag, solange sie heute Einsätze hat, und
 * sonst die Schule.
 */
export type HomeNowState =
  | {
      readonly kind: "own_running";
      readonly block: OwnAssignment;
      readonly next: OwnAssignment | null;
    }
  | {
      readonly kind: "own_next";
      readonly block: OwnAssignment;
      /** Minuten bis zum Beginn. */
      readonly minutesAhead: number;
    }
  | { readonly kind: "own_done"; readonly count: number }
  | {
      readonly kind: "school";
      readonly running: number;
      readonly notStarted: number;
      readonly next: PlannedTimetableInstance | null;
      readonly minutesAhead: number | null;
      readonly total: number;
    }
  /** Nichts, worauf die Uhr zeigen könnte: nur Uhrzeit und Aktionen. */
  | { readonly kind: "plain" };

/** Ein Einsatz, den die Person heute tatsächlich hat. */
function counts(block: OwnAssignment): boolean {
  return !block.cancelled && !block.isAbsent;
}

export function deriveOwnNow(
  assignments: readonly OwnAssignment[],
  now: string,
): Extract<
  HomeNowState,
  { kind: "own_running" | "own_next" | "own_done" }
> | null {
  const mine = assignments
    .filter(counts)
    .slice()
    .sort((a, b) => a.startTime.localeCompare(b.startTime));
  if (mine.length === 0) return null;

  const running = mine.find(
    (block) => blockPhase(block.startTime, block.endTime, now) === "running",
  );
  const upcoming = mine.filter(
    (block) => blockPhase(block.startTime, block.endTime, now) === "upcoming",
  );

  if (running) {
    return { kind: "own_running", block: running, next: upcoming[0] ?? null };
  }
  const next = upcoming[0];
  if (next) {
    return {
      kind: "own_next",
      block: next,
      minutesAhead: minutesBetween(now, next.startTime),
    };
  }
  return { kind: "own_done", count: mine.length };
}

export function deriveSchoolNow(
  blocks: readonly PlannedTimetableInstance[],
  now: string,
): Extract<HomeNowState, { kind: "school" }> {
  const live = blocks.filter((block) => block.status !== "cancelled");
  const running = live.filter((block) => block.status === "active").length;
  // „Nicht gestartet": geplant, der Beginn ist vorbei, niemand hat den Block
  // gestartet. Dasselbe Wort wie im Tagesplan.
  const notStarted = live.filter(
    (block) =>
      block.status === "planned" &&
      (block.isOverdue || block.startTime <= now) &&
      block.endTime > now,
  ).length;
  const next =
    live
      .filter((block) => block.status === "planned" && block.startTime > now)
      .sort((a, b) => a.startTime.localeCompare(b.startTime))[0] ?? null;

  return {
    kind: "school",
    running,
    notStarted,
    next,
    minutesAhead: next ? minutesBetween(now, next.startTime) : null,
    total: live.length,
  };
}

/**
 * Wählt den Blickwinkel. `own` ist `undefined`, wenn die eigenen Einsätze
 * gar nicht abgefragt wurden (kein Recht, kein Betreuungsplan), und `null`,
 * wenn sie abgefragt wurden und leer sind.
 */
export function deriveHomeNow(input: {
  readonly now: string;
  readonly own: readonly OwnAssignment[] | undefined;
  readonly school: readonly PlannedTimetableInstance[] | undefined;
}): HomeNowState {
  if (input.own !== undefined) {
    const own = deriveOwnNow(input.own, input.now);
    if (own) return own;
  }
  if (input.school !== undefined) {
    return deriveSchoolNow(input.school, input.now);
  }
  return { kind: "plain" };
}

/**
 * Der eigene Block, den die Person JETZT starten darf. Dieselbe Regel wie
 * der Starten-Knopf in „Mein Tag": eingeteilt, noch geplant, und der Server
 * gibt das Starten frei (Zeitfenster um den Beginn). Liegen zwei Blöcke im
 * Fenster, zählt der frühere.
 */
export function startableOwnBlock(
  blocks: readonly PlannedTimetableInstance[],
  at: Date,
): PlannedTimetableInstance | null {
  return (
    blocks
      .filter(
        (block) =>
          block.isAssigned &&
          block.status === "planned" &&
          canStartPlannedInstance(block, at),
      )
      .sort((a, b) => a.startTime.localeCompare(b.startTime))[0] ?? null
  );
}

type NowAction =
  | {
      readonly kind: "link";
      readonly href: string;
      readonly label: string;
    }
  | {
      readonly kind: "start";
      readonly block: PlannedTimetableInstance;
      readonly label: string;
    };

/**
 * Höchstens zwei Wege, die an dieser Stelle im Tag wirklich naheliegen —
 * nicht die ganze Navigation als Knopfleiste.
 *
 * Der erste Weg führt in die Aufsicht: läuft die eigene, zu ihr; steht der
 * eigene Block laut Plan an, startet der Knopf ihn. Danach alle Kinder:
 * nach Rückmeldung aus den Schulen der Weg, den das Team am häufigsten
 * nimmt. „Meine Gruppe" steht nicht in der Zone: die eigene Gruppe hat als
 * Baustein darunter ihren Platz, und als schwarzer Hauptknopf war sie das
 * Auffälligste der ganzen Startseite.
 */
export function nowActions({
  isSupervising,
  startable,
  canReadUsers,
  tenantPath,
}: {
  readonly isSupervising: boolean;
  /** Der eigene Block, der jetzt starten darf (`startableOwnBlock`). */
  readonly startable: PlannedTimetableInstance | null;
  readonly canReadUsers: boolean;
  readonly tenantPath: (path: string) => string;
}): readonly NowAction[] {
  const actions: NowAction[] = [];
  // Wer schon beaufsichtigt, startet nicht noch einen Block von hier aus:
  // ein zweiter Block im Fenster steht mit Starten-Knopf in „Mein Tag".
  if (isSupervising) {
    actions.push({
      kind: "link",
      href: tenantPath("/active-supervisions"),
      label: "Zur Aufsicht",
    });
  } else if (startable) {
    actions.push({
      kind: "start",
      block: startable,
      label: "Aufsicht starten",
    });
  }
  // Kein Weg in den Tagesplan: der Tag steht als Baustein direkt unter der
  // Zone, mit Weiterlink. Ein zweiter Knopf darüber wäre derselbe Weg zweimal.
  if (canReadUsers) {
    actions.push({
      kind: "link",
      href: tenantPath("/students/search"),
      label: "Alle Kinder",
    });
  }
  return actions.slice(0, 2);
}
