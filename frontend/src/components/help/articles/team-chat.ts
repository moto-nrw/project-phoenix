import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function teamChatTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teamChat,
    title: "Den Team-Chat nutzen",
    question: "Wie nutze ich den Team-Chat?",
    summary:
      "Schreiben Sie einer Betreuungskraft oder Lehrkraft Ihrer Schule eine Nachricht.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "ChatsCircle",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Team-Chat`.",
      "Wählen Sie `Neue Nachricht`.",
      "Suchen Sie die Person und wählen Sie sie aus.",
      "Schreiben Sie Ihre Nachricht.",
      "Wählen Sie `Senden`.",
    ],
    result:
      "Die Nachricht erscheint in Ihrer Unterhaltung mit dieser Person. Lehrkräfte lesen sie in moto schule.",
    differences: [
      "Nachrichten im Team-Chat können nicht nachträglich geändert oder gelöscht werden.",
      "Der Bereich fehlt? Dann hat Ihre OGS den Team-Chat nicht eingeschaltet.",
    ],
    notes: [
      "Eltern können den Team-Chat nicht sehen. Für Eltern nutzen Sie `Nachrichten` im Bereich `Eltern`.",
      "Mit `Nur ungelesen` sehen Sie nur Unterhaltungen mit neuen Nachrichten.",
    ],
    related: [HELP_TOPICS.findStaff, HELP_TOPICS.parentMessage],
  };
}
