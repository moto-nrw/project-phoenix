import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Regulaerer Austritt, Assistent `CareExitModal` in zwei Schritten (#2487).
 * Bewusst getrennt vom Loeschen: hier bleiben alle Daten erhalten.
 */
export function endCareTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEndCare,
    title: "Die Betreuung eines Kindes beenden",
    question: "Wie beende ich die Betreuung eines Kindes?",
    summary: "Sie setzen einen letzten Betreuungstag. Die Daten bleiben.",
    group: "kinderdaten",
    audience: "lead",
    icon: "LogOut",
    steps: [],
    instructionGroups: [
      {
        title: "Ein einzelnes Kind abmelden",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie das Kind aus der Liste.",
          "Wählen Sie oben rechts `Betreuung beenden`.",
          "Tragen Sie bei `Letzter Betreuungstag` den Tag ein.",
          "Wählen Sie einen Grund: `Umzug`, `Kein Betreuungsbedarf mehr` oder `Anderer Grund`.",
          "Wählen Sie `Weiter`.",
          "Prüfen Sie die Vorschau und wählen Sie `Betreuung beenden`.",
        ],
      },
      {
        title: "Mehrere Kinder auf einmal abmelden",
        steps: [
          "Wählen Sie oben rechts `Auswählen`.",
          "Haken Sie die Kinder an.",
          "Wählen Sie `Betreuung beenden`.",
          "Gehen Sie die beiden Schritte wie oben durch.",
        ],
      },
    ],
    result:
      "Ab dem Tag nach dem letzten Betreuungstag ist das Kind nicht mehr in Betreuung.",
    notes: [
      "Bei `Anderer Grund` tragen Sie zusätzlich eine kurze Erklärung ein.",
      "Beendete Kinder finden Sie über das Menü mit den drei Punkten unter `Beendete Betreuungen`.",
      "Nach der Löschfrist löscht moto das Kind mit allen Angaben. Die Frist sehen Sie unter `Einstellungen` und `Datenschutz`.",
      "Erziehungsberechtigte ohne weiteres Kind an der Schule löscht moto nach einer eigenen Frist. Auch sie steht unter `Datenschutz`.",
    ],
    differences: [
      "Ist schon ein Ende geplant, heißt der Knopf `Ende ändern`. Daneben steht `Ende stornieren`.",
      "Ist die Betreuung schon beendet, steht dort `Wieder aufnehmen`.",
    ],
    troubleshootingDetails: [
      "Die Vorschau blockiert das Beenden? Dann sprechen die geprüften Angaben dagegen. Lesen Sie den Hinweis in der Vorschau.",
      "Fehlt `Wieder aufnehmen`? Dann lief die Betreuung mit der Anmeldephase aus. Das lässt sich nicht zurücknehmen.",
    ],
    related: [
      HELP_TOPICS.leadDeleteStudent,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadGradeTransition,
    ],
  };
}
