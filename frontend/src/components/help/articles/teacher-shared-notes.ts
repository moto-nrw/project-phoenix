import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `Beobachtungen der OGS` (`student-row.tsx`, `/school/klasse/kind`, #4797).
 * Nur die Lehrkraft der Klasse; nur Notizen, die die OGS freigegeben hat.
 */
export function teacherSharedNotesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSharedNotes,
    title: "Beobachtungen der OGS lesen",
    question: "Was hat die OGS über ein Kind meiner Klasse notiert?",
    summary:
      "Sie lesen die Notizen, die die OGS für die Schule freigegeben hat.",
    group: "klasse",
    audience: "teacher",
    icon: "FileText",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse.",
      "Wählen Sie beim Kind `Beobachtungen der OGS`.",
      "Lesen Sie die Notizen. Die neueste steht oben.",
    ],
    result:
      "Sie sehen jede Notiz mit Tag und dem Namen der Person aus der OGS, die sie geschrieben hat.",
    notes: [
      "Sie können die Notizen nur lesen. Ändern oder beantworten können Sie sie nicht.",
      "Jeder Abruf wird protokolliert.",
      "Fragen zu einer Notiz klären Sie direkt mit der OGS, zum Beispiel über `Nachrichten`.",
    ],
    differences: [
      "Fehlt `Beobachtungen der OGS`? Dann sind Sie der Klasse nicht als Lehrkraft zugewiesen. Die Schulverwaltung sieht die Notizen nicht.",
      "Steht dort `Keine Beobachtungen`? Dann hat die OGS für dieses Kind nichts freigegeben.",
    ],
    troubleshootingDetails: [
      "Eine Notiz ist verschwunden? Dann hat die OGS die Freigabe zurückgenommen.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherDayHint,
      HELP_TOPICS.teacherMessages,
    ],
  };
}
