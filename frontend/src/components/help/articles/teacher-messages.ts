import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/nachrichten`: derselbe Team-Chat wie in der OGS, optional. */
export function teacherMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherMessages,
    title: "Nachrichten an die OGS schreiben",
    question: "Wie erreiche ich die OGS?",
    summary: "Sie schreiben dem OGS-Team und lesen seine Antworten.",
    group: "nachrichten",
    audience: "teacher",
    icon: "MessageSquareText",
    steps: [
      "Öffnen Sie `Nachrichten`.",
      "Wählen Sie eine Unterhaltung. Oder wählen Sie `Neue Nachricht`.",
      "Schreiben Sie Ihre Nachricht.",
      "Senden Sie sie ab.",
    ],
    result: "Das OGS-Team bekommt Ihre Nachricht sofort.",
    notes: [
      "Vor Ihrer eigenen Nachricht steht `Sie:`.",
      "Hier melden Sie, was die OGS wissen muss, zum Beispiel eine falsche Abholzeit.",
      "Die OGS kann Sie in einen Gruppenchat aufnehmen. Er steht dann mit seinem Namen in Ihrer Liste. Mit `Mitglieder` und `Gruppenchat verlassen` gehen Sie wieder heraus.",
    ],
    differences: [
      "Fehlt `Nachrichten` ganz? Dann nutzt Ihre OGS diese Funktion nicht.",
    ],
    troubleshootingDetails: [
      "Geht es um die Ankunftszeit Ihrer Klasse? Tragen Sie die lieber direkt ein, wenn Ihre OGS das freigegeben hat.",
    ],
    related: [
      HELP_TOPICS.teacherNotices,
      HELP_TOPICS.teacherSettings,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}
