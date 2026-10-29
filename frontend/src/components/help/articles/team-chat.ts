import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function teamChatTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teamChat,
    title: "Den Team-Chat nutzen",
    question: "Wie nutze ich den Team-Chat?",
    summary:
      "Schreiben Sie einer Person oder mehreren Personen Ihrer Schule, auch Lehrkräften.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "ChatsCircle",
    steps: [],
    instructionGroups: [
      {
        title: "Einer Person schreiben",
        ordered: true,
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Team-Chat`.",
          "Wählen Sie `Neue Nachricht`.",
          "Suchen Sie die Person und wählen Sie sie aus.",
          "Schreiben Sie Ihre Nachricht.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Einen Gruppenchat anlegen",
        description:
          "Ein Gruppenchat hat einen Namen und mehrere Mitglieder, auch Lehrkräfte.",
        ordered: true,
        steps: [
          "Öffnen Sie `Team-Chat`.",
          "Wählen Sie `Neuer Gruppenchat`.",
          "Geben Sie einen Namen ein, zum Beispiel `Frühbetreuung`.",
          "Kreuzen Sie die Personen an. Sie selbst sind automatisch dabei.",
          "Wählen Sie `Gruppenchat anlegen`.",
        ],
      },
      {
        title: "Mitglieder ändern oder den Gruppenchat verlassen",
        description:
          "Mitglieder und Namen ändern die Person, die den Gruppenchat angelegt hat, und die OGS-Leitung, wenn sie Mitglied ist.",
        ordered: true,
        steps: [
          "Öffnen Sie den Gruppenchat.",
          "Wählen Sie oben `Mitglieder`.",
          "Ändern Sie den Namen, wählen Sie `Personen hinzufügen` oder entfernen Sie eine Person über das Menü neben ihrem Namen.",
          "Mit `Gruppenchat verlassen` gehen Sie selbst aus dem Gruppenchat.",
        ],
      },
    ],
    result:
      "Die Nachricht erscheint in der Unterhaltung. Lehrkräfte lesen sie in moto schule.",
    differences: [
      "Nachrichten im Team-Chat können nicht nachträglich geändert oder gelöscht werden.",
      "Der Bereich fehlt? Dann hat Ihre OGS den Team-Chat nicht eingeschaltet.",
      "Neue Mitglieder eines Gruppenchats sehen die Nachrichten ab ihrem Beitritt, nicht die älteren.",
    ],
    notes: [
      "Unter Ihrer Nachricht in einem Gruppenchat steht, wie viele sie gelesen haben. Tippen Sie darauf, dann sehen Sie die Namen.",
      "Eltern können den Team-Chat nicht sehen. Für Eltern nutzen Sie `Nachrichten` im Bereich `Eltern`.",
      "Mit `Nur ungelesen` sehen Sie nur Unterhaltungen mit neuen Nachrichten.",
    ],
    related: [HELP_TOPICS.findStaff, HELP_TOPICS.parentMessage],
  };
}
