import {
  berlinDateTimeISO,
  berlinTodayISO,
  parseISODate,
} from "../../src/lib/date-helpers";

// Der Referenzzeitpunkt der Produkt-Screenshots (#3759): ein Werktag, 13:30 Uhr
// Berlin, mitten im Nachmittag der OGS. Die Browser-Uhr steht bei jedem Bild
// darauf, damit Datum und Uhrzeit überall gleich und plausibel sind. Der
// Workflow stellt die Uhr der VM vor dem Seed auf Dienstag 12:00; Anwesenheit
// und Nachrichten tragen dann Zeiten von kurz nach 12 Uhr, wenn die Kinder
// laut Wochenplan nach dem Unterricht ankommen.
const REFERENCE_CLOCK = "13:30";

function isWeekend(day: Date): boolean {
  return day.getDay() === 0 || day.getDay() === 6;
}

/**
 * Der heutige Berliner Werktag um 13:30, auch wenn 13:30 noch bevorsteht.
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
