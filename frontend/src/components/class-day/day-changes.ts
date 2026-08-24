// Abweichungen vom üblichen Plan (#2294).
//
// Die Klassenansicht zeigt den ganzen Tag einer Klasse. Was sie nicht zeigte:
// was heute ANDERS ist als sonst. Genau das braucht eine Lehrkraft für die
// Entscheidung, ob sie ein Kind früher gehen lässt, und genau das läuft heute
// manuell über das OGS-Team.
//
// Die Abweichung wird am Kind selbst gekennzeichnet, nicht in einem eigenen
// Block: jedes Kind steht genau einmal auf der Seite. Über die Klassen hinweg
// trägt die Klassenzeile der Übersicht nur die Anzahl — sie beantwortet die
// eine Frage, die man nicht durch Lesen einer Liste beantworten kann: welche
// Klasse muss ich überhaupt aufmachen.
//
// Diese Datei enthält nur die Regel und die Zählung, kein React, kein Fetch.

import type { ClassDayReport, ClassDayRow } from "~/lib/class-day-api";
import { berlinTodayISO, formatChatClockTime } from "~/lib/date-helpers";

/**
 * Eine Zeile weicht ab, wenn ein Tagesstatus gemeldet ist (krank,
 * entschuldigt, Klassenfahrt, abgemeldet) oder die Abholzeit heute von der
 * Regelzeit abweicht.
 *
 * Klassenlisteneinträge (#2382) sind ausgenommen: ein Kind ohne
 * OGS-Datensatz hat keinen üblichen Plan, von dem es abweichen könnte.
 */
export function isDayChange(row: ClassDayRow): boolean {
  if (row.list_entry) return false;
  return Boolean(row.status) || row.pickup_changed === true;
}

/**
 * Anzahl der abweichenden Kinder einer Klasse an dem Tag. Ein Tag ohne
 * Übergabe (Wochenende) hat keine Abweichungen, nicht "unbekannt viele".
 */
export function countDayChanges(report: ClassDayReport | null): number {
  if (!report?.school_day) return 0;
  return report.rows.filter(isDayChange).length;
}

/**
 * War die Meldung kurzfristig? Kurzfristig heißt: sie kam am heutigen
 * Kalendertag herein, unabhängig davon, welcher Tag gerade angezeigt wird.
 * Berlin-Kalendertag auf beiden Seiten, damit die Aussage nicht von der
 * Zeitzone des Browsers abhängt.
 */
export function isReportedToday(
  reportedAt: string | null | undefined,
  now: Date = new Date(),
): boolean {
  if (!reportedAt) return false;
  const stamp = new Date(reportedAt);
  if (Number.isNaN(stamp.getTime())) return false;
  return berlinTodayISO(stamp) === berlinTodayISO(now);
}

/**
 * "Heute 09:24 gemeldet" — und sonst nichts.
 *
 * Der Zeitpunkt erscheint nur bei Meldungen von heute: genau dann ist er eine
 * Neuigkeit. Bei einer vor zwei Wochen geplanten Klassenfahrt beantwortet er
 * keine Frage und würde die Zeile nur länger machen.
 */
export function reportedTodayLabel(
  reportedAt: string | null | undefined,
  now: Date = new Date(),
): string | null {
  if (!isReportedToday(reportedAt, now)) return null;
  return `Heute ${formatChatClockTime(reportedAt!)} gemeldet`;
}
