// Abweichungen vom üblichen Plan, klassenübergreifend (#2294).
//
// Die Klassenansicht zeigt den ganzen Tag jeder Klasse. Was sie bisher nicht
// zeigte: was heute ANDERS ist als sonst. Genau das ist die Information, die
// eine Lehrkraft für die Entscheidung braucht, ob sie ein Kind früher gehen
// lässt, und die heute manuell über das OGS-Team läuft.
//
// Diese Datei enthält nur die Auswahl und die Reihenfolge — kein React, kein
// Fetch —, damit die Regel ("was gilt als Abweichung") einzeln prüfbar ist.

import type { ClassDayReport, ClassDayRow } from "~/lib/class-day-api";
import { berlinTodayISO } from "~/lib/date-helpers";

export type DayChangeKind = "status" | "pickup";

export interface DayChange {
  /** Stabiler Listen-Key über alle Klassen hinweg. */
  readonly key: string;
  readonly schoolClass: string;
  readonly row: ClassDayRow;
  /**
   * `status` = gemeldete Abwesenheit oder Abmeldung, `pickup` = die Betreuung
   * läuft, aber die Abholzeit weicht ab. Der Status ist die stärkere Aussage
   * und gewinnt, wenn beides zutrifft.
   */
  readonly kind: DayChangeKind;
  /** ISO-Zeitstempel, seit wann die Abweichung bekannt ist. */
  readonly reportedAt: string | null;
}

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
 * Sammelt die Abweichungen aller geladenen Klassen in einer Liste.
 *
 * `classes` gibt die Reihenfolge vor, damit die Liste sich nicht bei jedem
 * Abruf umsortiert; fehlgeschlagene Klassen fehlen schlicht (der
 * Teilausfall-Banner der Ansicht trägt diese Aussage bereits).
 */
export function collectDayChanges(
  classes: readonly string[],
  reports: Readonly<Record<string, ClassDayReport>>,
): DayChange[] {
  const changes: DayChange[] = [];
  for (const schoolClass of classes) {
    const report = reports[schoolClass];
    if (!report?.school_day) continue;
    for (const row of report.rows) {
      if (!isDayChange(row)) continue;
      changes.push({
        key: `${schoolClass}-${row.student_id}`,
        schoolClass,
        row,
        kind: row.status ? "status" : "pickup",
        reportedAt: row.reported_at ?? null,
      });
    }
  }
  return changes.sort(compareDayChanges);
}

/**
 * Zuletzt gemeldet zuerst: die kurzfristige Meldung von heute Vormittag ist
 * die, die die Lehrkraft noch nicht kennt. Zeilen ohne Zeitstempel hängen
 * hinten, danach entscheidet Klasse und Name — nie die Abrufreihenfolge, sonst
 * springt die Liste bei jeder Aktualisierung.
 */
function compareDayChanges(a: DayChange, b: DayChange): number {
  if (a.reportedAt !== b.reportedAt) {
    if (a.reportedAt === null) return 1;
    if (b.reportedAt === null) return -1;
    if (a.reportedAt > b.reportedAt) return -1;
    if (a.reportedAt < b.reportedAt) return 1;
  }
  if (a.schoolClass !== b.schoolClass) {
    return a.schoolClass.localeCompare(b.schoolClass, "de");
  }
  const lastName = a.row.last_name.localeCompare(b.row.last_name, "de");
  if (lastName !== 0) return lastName;
  return a.row.first_name.localeCompare(b.row.first_name, "de");
}

/**
 * War die Meldung kurzfristig? Kurzfristig heißt: sie kam am heutigen
 * Kalendertag herein, unabhängig davon, welcher Tag gerade angezeigt wird.
 * Berlin-Kalendertag auf beiden Seiten, damit die Aussage nicht von der
 * Zeitzone des Browsers abhängt.
 */
export function isReportedToday(
  reportedAt: string | null,
  now: Date = new Date(),
): boolean {
  if (!reportedAt) return false;
  const stamp = new Date(reportedAt);
  if (Number.isNaN(stamp.getTime())) return false;
  return berlinTodayISO(stamp) === berlinTodayISO(now);
}

/**
 * Der Satz, der die Abweichung benennt. Bei einer geänderten Abholzeit nennt
 * er beide Zeiten: ohne die Regelzeit daneben liest sich "geht um 12:15" wie
 * der Normalfall, und genau diese Fehllesung soll der Block verhindern.
 */
export function describeDayChange(
  change: DayChange,
  statusLabelOf: (status: string) => string,
): string {
  if (change.kind === "status") {
    return statusLabelOf(change.row.status ?? "");
  }
  const pickup = change.row.pickup ?? "";
  const regular = change.row.pickup_regular ?? "";
  if (!pickup) return "Abholzeit geändert";
  if (!regular) return `Geht um ${pickup} Uhr`;
  return `Geht um ${pickup} Uhr statt um ${regular} Uhr`;
}
