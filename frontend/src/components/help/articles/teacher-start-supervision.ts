import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Aufsicht starten: erst danach zeigt moto die Kinder mit Kontakten. */
export function teacherStartSupervisionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherStartSupervision,
    title: "Eine Aufsicht starten",
    question: "Wie starte ich meine Aufsicht?",
    summary: "Mit dem Start sehen Sie die Kinder, die zu Ihnen gehören.",
    group: "aufsicht",
    audience: "teacher",
    icon: "Clock3",
    steps: [
      "Öffnen Sie `Meine Aufsichten`.",
      "Wählen Sie die Aufsicht, die jetzt beginnt.",
      "Wählen Sie `Aufsicht starten`.",
    ],
    result: "moto zeigt danach die Kinder dieser Aufsicht mit ihren Angaben.",
    notes: [
      "Starten können Sie erst kurz vor dem Beginn, nicht schon am Morgen.",
    ],
    differences: [
      "Ist `Aufsicht starten` grau, ist es noch zu früh oder die Aufsicht läuft schon.",
    ],
    troubleshootingDetails: [
      "Der Knopf fehlt ganz? Dann ist diese Aufsicht nicht Ihre. Prüfen Sie die Übersicht.",
    ],
    related: [
      HELP_TOPICS.teacherSupervision,
      HELP_TOPICS.teacherSupervisionRoster,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
