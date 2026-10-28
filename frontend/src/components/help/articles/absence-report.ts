import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/absences`: Auswertung ueber einen frei gewaehlten Zeitraum. */
export function absenceReportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadAbsenceReport,
    title: "Abwesenheiten auswerten",
    question: "Wie sehe ich, wer wann gefehlt hat?",
    summary: "Die Auswertung zeigt Krankmeldungen und Entschuldigungen.",
    group: "auswertung",
    audience: "lead",
    icon: "BellRing",
    steps: [
      "Öffnen Sie im `Tagesbetrieb` den Bereich `Alle Kinder`.",
      "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
      "Wählen Sie `Abwesenheiten`.",
      "Wählen Sie oben den Zeitraum.",
      "Filtern Sie bei Bedarf nach `Status` und `Gruppe`.",
    ],
    result:
      "Unter `Eingetragene Abwesenheitstage` steht jeder Tag mit Kind und Grund.",
    notes: [
      "Als `Status` stehen `Krank`, `Entschuldigt` und `Klassenfahrt` zur Wahl.",
      "Der Zeitraum lässt sich frei wählen. Es gibt auch fertige Zeiträume.",
      "Über die Suche finden Sie ein einzelnes Kind.",
    ],
    differences: ["Die Auswertung zeigt auch geplante Tage in der Zukunft."],
    troubleshootingDetails: [
      "Ein Tag fehlt? Prüfen Sie den gewählten Zeitraum und die Filter.",
    ],
    related: [
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
