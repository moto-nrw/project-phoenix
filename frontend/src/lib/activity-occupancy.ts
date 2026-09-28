// Belegung einer laufenden Aktivität gegen ihre Teilnehmergrenze (#3634).
//
// Die Zahl ist immer die der Kinder, die gerade in der Sitzung sind, also
// dieselbe, die das Tablet mit der Grenze vergleicht. Im Web darf die Grenze
// überschritten werden; das Tablet nimmt dann keine Kinder mehr an. Darum ist
// „überbucht" ein eigener Zustand neben „voll" (Belegung = Grenze).

/** Grenze der Aktivität; null oder undefined heißt: keine Grenze. */
export type ParticipantLimit = number | null | undefined;

/** Kinder einer laufenden Sitzung und die Grenze ihrer Aktivität. */
export interface Occupancy {
  readonly count: number;
  readonly limit: ParticipantLimit;
}

export const OVERBOOKED_LABEL = "Überbucht";

export function isOverbooked(count: number, limit: ParticipantLimit): boolean {
  return limit != null && limit > 0 && count > limit;
}

/** „66 / 45 Kinder" mit Grenze, sonst „66 Kinder" bzw. „1 Kind". */
export function formatChildCount(
  count: number,
  limit: ParticipantLimit,
): string {
  if (limit != null && limit > 0) return `${count} / ${limit} Kinder`;
  return `${count} ${count === 1 ? "Kind" : "Kinder"}`;
}

/** „66 / 45 anwesend" mit Grenze, sonst „66 anwesend". */
export function formatPresentAgainstLimit(
  present: number,
  limit: ParticipantLimit,
): string {
  if (limit != null && limit > 0) return `${present} / ${limit} anwesend`;
  return `${present} anwesend`;
}

/**
 * Kurzer Hinweis zu einer überbuchten Aktivität. Die Sätze zum Tablet gibt es
 * nur für Schulen mit NFC; ohne Tablet lehnt nichts ab.
 */
export function overbookedHint(limit: number, nfcEnabled: boolean): string {
  const tooMany = `Mehr Kinder als erlaubt (höchstens ${limit}).`;
  if (!nfcEnabled) return tooMany;
  return `${tooMany} Am Tablet kann sich jetzt kein Kind anmelden. Das geht wieder unter ${limit} Kindern oder mit höherer Grenze.`;
}

/** Der Hinweis, wenn die Sitzung überbucht ist; sonst null. */
export function overbookedHintFor(
  { count, limit }: Occupancy,
  nfcEnabled: boolean,
): string | null {
  if (limit == null || !isOverbooked(count, limit)) return null;
  return overbookedHint(limit, nfcEnabled);
}
