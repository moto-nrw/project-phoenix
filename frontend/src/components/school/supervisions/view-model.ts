// Was die Aufsichten-Ansicht zeigt (#2527). Ohne React, damit die eine
// Entscheidung, die man falsch treffen kann, einzeln prüfbar ist.
//
// Die Seite hat zwei Zustände, und der Tag bestimmt sie, nicht die Person:
// läuft eine Aufsicht, IST die Seite diese Aufsicht. Läuft keine, zeigt sie
// den Tag. Es gibt deshalb kein Auswahl-Bedienelement — eine Lehrkraft macht
// immer die Aufsicht, die gerade dran ist, und muss das nicht erst angeben.

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

/**
 * Was die Person angesehen HABEN WILL. "auto" ist der Normalfall und heißt:
 * der Tag entscheidet. Die beiden anderen entstehen nur durch ein Antippen.
 */
export type SupervisionViewIntent =
  | { readonly kind: "auto" }
  | { readonly kind: "overview" }
  | { readonly kind: "detail"; readonly id: string };

export const AUTO_VIEW: SupervisionViewIntent = { kind: "auto" };

/** Was die Seite daraufhin tatsächlich rendert. */
export type SupervisionView =
  | { readonly mode: "overview" }
  | {
      readonly mode: "detail";
      readonly instance: PlannedTimetableInstance;
      /**
       * Ob die Seite einen Weg zum Tagesüberblick anbietet. Bei genau einer
       * Aufsicht wäre der Überblick eine Liste mit einer Zeile, also nichts;
       * ab zwei ist sein Fehlen eine Sackgasse — auch während eine Aufsicht
       * läuft, denn dann sind die anderen sonst unerreichbar.
       */
      readonly canGoBack: boolean;
    };

/**
 * Löst den Wunsch gegen den heutigen Stand auf.
 *
 * Eine ausdrücklich geöffnete Aufsicht gewinnt, solange es sie gibt — wird die
 * Einteilung entzogen, während die Ansicht offen ist, fällt die Seite auf den
 * Tag zurück statt auf eine leere Detailansicht.
 *
 * Im Normalfall ("auto") gibt es genau eine Regel: läuft eine Aufsicht, ist
 * sie es. Sonst der Überblick. Bewusst NICHT "die nächste geplante" — die
 * würde die Kinderliste eines Blocks zeigen, der noch gar nicht begonnen hat,
 * und den Startknopf zwischen die Kinder setzen.
 */
export function resolveSupervisionView(
  intent: SupervisionViewIntent,
  instances: readonly PlannedTimetableInstance[],
): SupervisionView {
  const canGoBack = instances.length >= 2;

  if (intent.kind === "detail") {
    const opened = instances.find((item) => item.id === intent.id);
    if (opened) return { mode: "detail", instance: opened, canGoBack };
    return { mode: "overview" };
  }
  if (intent.kind === "overview") return { mode: "overview" };

  const running = instances.find((item) => item.status === "active");
  if (running) return { mode: "detail", instance: running, canGoBack };
  return { mode: "overview" };
}

/**
 * Die Aufsichten, die nach der laufenden noch kommen. Als Satz unter der
 * Kinderliste beantwortet das die einzige andere Frage, die man während einer
 * Aufsicht hat — ohne ein Bedienelement dafür zu bauen.
 */
export function upcomingAfter(
  current: PlannedTimetableInstance,
  instances: readonly PlannedTimetableInstance[],
): PlannedTimetableInstance[] {
  return instances.filter(
    (item) =>
      item.id !== current.id &&
      item.status === "planned" &&
      item.startTime >= current.startTime,
  );
}

/**
 * Was mit einer Aufsicht, die nicht läuft, überhaupt noch passieren kann.
 *
 * Die Zeitfenster kommen vom Server (startAvailableAt / startExpiresAt), aber
 * ob "noch nicht" oder "nicht mehr" gilt, hängt an der Uhr des Geräts — und
 * genau diese zwei Fälle darf die Oberfläche nicht verwechseln. "Lässt sich ab
 * 11:00 starten" um 13:20 ist eine Aufforderung zu warten, die nie endet.
 */
export type SupervisionStartState =
  "cancelled" | "completed" | "startable" | "too_early" | "expired";

export function supervisionStartState(
  instance: PlannedTimetableInstance,
  now: Date,
): SupervisionStartState {
  if (instance.status === "cancelled") return "cancelled";
  if (instance.status === "completed") return "completed";
  if (instance.canStart !== false) return "startable";

  const availableAt = parseInstant(instance.startAvailableAt);
  if (availableAt && now.getTime() < availableAt.getTime()) return "too_early";

  const expiresAt = parseInstant(instance.startExpiresAt);
  if (expiresAt && now.getTime() >= expiresAt.getTime()) return "expired";

  // Der Server sagt "nein", ohne dass ein Fenster das erklärt. Als "noch
  // nicht" zu lesen wäre die freundlichere, aber falsche Annahme: dann wartet
  // jemand auf einen Knopf, der nicht mehr kommt.
  return "expired";
}

function parseInstant(value: string | undefined): Date | null {
  if (!value) return null;
  const parsed = new Date(value);
  return Number.isFinite(parsed.getTime()) ? parsed : null;
}

/**
 * Wie weit die Aufsicht noch weg ist, in einem Satzteil.
 *
 * `minutesUntilStart` kommt vom Server und wird negativ, sobald die Startzeit
 * durch ist. Beide Richtungen brauchen ein eigenes Wort: "in 5 Minuten" und
 * "seit 5 Minuten faellig" sagen etwas sehr Verschiedenes, und die Null
 * dazwischen ist "jetzt", nicht "in 0 Minuten".
 *
 * Gibt null zurueck, wenn die Angabe nichts beitraegt (laufende, beendete oder
 * abgesagte Aufsicht, oder mehr als ein halber Tag entfernt).
 */
export function startProximityLabel(
  instance: PlannedTimetableInstance,
): string | null {
  if (instance.status !== "planned") return null;
  const minutes = instance.minutesUntilStart;
  if (!Number.isFinite(minutes) || Math.abs(minutes) > 12 * 60) return null;

  if (minutes <= -60) {
    const hours = Math.floor(-minutes / 60);
    return `seit ${hours} ${hours === 1 ? "Stunde" : "Stunden"} fällig`;
  }
  if (minutes < 0) return `seit ${-minutes} Min. fällig`;
  if (minutes === 0) return "jetzt";
  if (minutes < 60) return `in ${minutes} Min.`;
  const hours = Math.round(minutes / 60);
  return `in etwa ${hours} ${hours === 1 ? "Stunde" : "Stunden"}`;
}

/** Was der Tag insgesamt bringt — die Zeile ueber der Liste. */
export interface SupervisionDaySummary {
  readonly count: number;
  readonly children: number;
  /** Die naechste Aufsicht, die noch bevorsteht; null wenn alles durch ist. */
  readonly next: PlannedTimetableInstance | null;
  readonly running: PlannedTimetableInstance | null;
}

export function summarizeDay(
  instances: readonly PlannedTimetableInstance[],
): SupervisionDaySummary {
  const relevant = instances.filter((item) => item.status !== "cancelled");
  return {
    count: relevant.length,
    // Kinder werden summiert, nicht dedupliziert: dasselbe Kind in Lernzeit
    // UND Mensa sind zwei Aufsichten, in denen es betreut wird. "13 Kinder
    // heute" ist die Arbeitsmenge, nicht die Kopfzahl.
    children: relevant.reduce(
      (sum, item) => sum + item.expectedStudentsCount,
      0,
    ),
    next: relevant.find((item) => item.status === "planned") ?? null,
    running: relevant.find((item) => item.status === "active") ?? null,
  };
}
