import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { parentAnnouncementSteps } from "../help-article-shared";

/**
 * Umfrage: Mitteilung mit Antwortmoeglichkeiten (#1371). Im Formular heisst
 * das Titelfeld deshalb `Frage` statt `Titel` (page.tsx:1363).
 */
export function parentSurveyTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentSurvey,
    title: "Eine kurze Elternumfrage erstellen",
    question: "Wie frage ich die Eltern nach ihrer Rückmeldung?",
    summary: "Eine Umfrage sammelt die Antworten der Eltern an einem Ort.",
    group: "elternarbeit",
    audience: "lead",
    icon: "ListChecks",
    steps: [
      ...parentAnnouncementSteps("Umfragen", "Umfrage", "Frage"),
      "Tragen Sie unter `Antwortmöglichkeiten` die Antworten ein. `Ja` und `Nein` stehen schon da, `+ Antwort hinzufügen` ergänzt weitere.",
      "Tragen Sie bei Bedarf ein Datum bei `Antwortfrist (optional)` ein.",
      "Wählen Sie `Veröffentlichen`.",
    ],
    result: "Die Eltern sehen die Frage in moto und können antworten.",
    notes: [
      "Mit `Mehrfachauswahl erlauben` dürfen Eltern mehrere Antworten wählen.",
      "Die Ergebnisse sehen Sie später beim Öffnen der Umfrage.",
      "Zwei bis 60 Antworten sind möglich. Eltern antworten für jedes Kind einzeln.",
      "Viele Termine zur Auswahl? Kopieren Sie die Liste, zum Beispiel aus einer Tabelle. Fügen Sie sie in ein Antwortfeld ein. Jede Zeile wird eine eigene Antwort.",
    ],
    differences: ["Das erste Feld heißt hier `Frage`, nicht `Titel`."],
    troubleshootingDetails: [
      "Brauchen Sie keine Antwort? Nehmen Sie stattdessen eine Mitteilung.",
      "Mehrere Fragen, eine Skala oder eine anonyme Befragung? Dafür gibt es den Bereich `Umfragen`.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentLetter,
      HELP_TOPICS.leadSurveys,
      HELP_TOPICS.parentRequests,
    ],
  };
}
