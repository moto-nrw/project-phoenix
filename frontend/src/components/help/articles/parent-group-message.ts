import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/messages/groups`: Gruppennachrichten (#4784). Die OGS schreibt an eine
 * Gruppe, Klasse oder AG; Eltern antworten nur der OGS.
 */
export function parentGroupMessageTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentGroupMessage,
    title: "Einer Gruppe eine Nachricht schreiben",
    question: "Wie schreibe ich allen Eltern einer Gruppe?",
    summary:
      "Schreiben Sie allen Eltern einer Gruppe, Klasse oder AG. Jede Familie antwortet nur der OGS.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "UsersThree",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Gruppennachricht beginnen",
        steps: [
          "Öffnen Sie unter `Eltern` den Bereich `Nachrichten`.",
          "Wählen Sie den Reiter `Gruppennachrichten`.",
          "Wählen Sie `Neue Gruppennachricht`.",
          "Wählen Sie `Gruppe`, `Klasse` oder `AG` und dann den Namen.",
          "Prüfen Sie, wie viele Familien die Nachricht bekommen.",
          "Geben Sie einen Titel und Ihre Nachricht ein.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Eine weitere Nachricht schreiben",
        steps: [
          "Öffnen Sie die Gruppennachricht in der Liste.",
          "Schreiben Sie unten Ihre Nachricht.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Eine Gruppennachricht schließen",
        description: "Danach können die Eltern nicht mehr antworten.",
        steps: [
          "Öffnen Sie die Gruppennachricht.",
          "Öffnen Sie oben das Menü mit den drei Punkten.",
          "Wählen Sie `Schließen` und bestätigen Sie.",
        ],
      },
    ],
    result:
      "Jede Familie liest die Nachricht in der Eltern-App unter `Nachrichten`. Antworten erscheinen in der Unterhaltung mit der Familie, mit dem Hinweis `Antwort auf Gruppennachricht`.",
    differences: [
      "Sehen Sie den Reiter `Gruppennachrichten` nicht? Dann ist der Nachrichtenaustausch mit den Eltern ausgeschaltet, oder Ihnen fehlt der Zugriff auf alle Kinder.",
      "Steht bei einem Kind `Kein Eltern-Zugang`? Dann bekommt diese Familie die Nachricht nicht.",
    ],
    notes: [
      "Keine Familie sieht, wer die Nachricht noch bekommt oder wer geantwortet hat.",
      "moto ermittelt die Empfänger bei jeder Nachricht neu. Wer später in die Gruppe kommt, liest nur die neuen Nachrichten.",
      "Unter `Gelesen` sehen Sie für jedes Kind, ob die Eltern die neueste Nachricht gelesen haben.",
      "Über dem Schreibfeld steht bei `Ging an:`, an wen die Nachricht ging und wie viele Familien sie bekommen haben.",
    ],
    related: [HELP_TOPICS.parentMessage, HELP_TOPICS.leadParentAnnouncement],
  };
}
