import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `Ankunft heute ändern` (`class-arrival-exception-dialog.tsx`). Nur mit
 * Freigabe der OGS; gilt fuer einen Tag und die ganze Klasse.
 */
export function teacherArrivalChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherArrivalChange,
    title: "Die Ankunftszeit meiner Klasse ändern",
    question: "Der Unterricht fällt aus. Wie sage ich das der OGS?",
    summary: "Sie tragen die neue Ankunftszeit für einen einzelnen Tag ein.",
    group: "klasse",
    audience: "teacher",
    icon: "Clock3",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse.",
      "Wählen Sie `Ankunft heute ändern`.",
      "Wählen Sie den Tag.",
      "Tragen Sie bei `Kommt um` die neue Zeit ein.",
      "Tragen Sie bei Bedarf einen `Grund (optional)` ein, zum Beispiel `Wandertag`.",
      "Wählen Sie `Tag speichern`.",
    ],
    result:
      "Die OGS sieht die neue Zeit sofort. Sie gilt nur für Kinder mit Betreuung an diesem Tag.",
    notes: [
      "Die Zeit gilt für alle Kinder der Klasse und nur an diesem Tag.",
      "Mit `Entfernen` nehmen Sie einen Eintrag wieder zurück.",
    ],
    differences: [
      "Fehlt `Ankunft heute ändern`? Dann hat Ihre OGS das Eintragen nicht freigegeben. Sie sehen dann nur die Zeit, die die OGS eingetragen hat.",
      "Bei einem Eintrag der OGS steht `Eingetragen von der OGS`.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Eintragen sei nicht mehr freigegeben? Dann hat die OGS es zurückgenommen. Schreiben Sie ihr eine Nachricht.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
