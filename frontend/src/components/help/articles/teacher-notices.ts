import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/tagesinformationen`: Hinweise der OGS-Leitung, teils zu bestaetigen. */
export function teacherNoticesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherNotices,
    title: "Tagesinformationen lesen",
    question: "Wo stehen die Hinweise der OGS-Leitung?",
    summary: "Unter `Tagesinformationen` steht, was für heute gilt.",
    group: "nachrichten",
    audience: "teacher",
    icon: "Megaphone",
    steps: [
      "Öffnen Sie `Tagesinformationen`.",
      "Lesen Sie die Hinweise für heute.",
      "Wählen Sie bei Bedarf `Zur Kenntnis nehmen`.",
    ],
    result: "Bestätigte Hinweise tragen danach `Zur Kenntnis genommen`.",
    notes: [
      "Sie sehen nur Hinweise, die für Lehrkräfte oder für alle gelten.",
      "Ein Zähler neben dem Eintrag zeigt, wie viele Hinweise offen sind.",
    ],
    differences: [
      "Nicht jeder Hinweis verlangt eine Bestätigung. Dann fehlt der Knopf.",
    ],
    troubleshootingDetails: [
      "moto meldet, die Kenntnisnahme sei nicht gespeichert worden? Versuchen Sie es noch einmal.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherSettings,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}
