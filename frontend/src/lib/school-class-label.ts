/**
 * Beschriftung einer Schulklasse.
 *
 * Klassennamen sind Freitext: manche Schulen speichern "1a", andere schon
 * "Klasse 1a". Beide dürfen nicht zu "Klasse Klasse 1a" werden. Geteilt
 * zwischen Klassenansicht und Aufsichten (#2527), damit die zwei Ansichten
 * desselben Portals dieselbe Klasse gleich schreiben.
 */
export function schoolClassLabel(klass: string): string {
  const trimmed = klass.trim();
  if (trimmed === "") return "";
  return /^klasse\b/i.test(trimmed) ? trimmed : `Klasse ${trimmed}`;
}
