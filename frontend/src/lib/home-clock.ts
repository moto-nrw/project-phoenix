/**
 * Rechnen mit Wanduhrzeiten „HH:MM" für die Startseite (#2180).
 *
 * Die Blöcke des Betreuungsplans kennen keine Zeitzone, nur eine Uhrzeit an
 * der Wand. Was „jetzt" ist, entscheidet dieselbe Uhr (Europe/Berlin), und der
 * Vergleich bleibt eine Rechnung mit Minuten — kein Date-Objekt, das um
 * Mitternacht einen Tag verrutschen könnte.
 */

/** Minuten seit Mitternacht; eine unbrauchbare Angabe zählt als 0. */
export function wallClockMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(":");
  const hours = Number(h);
  const minutes = Number(m);
  if (!Number.isFinite(hours) || !Number.isFinite(minutes)) return 0;
  return hours * 60 + minutes;
}

/** Minuten von `from` bis `to`; negativ, wenn `to` schon vorbei ist. */
export function minutesBetween(from: string, to: string): number {
  return wallClockMinutes(to) - wallClockMinutes(from);
}

export type BlockPhase = "past" | "running" | "upcoming";

/**
 * Wo ein Block relativ zur Uhr steht. Das Ende ist exklusiv: um 11:00 ist
 * ein Block „10:00–11:00" vorbei und der nächste läuft.
 */
export function blockPhase(
  startTime: string,
  endTime: string,
  now: string,
): BlockPhase {
  if (endTime <= now) return "past";
  if (startTime <= now) return "running";
  return "upcoming";
}

/**
 * „in 5 Min", „in 1 Std", „in 1 Std 20 Min". Unter einer Minute heißt es
 * „gleich": eine Null vor „Min" liest sich wie ein Fehler.
 */
export function formatMinutesAhead(minutes: number): string {
  if (minutes < 1) return "gleich";
  if (minutes < 60) return `in ${minutes} Min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `in ${hours} Std` : `in ${hours} Std ${rest} Min`;
}
