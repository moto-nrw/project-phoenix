import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Loeschen einer Anmeldung aus der Detailansicht (Knopf im Statusbereich). */
export function enrollmentCleanupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentCleanup,
    title: "Eine fehlerhafte Anmeldung löschen",
    question: "Wie entferne ich eine doppelte oder falsche Anmeldung?",
    summary: "Löschen entfernt die ganze Anmeldung samt aller Kinder darin.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Öffnen Sie die betroffene Anmeldung.",
      "Wählen Sie rechts unter `Status der Anmeldung` den Knopf `Gesamte Anmeldung löschen`.",
      "Bestätigen Sie mit `Ja, löschen`.",
    ],
    result: "Die Anmeldung ist entfernt. Die Eltern können sich neu anmelden.",
    notes: [
      "Soll die Anmeldung nur nicht angenommen werden? Wählen Sie stattdessen `Ablehnen`.",
    ],
    differences: [
      "Eine abgelehnte Anmeldung lässt sich mit `Anmeldung wiederherstellen` zurückholen. Eine gelöschte nicht.",
    ],
    troubleshootingDetails: [
      "Ist das Kind schon übernommen? Dann ändert das Löschen der Anmeldung daran nichts. Beenden Sie stattdessen die Betreuung des Kindes.",
    ],
    related: [
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}
