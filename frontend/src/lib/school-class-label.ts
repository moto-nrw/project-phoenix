/**
 * Beschriftung einer Schulklasse — die einzige Stelle im Frontend, die
 * „Klasse“ vor einen Klassennamen setzt (#5101).
 *
 * Klassennamen sind Freitext: manche Schulen speichern "1a", andere
 * "Klasse 1a", "klasse 1a" oder "1. Klasse". Keiner davon darf zu
 * "Klasse Klasse 1a" werden. Das Backend folgt derselben Regel
 * (`collation.SchoolClassLabel` in `backend/sharedkernel/collation`).
 * Die oxlint-Regel `school-class/no-hand-built-label` sperrt neue
 * Stellen, die „Klasse “ selbst vor einen Wert setzen.
 */

// „Klasse 1a“, „klasse 1a“, „Klasse1a“: der Name trägt das Wort schon vorn.
// „Klassenfahrt“ zählt nicht, dort folgt ein Buchstabe.
const LEADING_PREFIX = /^klasse(?=$|[\s\d])\s*/i;
// „1. Klasse“: eine Klassenstufe, die das Wort hinten trägt.
const TRAILING_WORD = /\sklasse$/i;

/**
 * „1b“, „Klasse 1b“, „klasse 1b“ und „ 1b “ → „Klasse 1b“;
 * „1. Klasse“ bleibt, wie es ist. Ein leerer Name ergibt "".
 */
export function schoolClassLabel(klass: string): string {
  const trimmed = klass.trim();
  if (trimmed === "") return "";
  if (TRAILING_WORD.test(trimmed)) return trimmed;
  const name = schoolClassName(trimmed);
  return name === "" ? "Klasse" : `Klasse ${name}`;
}

/**
 * Der Name ohne vorangestelltes „Klasse“: „Klasse 1b“ → „1b“. Für Texte,
 * die das Wort selbst tragen (Übersetzungen, Mehrzahl „Klassen 1a, 1b“).
 */
export function schoolClassName(klass: string): string {
  return klass.trim().replace(LEADING_PREFIX, "");
}

function schoolClassNameForPlural(klass: string): string {
  const name = schoolClassName(klass);
  return name === "" ? schoolClassLabel(klass) : name;
}

/**
 * Mehrere Klassen in einer Zeile: eine Klasse wie `schoolClassLabel`,
 * mehrere als „Klassen 1a, 1b“. `join` setzt die Liste zusammen
 * (Standard: Komma).
 */
export function schoolClassesLabel(
  classes: readonly string[],
  join: (names: string[]) => string = (names) => names.join(", "),
): string {
  const names = classes.map((name) => name.trim()).filter(Boolean);
  if (names.length === 0) return "";
  if (names.length === 1) return schoolClassLabel(names[0] ?? "");
  return `Klassen ${join(names.map(schoolClassNameForPlural))}`;
}
