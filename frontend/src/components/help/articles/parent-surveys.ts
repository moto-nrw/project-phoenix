import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/news`: eine Umfrage mit Fragebogen der OGS beantworten (#4781).
 * Eine Antwort je Elternkonto, nicht je Kind.
 */
export function parentSurveysTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentSurveys,
    title: "An einer Umfrage teilnehmen",
    question: "Was bedeutet eine anonyme Umfrage?",
    summary:
      "Die OGS fragt nach Ihrer Meinung. Vor der ersten Frage steht, ob die Umfrage anonym ist.",
    group: "nachrichten",
    audience: "parent",
    icon: "CheckSquare",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Elternbriefe`.",
      "Tippen Sie unter `Offen` auf den Eintrag mit `Umfrage`.",
      "Lesen Sie den Hinweis `Anonym` oder `Nicht anonym`.",
      "Beantworten Sie die Fragen.",
      "Tippen Sie auf `Antwort absenden`.",
    ],
    result:
      "Sie sehen `Danke, Ihre Antwort ist gespeichert.` Der Eintrag steht danach mit `Teilgenommen` unter `Erledigt`.",
    notes: [
      "`Anonym` heißt: Die Schule sieht nicht, wer was geantwortet hat. Sie sieht nur Summen, erst nach dem Ende und ab 5 Antworten.",
      "`Nicht anonym` heißt: Die Schule sieht Ihren Namen bei Ihren Antworten.",
      "Sie antworten einmal für Ihr Elternkonto, nicht für jedes Kind.",
      "Nach dem Absenden können Sie Ihre Antwort nicht mehr ändern.",
    ],
    differences: [
      "Steht dort `Die Teilnahme ist beendet.`? Dann ist die Frist vorbei. Antworten geht nicht mehr.",
    ],
    image: "/help/screens/umfrage-eltern.webp",
    imageAlt:
      "Umfrage im Elternportal. Vor der ersten Frage steht, ob sie anonym ist.",
    related: [HELP_TOPICS.parentNews, HELP_TOPICS.parentNotifications],
  };
}
