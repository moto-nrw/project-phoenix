import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/parents/messages`: je Kind eine Unterhaltung mit dem OGS-Team. */
export function parentMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentMessages,
    title: "Nachrichten lesen und schreiben",
    question: "Wie erreiche ich die OGS?",
    summary: "Sie schreiben dem OGS-Team und lesen seine Antworten.",
    group: "nachrichten",
    audience: "parent",
    icon: "MessageSquareText",
    steps: [
      "Tippen Sie unten auf `Nachrichten`.",
      "Tippen Sie auf die Unterhaltung des Kindes.",
      "Tippen Sie unten in das Feld `Nachricht an die OGS schreiben`.",
      "Tippen Sie rechts auf den Pfeil `Senden`.",
    ],
    result: "Die OGS bekommt Ihre Nachricht sofort.",
    notes: [
      "Jedes Kind hat eine eigene Unterhaltung. Oben steht, für welches Kind sie gilt.",
      "Oben und über dem Schreibfeld steht, wer mitliest, zum Beispiel `Lesen können: Sie und alle Mitarbeitenden der OGS.` Andere Bezugspersonen Ihres Kindes lesen Ihre Unterhaltung nicht.",
      "Unter Ihrer Nachricht steht `Gesendet` und später `Von der OGS gelesen`.",
    ],
    differences: [
      "Bei einem Kind steht `Für dieses Kind können Sie nicht schreiben`? Dann dürfen Sie den Verlauf nur lesen.",
      "Sehen Sie oben `Ganzes OGS-Team` und `Nur die Leitung`? Dann bietet Ihre OGS das an. Tippen Sie auf `Nur die Leitung`, wenn nur die Leitung Ihre Nachricht lesen soll. Darunter stehen die Namen der Leitung.",
      "Steht in der Unterhaltung mit der Leitung `Zurzeit liest niemand aus der Leitung mit`? Dann schreiben Sie an `Ganzes OGS-Team`. Den Verlauf können Sie weiter lesen.",
    ],
    troubleshootingDetails: [
      "moto sagt, Ihre Sitzung sei abgelaufen? Melden Sie sich neu an und schicken Sie die Nachricht erneut.",
      "Es geht um eine Abwesenheit oder eine andere Abholzeit? Nutzen Sie dafür die passende Meldung statt einer Nachricht.",
    ],
    related: [
      HELP_TOPICS.parentNews,
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}
