// Adressen und Tagesparameter der Klassenansicht (#2294).
//
// Übersicht und Klassenseite müssen sich über denselben Tag einig sein: der
// Link in eine Klasse trägt ihn hin, der Zurück-Weg trägt ihn zurück. Beide
// Richtungen aus einer Datei, damit sie nicht auseinanderlaufen.

import {
  berlinTodayISO,
  isValidISODate,
  parseISODate,
} from "~/lib/date-helpers";

/** Query-Parameter, unter dem der angezeigte Tag in der Adresse steht. */
const CLASS_DAY_DATE_PARAM = "tag";

/**
 * Query-Parameter, unter dem die angezeigte Klasse in der Adresse steht.
 *
 * Bewusst ein Query-Parameter und kein Adress-Segment: Next.js reicht
 * `params` an eine Seite roh durch, kodiert wie in der Adresse (gemessen an
 * Next 16.3 in einem leeren Projekt, Server- wie Client-Seite, direkter
 * Aufruf wie Navigation im Portal). Ein Segment müsste die Seite deshalb
 * selbst dekodieren — eine Zeile, deren Richtigkeit man nur am laufenden
 * Portal sieht. `useSearchParams()` dekodiert dagegen selbst und
 * zuverlässig, auch für Namen mit Prozentzeichen, Schrägstrich oder Plus
 * ("5 b/c", "100%"). Damit gibt es im Produktcode kein manuelles Dekodieren
 * mehr (#2294).
 */
export const CLASS_DAY_CLASS_PARAM = "klasse";

/**
 * Tag aus der Adresse lesen. Fehlt er oder ist er unbrauchbar, gilt heute —
 * eine kaputte Adresse darf die Übergabe nicht blockieren.
 */
export function classDayDateParam(
  raw: string | null | undefined,
  today: string = berlinTodayISO(),
): string {
  if (!raw || !isValidISODate(raw)) return today;
  return raw;
}

/**
 * Adresse einer Klassenseite. Klassennamen sind Freitext und enthalten
 * Leerzeichen ("Klasse 2a") oder Schrägstriche ("5 b/c"), deshalb kodiert.
 */
export function classDayPath(schoolClass: string, dateISO: string): string {
  const klass = encodeURIComponent(schoolClass);
  return `/school/klasse?${CLASS_DAY_CLASS_PARAM}=${klass}&${CLASS_DAY_DATE_PARAM}=${dateISO}`;
}

/** Adresse der Tagesübersicht für denselben Tag. */
export function classDayOverviewPath(dateISO: string): string {
  return `/school?${CLASS_DAY_DATE_PARAM}=${dateISO}`;
}

/**
 * Samstag und Sonntag haben keine Übergabe — client-seitig erkennbar, also
 * gar nicht erst laden (spart pro Klasse den vollen Report samt GDPR-Logzeile).
 */
export function isWeekendISO(dateISO: string): boolean {
  const day = parseISODate(dateISO).getDay();
  return day === 0 || day === 6;
}

/**
 * Klassenname aus der Adresse. Gegenstück zu `classDayPath`.
 *
 * `useSearchParams().get()` liefert den Namen bereits dekodiert; hier bleibt
 * nur die Frage, ob überhaupt eine Klasse in der Adresse steht. Fehlt sie,
 * kommt ein leerer Name zurück und die Seite sagt das, statt eine Klasse
 * ohne Namen zu laden.
 */
export function classDayClassParam(raw: string | null | undefined): string {
  return raw?.trim() ?? "";
}
