import { blockPhase, minutesBetween } from "~/lib/home-clock";
import type { OwnAssignment } from "~/lib/shift-helpers";
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

interface NowAction {
  readonly href: string;
  readonly label: string;
}

/**
 * Höchstens zwei Wege, die an dieser Stelle im Tag wirklich naheliegen —
 * nicht die ganze Navigation als Knopfleiste. Läuft eine Aufsicht, ist das
 * Fortsetzen der eine Weg; sonst der Tag, sonst die eigene Gruppe, sonst
 * alle Kinder.
 */
export function nowActions({
  isSupervising,
  canOpenGroup,
  tenantPath,
}: {
  readonly isSupervising: boolean;
  readonly canOpenGroup: boolean;
  readonly tenantPath: (path: string) => string;
}): readonly NowAction[] {
  const actions: NowAction[] = [];
  if (isSupervising) {
    actions.push({
      href: tenantPath("/active-supervisions"),
      label: "Aufsicht fortsetzen",
    });
  }
  // Kein Weg in den Tagesplan: der Tag steht als Baustein direkt unter der
  // Zone, mit Weiterlink und Starten-Knopf. Ein zweiter Knopf darüber wäre
  // derselbe Weg zweimal.
  if (canOpenGroup) {
    actions.push({ href: tenantPath("/ogs-groups"), label: "Meine Gruppe" });
  }
  if (actions.length === 0) {
    actions.push({
      href: tenantPath("/students/search"),
      label: "Alle Kinder",
    });
  }
  return actions.slice(0, 2);
}
