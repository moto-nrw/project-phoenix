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
 * Leerzeichen ("Klasse 2a"), deshalb kodiert.
 */
export function classDayPath(schoolClass: string, dateISO: string): string {
  const klass = encodeURIComponent(schoolClass);
  return `/school/klasse/${klass}?${CLASS_DAY_DATE_PARAM}=${dateISO}`;
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
