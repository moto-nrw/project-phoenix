import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `Hinweis für die OGS` (`class-day-class.tsx`, #4795). Nur mit Freigabe der
 * OGS; gilt nur für heute, für die Klasse oder ein Kind.
 */
export function teacherDayHintTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherDayHint,
    title: "Der OGS einen Hinweis für heute geben",
    question: "Wie sage ich der OGS, was sie heute wissen muss?",
    summary:
      "Sie schreiben einen kurzen Hinweis für Ihre Klasse oder ein Kind.",
    group: "klasse",
    audience: "teacher",
    icon: "MessageSquareText",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse für heute.",
      "Wählen Sie `Hinweis für die OGS`.",
      "Wählen Sie bei `Für wen ist der Hinweis?` die ganze Klasse oder ein Kind.",
      "Schreiben Sie den Hinweis. Er darf bis zu 500 Zeichen lang sein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die OGS sieht den Hinweis sofort am Kind: in der Aufsicht, in der Gruppe und in der Kindersuche. Dort steht `Hinweis der Lehrkraft`.",
    notes: [
      "Der Hinweis gilt nur heute. Morgen ist er aus diesen Ansichten verschwunden.",
      "Wählen Sie das Kind oder die Klasse noch einmal, um Ihren Hinweis zu ändern.",
      "Mit `Hinweis löschen` nehmen Sie ihn zurück.",
    ],
    differences: [
      "Fehlt `Hinweis für die OGS`? Dann hat Ihre OGS das nicht freigegeben, oder Sie sehen einen anderen Tag als heute.",
      "Für Kinder ohne OGS-Betreuung können Sie keinen eigenen Hinweis schreiben.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherArrivalChange,
      HELP_TOPICS.teacherMessages,
    ],
  };
}
