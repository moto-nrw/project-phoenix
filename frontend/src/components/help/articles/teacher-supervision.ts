import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/aufsichten`: die eigenen Dienste des Tages. */
export function teacherSupervisionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSupervision,
    title: "Meine Aufsichten ansehen",
    question: "Wann habe ich heute Aufsicht?",
    summary: "Unter `Meine Aufsichten` stehen Ihre eigenen Dienste des Tages.",
    group: "aufsicht",
    audience: "teacher",
    icon: "Eye",
    steps: [
      "Öffnen Sie `Meine Aufsichten`.",
      "Lesen Sie, welche Aufsichten heute für Sie eingeteilt sind.",
      "Wählen Sie eine Aufsicht, um Einzelheiten zu sehen.",
    ],
    result: "Sie sehen Zeit, Raum und die Kinder Ihrer Aufsicht.",
    notes: [
      "Mit `Zurück zu allen Aufsichten heute` kommen Sie zurück zur Übersicht.",
    ],
    differences: [
      "Eine Aufsicht trägt `Fällt aus`? Dann findet sie heute nicht statt.",
      "Vertreten Sie jemanden, steht das an der Aufsicht.",
    ],
    troubleshootingDetails: [
      "Die Liste ist leer? Dann sind Sie heute nicht für eine Aufsicht eingeteilt. Eingeteilt wird im OGS-Team.",
    ],
    related: [
      HELP_TOPICS.teacherStartSupervision,
      HELP_TOPICS.teacherSupervisionRoster,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}
