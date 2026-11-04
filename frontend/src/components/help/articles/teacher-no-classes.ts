import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Problemartikel: Klassen weist die OGS in der Personalakte zu. */
export function teacherNoClassesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherNoClasses,
    title: "Mir wird keine Klasse angezeigt",
    question: "Warum sehe ich meine Klasse nicht?",
    summary: "Die OGS muss Ihnen die Klassen erst zuweisen.",
    group: "probleme",
    audience: "teacher",
    icon: "Search",
    steps: [
      "Lesen Sie, ob dort `Keine Klassen zugewiesen` steht.",
      "Schreiben Sie der OGS und nennen Sie Ihre Klassen.",
      "Nennen Sie die Klassen genau so, wie sie an der Schule heißen.",
    ],
    result:
      "Nach der Zuweisung erscheinen Ihre Klassen in der `Klassenansicht`.",
    notes: [
      "Die Zuweisung macht die OGS in Ihrer Personalakte.",
      "Die Schreibweise muss zur Klasse der Kinder passen. Groß- und Kleinschreibung spielt keine Rolle.",
    ],
    differences: [
      "Mit der Rolle `Schulverwaltung` sehen Sie alle Klassen der Schule. Dafür braucht es keine Zuweisung.",
      "Sie sehen eine Klasse, aber keine Kinder? Dann ist die Klasse anders geschrieben als bei den Kindern.",
      "Nach einem Schuljahreswechsel wandern die Klassen automatisch mit.",
    ],
    troubleshootingDetails: [
      "Es steht `Kein Schultag`? Dann ist heute kein Unterricht. Prüfen Sie einen anderen Tag.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherMessages,
    ],
  };
}
