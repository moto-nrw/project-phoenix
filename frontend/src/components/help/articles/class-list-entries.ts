import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Klassenlisteneintraege unter `/database/students/class-list` (#2382):
 * Kinder ohne OGS-Betreuung, damit Klassenlisten vollstaendig sind.
 */
export function classListEntriesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadClassListEntries,
    title: "Kinder ohne OGS-Betreuung erfassen",
    question: "Wie erfasse ich Kinder, die nicht in der OGS sind?",
    summary: "Für vollständige Klassenlisten erfassen Sie auch diese Kinder.",
    group: "kinderdaten",
    audience: "lead",
    icon: "ClipboardList",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Eintrag anlegen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Klassenliste`.",
          "Wählen Sie oben rechts `+ Eintrag`.",
          "Tragen Sie Vorname, Nachname und Klasse ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Viele Einträge auf einmal",
        steps: [
          "Öffnen Sie `Klassenliste`.",
          "Wählen Sie oben `Sammelimport`.",
          "Folgen Sie den Schritten auf der Seite.",
        ],
      },
    ],
    result:
      "Das Kind erscheint auf den Klassenlisten, aber nicht in der Betreuung.",
    notes: [
      "Sie können ein Kind auch beim Anlegen über `Nur Klassenliste` erfassen.",
      "Schreiben Sie die Klasse genau wie bei den regulären Kindern.",
    ],
    differences: [
      "Ein Eintrag trägt `Mögliche Dublette`? Dann gibt es ein reguläres Kind mit demselben Namen in derselben Klasse.",
      "Ein Eintrag trägt `In moto angelegt`? Dann ist es ein reguläres Kind. Gepflegt wird es unter `Kinderdaten`.",
    ],
    troubleshootingDetails: [
      "Kommt das Kind doch in die Betreuung? Ordnen Sie den Eintrag dem regulären Kind zu. Der Eintrag verschwindet dann aus der Klassenliste.",
    ],
    related: [
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadDayLists,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
