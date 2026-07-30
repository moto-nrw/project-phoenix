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
 * Die Daten kommen aus derselben Dienstplan-Übersicht, die auch das
 * Wochenraster des Dienstplans rendert (`coverageStatus === "uncovered"`,
 * berechnet in backend/services/schedule/staff_schedule_overview.go). Wer die
 * Übersicht nicht lesen darf oder eine Woche ohne gepflegten Dienstplan
 * ansieht, bekommt gar keinen Hinweis — der Aufrufer entscheidet das, diese
 * Komponente rendert nur.
 */

import Link from "next/link";

import { Alert } from "~/components/ui/alert";
import { parseISODate } from "~/lib/date-helpers";
import type { StaffScheduleAssignment } from "~/lib/shift-helpers";
import { getGermanWeekdayShort, staffLabel } from "~/lib/timetable-helpers";

/** Wie viele Einsätze die Meldung namentlich nennt, bevor sie zusammenfasst. */
const NAMED_PREVIEW_LIMIT = 2;

export interface VertretungCoverageNoticeProps {
  /**
   * Die nicht abgedeckten Einsätze des angezeigten Zeitraums. Der Aufrufer
   * filtert (uncovered, Zeitraum, keine Vergangenheit); leer heißt: nichts
   * rendern.
   */
  readonly assignments: readonly StaffScheduleAssignment[];
  /** Anzeigenamen fürs Personal, wie in der Störungsliste. */
  readonly staffNames: Map<string, string>;
  /** "an diesem Tag" oder "in dieser Woche" — der angezeigte Zeitraum. */
  readonly scopeLabel: string;
  /** Wochenansicht: den Wochentag mitnennen, weil mehrere Tage gemischt sind. */
  readonly withWeekday: boolean;
  /** Ziel des Links "Dienstplan öffnen" (tenant-aware, vom Aufrufer gebaut). */
  readonly dienstplanHref: string;
}

/** "Ada Staff (12:30–14:00)" bzw. "Ada Staff (Mi, 12:30–14:00)". */
function assignmentPreview(
  assignment: StaffScheduleAssignment,
  staffNames: Map<string, string>,
  withWeekday: boolean,
): string {
  const name = staffLabel(staffNames, assignment.staffId);
  // Der offene Zeitraum ist die eigentliche Information; ohne Intervall (das
  // Backend liefert für "uncovered" immer mindestens eines) fällt die Meldung
  // auf das Einsatzfenster zurück, statt eine leere Klammer zu zeigen.
  const interval = assignment.uncoveredIntervals[0];
  const range = interval
    ? `${interval.startTime}–${interval.endTime}`
    : `${assignment.startTime}–${assignment.endTime}`;
  const weekday = withWeekday
    ? `${getGermanWeekdayShort(parseISODate(assignment.date))}, `
    : "";
  return `${name} (${weekday}${range})`;
}

/**
 * Baut den Meldungstext. Exportiert, weil die Pluralisierung und die
 * Zusammenfassung der Rest-Einsätze die eigentliche Logik dieser Komponente
 * sind und direkt getestet werden.
 */
export function buildCoverageNoticeMessage(
  assignments: readonly StaffScheduleAssignment[],
  staffNames: Map<string, string>,
  scopeLabel: string,
  withWeekday: boolean,
): string {
  const count = assignments.length;
  const lead =
    count === 1
      ? `1 Einsatz ${scopeLabel} ist nicht durch den Dienstplan abgedeckt`
      : `${count} Einsätze ${scopeLabel} sind nicht durch den Dienstplan abgedeckt`;
  const named = assignments
    .slice(0, NAMED_PREVIEW_LIMIT)
    .map((assignment) =>
      assignmentPreview(assignment, staffNames, withWeekday),
    );
  const rest = count - named.length;
  const restLabel =
    rest === 1 ? "1 weiterer Einsatz" : `${rest} weitere Einsätze`;
  const detail =
    rest > 0 ? `${named.join(", ")} und ${restLabel}` : named.join(", ");
  return `${lead}: ${detail}.`;
}

export function VertretungCoverageNotice({
  assignments,
  staffNames,
  scopeLabel,
  withWeekday,
  dienstplanHref,
}: VertretungCoverageNoticeProps) {
  if (assignments.length === 0) return null;

  return (
    <Alert
      type="warning"
      // Kein "assertive": der Hinweis ist keine Störung und darf einen
      // Screenreader nicht unterbrechen.
      announce="polite"
      message={buildCoverageNoticeMessage(
        assignments,
        staffNames,
        scopeLabel,
        withWeekday,
      )}
      action={
        <Link
          href={dienstplanHref}
          className="font-medium whitespace-nowrap underline underline-offset-2 hover:no-underline"
        >
          Dienstplan öffnen
        </Link>
      }
    />
  );
}
