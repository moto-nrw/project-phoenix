"use client";

/**
 * Hinweis "nicht durch den Dienstplan abgedeckt" über der Störungsliste des
 * Vertretungsbereichs.
 *
 * Warum hier und nicht als Störung: die Störungsdefinition
 * (vertretung-day-list.tsx) ist bewusst vierteilig — offene Lücke, quittiert,
 * abgesagt, Abwesenheit. Eine fehlende Schicht ist keine davon: die Person
 * fällt nicht aus, Betreuungsplan und Dienstplan widersprechen sich nur. Der
 * Hinweis zählt daher NICHT in die Zähler "Offen"/"Quittiert" und nicht in die
 * Tageschips, sondern steht als eigene Zeile darüber und verlinkt in den
 * Dienstplan, wo die Schicht nachgetragen wird.
 *
 * Absichtlich nur die Anzahl: welche Person in welchem Zeitfenster fehlt, sagt
 * der Dienstplan pro Einsatz genau ("Nicht abgedeckt: 12:30–14:00"), und dorthin
 * führt der Link. Der Hinweis muss das nicht doppeln, er muss nur auffallen.
 * Deshalb ist der Text in Tages- und Wochenansicht derselbe.
 *
 * Die Daten kommen aus derselben Dienstplan-Übersicht, die auch das
 * Wochenraster des Dienstplans rendert (`coverageStatus === "uncovered"`,
 * berechnet in backend/services/schedule/staff_schedule_overview.go). Wer die
 * Übersicht nicht lesen darf oder eine Woche ohne gepflegten Dienstplan
 * ansieht, bekommt gar keinen Hinweis — der Aufrufer entscheidet das, diese
 * Komponente rendert nur.
 */

import Link from "~/components/ui/navigation-link";

import { Alert } from "~/components/ui/alert";

export interface VertretungCoverageNoticeProps {
  /**
   * Anzahl der nicht abgedeckten Einsätze im angezeigten Zeitraum. Der Aufrufer
   * filtert (uncovered, Zeitraum, keine Vergangenheit); 0 heißt: nichts
   * rendern.
   */
  readonly count: number;
  /** Ziel des Links "Dienstplan öffnen" (tenant-aware, vom Aufrufer gebaut). */
  readonly dienstplanHref: string;
}

/** Exportiert, weil die Pluralisierung direkt getestet wird. */
export function buildCoverageNoticeMessage(count: number): string {
  return count === 1
    ? "1 Einsatz ist nicht durch den Dienstplan abgedeckt."
    : `${count} Einsätze sind nicht durch den Dienstplan abgedeckt.`;
}

export function VertretungCoverageNotice({
  count,
  dienstplanHref,
}: VertretungCoverageNoticeProps) {
  if (count <= 0) return null;

  return (
    // Weißer Unterbau: die getönte Alert-Fläche ist absichtlich
    // halbtransparent, und das Punktraster der Planungsseiten schiene sonst
    // durch den Hinweis durch.
    <div className="rounded-lg bg-white">
      <Alert
        type="warning"
        // Kein "assertive": der Hinweis ist keine Störung und darf einen
        // Screenreader nicht unterbrechen.
        announce="polite"
        message={buildCoverageNoticeMessage(count)}
        action={
          <Link
            href={dienstplanHref}
            className="font-medium whitespace-nowrap underline underline-offset-2 hover:no-underline"
          >
            Dienstplan öffnen
          </Link>
        }
      />
    </div>
  );
}
