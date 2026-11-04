import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Kindblatt in der laufenden Aufsicht: Abholung und Notfallkontakt. */
export function teacherSupervisionRosterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSupervisionRoster,
    title: "Die Kinder meiner Aufsicht sehen",
    question: "Wer darf ein Kind abholen und wen rufe ich im Notfall an?",
    summary:
      "In einer laufenden Aufsicht sehen Sie Abholung und Notfallkontakt.",
    group: "aufsicht",
    audience: "teacher",
    icon: "User",
    steps: [
      "Starten Sie Ihre Aufsicht.",
      "Wählen Sie ein Kind aus der Liste.",
      "Lesen Sie unter `Heute` die Zeiten des Kindes.",
      "Unter `Darf abholen` steht, wer es abholen darf.",
      "Unter `Im Notfall anrufen` stehen die Notfallkontakte.",
    ],
    result:
      "Sie können ein Kind sicher übergeben, ohne im OGS-Team nachzufragen.",
    notes: [
      "Diese Angaben sehen Sie nur zu Kindern Ihrer laufenden Aufsicht.",
      "Jeder Zugriff wird protokolliert.",
    ],
    differences: [
      "In der `Klassenansicht` fehlen `Darf abholen` und `Im Notfall anrufen`. Das ist Absicht.",
    ],
    troubleshootingDetails: [
      "Ein Kontakt fehlt oder stimmt nicht? Melden Sie das dem OGS-Team. Ändern können Sie es hier nicht.",
    ],
    related: [
      HELP_TOPICS.teacherStartSupervision,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherSupervision,
    ],
  };
}
