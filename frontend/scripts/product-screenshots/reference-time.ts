import {
  berlinDateTimeISO,
  berlinTodayISO,
  parseISODate,
} from "../../src/lib/date-helpers";

// Der Referenzzeitpunkt der Produkt-Screenshots (#3759): ein Werktag, 10:15 Uhr
// Berlin. Die Browser-Uhr steht bei jedem Bild darauf, damit Datum und Uhrzeit
// überall gleich und plausibel sind. Das Seed-Profil `marketing` legt seine
// Wochenpläne relativ zu dieser Uhrzeit an (marketingReferenceClock in
// backend/seed/api); beide Werte ändern sich nur gemeinsam.
const REFERENCE_CLOCK = "10:15";

function isWeekend(day: Date): boolean {
  return day.getDay() === 0 || day.getDay() === 6;
}

/**
 * Der heutige Berliner Werktag um 10:15, auch wenn 10:15 noch bevorsteht.
 * Am Wochenende der Freitag davor.
 *
 * Warum der heutige Tag: Der Server rendert mit seiner echten Uhr, die
 * Browser-Uhr steht auf dem Referenzzeitpunkt. Liegen beide auf verschiedenen
 * Kalendertagen, stimmt das erste Server-HTML nicht mit dem Client überein
 * (Hydration-Fehler, etwa im `dateTime` der Startseite). Nur die Uhrzeit ist
 * frei wählbar, weil die Seiten sie erst im Browser füllen.
 *
 * Grenze: Am Wochenende liegt der Referenztag vor dem Serverdatum. Der Lauf
 * bricht dann wegen dieses Fehlers ab, statt ein uneinheitliches Bild
 * auszugeben. Die Live-Anwesenheit folgt ebenfalls dem Serverdatum; access.ts
 * verlangt deshalb einen Seed vom heutigen Berliner Kalendertag.
 */
export function referenceInstant(now: Date = new Date()): Date {
  let day = parseISODate(berlinTodayISO(now));
  while (isWeekend(day)) {
    day = new Date(day.getFullYear(), day.getMonth(), day.getDate() - 1);
  }
  return new Date(berlinDateTimeISO(day, REFERENCE_CLOCK));
}
