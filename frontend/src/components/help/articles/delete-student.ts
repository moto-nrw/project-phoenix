import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Endgueltiges Loeschen, `StudentDeletionModal`. Textbestaetigung statt
 * Zwei-Klick-Tor: der Name des Kindes muss abgetippt werden (Bauart 2).
 */
export function deleteStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDeleteStudent,
    title: "Ein Kind dauerhaft löschen",
    question: "Wie lösche ich ein Kind endgültig?",
    summary: "Löschen entfernt die Daten des Kindes. Das ist endgültig.",
    group: "kinderdaten",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
      "Wählen Sie das Kind aus der Liste.",
      "Wählen Sie oben rechts `Löschen`.",
      "Lesen Sie, welche Daten entfernt und welche gelöst werden.",
      "Wählen Sie einen Grund für das Löschen.",
      "Haken Sie die Bestätigung an.",
      "Tippen Sie den Namen des Kindes in das Feld.",
      "Wählen Sie `Kind endgültig löschen`.",
    ],
    result:
      "Das Kind ist entfernt. Dieser Schritt lässt sich nicht zurücknehmen.",
    notes: [
      "Als Grund stehen `Testdaten`, `Fehlerhafte Erfassung`, `Doppelter Datensatz` und `Datenschutz-/Löschanfrage` zur Wahl.",
      "Ist die Betreuung bereits beendet, kommt `Aufbewahrungsfrist abgelaufen` dazu.",
    ],
    differences: [
      "Ein geplanter letzter Betreuungstag wird nicht abgewartet. Das Kind wird sofort gelöscht.",
    ],
    troubleshootingDetails: [
      "`Kind endgültig löschen` bleibt grau? Dann fehlt der Grund, das Häkchen oder der getippte Name.",
      "Soll das Kind nur aufhören? Nutzen Sie `Betreuung beenden`. Dabei bleiben alle Daten erhalten.",
    ],
    related: [
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.dataManagement,
    ],
  };
}
