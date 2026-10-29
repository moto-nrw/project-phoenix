import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Jahrgangswechsel unter `/database/grade-transitions`. Nur am Computer: die
 * Seite rendert ihren Inhalt erst ab `lg` (page.tsx:69) und zeigt darunter
 * einen Hinweis. Braucht `grade_transitions:read`, das Anwenden zusaetzlich
 * `grade_transitions:apply`.
 */
export function gradeTransitionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadGradeTransition,
    title: "Kinder in den nächsten Jahrgang übernehmen",
    question: "Wie übernehme ich die Kinder in das neue Schuljahr?",
    summary: "moto schlägt für jede Klasse die nächste vor. Sie prüfen nur.",
    group: "kinderdaten",
    audience: "lead",
    icon: "ArrowUpRight",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Schuljahreswechsel`.",
      "Wählen Sie `Neuer Schuljahreswechsel`.",
      "Tragen Sie bei `Schuljahr (neu)` das neue Schuljahr ein, zum Beispiel `2026-2027`.",
      "Tragen Sie bei Bedarf eine `Notiz (optional)` ein.",
      "Prüfen Sie die vorgeschlagenen Klassen und passen Sie sie an.",
      "Wählen Sie `Weiter zur Vorschau`.",
      "Prüfen Sie die Vorschau.",
      "Wählen Sie `Schuljahreswechsel anwenden`. Bestätigen Sie mit `Ja, anwenden`.",
    ],
    result:
      "Alle Kinder stehen in der neuen Klasse. Abgänge sind aus der Betreuung heraus.",
    notes: [
      "moto schlägt jede Klasse automatisch vor, zum Beispiel `1a` in `2a`.",
      "Ein Entwurf bleibt liegen. Über `Zurück zum Entwurf` arbeiten Sie später weiter.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Der Schuljahreswechsel läuft am Computer.",
      "Ohne das Recht zum Anwenden sehen Sie die Vorschau, aber keinen Knopf zum Anwenden.",
    ],
    troubleshootingDetails: [
      "Die Vorschau meldet geänderte Daten? Dann hat jemand zwischendurch Klassen geändert. Prüfen Sie erneut und bestätigen Sie dann.",
      "Fehlt `Schuljahreswechsel`? Dann fehlt Ihnen das Recht dafür.",
    ],
    related: [
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadCalendarPeriods,
    ],
  };
}
