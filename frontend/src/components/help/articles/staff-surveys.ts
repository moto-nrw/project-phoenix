import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/umfragen`: Umfragen der Schule an das Team beantworten (#4781). Ob die
 * Umfrage anonym ist, steht vor der ersten Frage.
 */
export function staffSurveysTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.staffSurveys,
    title: "An einer Umfrage im Team teilnehmen",
    question: "Wie beantworte ich eine Umfrage meiner Schule?",
    summary:
      "Ihre Schule kann dem Team Fragen stellen. Sie antworten einmal, auf Wunsch der Schule auch anonym.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "CheckSquare",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Umfragen`.",
      "Wählen Sie eine Umfrage mit `Antwort offen`.",
      "Lesen Sie oben, ob die Umfrage `Anonym` oder `Nicht anonym` ist.",
      "Beantworten Sie die Fragen. Pflichtfragen sind markiert.",
      "Wählen Sie `Antwort absenden`.",
    ],
    result:
      "Sie sehen `Danke, Ihre Antwort ist gespeichert.` Die Umfrage steht danach mit `Teilgenommen` in der Liste.",
    notes: [
      "`Anonym` heißt: Die Schule sieht nicht, wer was geantwortet hat. Sie sieht nur Summen, erst nach dem Ende und ab 5 Antworten.",
      "`Nicht anonym` heißt: Die Schule sieht Ihren Namen bei Ihren Antworten.",
      "Nach dem Absenden können Sie Ihre Antwort nicht mehr ändern.",
      "Die Schule erfährt nie, wer noch nicht teilgenommen hat. Eine Erinnerung geht an alle, die noch offen sind.",
    ],
    differences: [
      "Sehen Sie die Reiter `Alle Umfragen` und `Für Sie`? Dann dürfen Sie Umfragen auch anlegen. Ihre eigenen offenen Umfragen stehen unter `Für Sie`.",
      "Steht dort `Keine offenen Umfragen`? Dann hat Ihre Schule gerade keine Umfrage an das Team.",
    ],
    image: "/help/screens/umfrage-beantworten.webp",
    imageAlt:
      "Fragebogen einer anonymen Umfrage. Oben steht, dass die Schule nicht sieht, wer was geantwortet hat.",
    related: [HELP_TOPICS.teamChat, HELP_TOPICS.leadSurveys],
  };
}
