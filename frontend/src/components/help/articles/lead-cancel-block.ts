import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Einen einzelnen Termin im Betreuungsplan absagen (#2601, #3731). Ob die
 * Eltern Bescheid bekommen, entscheidet die absagende Person im Dialog; es
 * gibt dafür keine Schuleinstellungen mehr. Beschriftungen live geprüft.
 */
export function leadCancelBlockTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCancelBlock,
    title: "Einen Termin absagen",
    question: "Wie sage ich einen Termin ab und informiere die Eltern?",
    summary:
      "Ein abgesagter Termin bleibt im Plan sichtbar. Die Eltern der eingetragenen Kinder können Sie dabei gleich informieren.",
    group: "planung",
    audience: "lead",
    icon: "CalendarCheck",
    requirements: ["Sie dürfen den Betreuungsplan bearbeiten."],
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Betreuungsplan` und wählen Sie den Termin.",
      "Wählen Sie `Absagen`.",
      "Lassen Sie den Haken bei `Eltern per App und E-Mail informieren` gesetzt, wenn die Familien Bescheid bekommen sollen.",
      "Prüfen Sie `Betreff` und `Text an die Eltern`. moto schlägt beides vor.",
      "Wählen Sie `Absagen`.",
    ],
    result:
      "Der Termin ist als abgesagt markiert. Mit Haken bekommen die Familien eine Mitteilung im Elternportal und eine E-Mail.",
    notes: [
      "Der interne Grund der Absage geht nicht an die Eltern. Sie sehen nur Ihren Text.",
      "Ohne Haken bekommen die Eltern keine Nachricht.",
    ],
    differences: [
      "Fehlt der Haken? Dann sind für diesen Termin keine Kinder eingetragen, oder der Termin liegt in der Vergangenheit.",
      "Einzelne Familien haben E-Mails oder Hinweise in ihrem Konto abgeschaltet. Diese Familien sehen die Mitteilung nur im Elternportal.",
    ],
    related: [HELP_TOPICS.leadCarePlan, HELP_TOPICS.leadCalendarPeriods],
  };
}
