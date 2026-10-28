import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/messages/group/…`: Gruppennachrichten der OGS lesen und
 * beantworten (#4784). Die Antwort geht nur an die OGS.
 */
export function parentGroupMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentGroupMessages,
    title: "Gruppennachrichten lesen und beantworten",
    question: "Was ist eine Gruppennachricht?",
    summary:
      "Die OGS schreibt allen Eltern einer Gruppe, Klasse oder AG. Ihre Antwort lesen nur die OGS.",
    group: "nachrichten",
    audience: "parent",
    icon: "UsersThree",
    steps: [
      "Tippen Sie unten auf `Nachrichten`.",
      "Tippen Sie unter `Gruppennachrichten der OGS` auf die Nachricht.",
      "Tippen Sie unter `Antwort an die OGS` in das Feld `Antwort an die OGS schreiben`.",
      "Tippen Sie rechts auf den Pfeil `Senden`.",
    ],
    result:
      "Ihre Antwort steht danach im Chat mit der OGS zu Ihrem Kind. Andere Eltern sehen sie nicht.",
    notes: [
      "Sie sehen nicht, welche anderen Eltern die Nachricht bekommen.",
      "Haben mehrere Ihrer Kinder die Nachricht bekommen? Dann wählen Sie vor dem Senden das Kind.",
    ],
    differences: [
      "Steht dort `Diese Gruppennachricht ist geschlossen`? Dann können Sie nicht mehr antworten. Schreiben Sie der OGS im Chat zu Ihrem Kind.",
    ],
    related: [HELP_TOPICS.parentMessages, HELP_TOPICS.parentNotifications],
  };
}
