import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/personal` (#4705, ADR 0089): eine Personalakte über alle
 * Schulen. Vorschläge prüfen und verknüpfen, Personalnummer und Bank- und
 * Steuerdaten pflegen, eine Akte auflösen. Nur mit der Rolle Personal und
 * nach der Freigabe „Personal beim Träger“.
 */
export function traegerStaffRecordsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerStaffRecords,
    title: "Personalakten über alle Schulen",
    question: "Wie führe ich eine Person mit einer Akte über alle Schulen?",
    summary:
      "Arbeitet eine Person an mehreren Schulen, führen Sie sie mit einer Akte. Die Akte hat eine Personalnummer und die Bank- und Steuerdaten.",
    group: "personal",
    audience: "traeger",
    icon: "UsersThree",
    requirements: [
      "Sie haben die Rolle `Personal`.",
      "Das moto-Team hat `Personal beim Träger` für Ihren Träger freigeschaltet. Dafür bestätigt Ihr Träger: Es gibt eine Vereinbarung mit Betriebsrat oder Mitarbeitervertretung, oder es gibt keine Vertretung.",
    ],
    instructionGroups: [
      {
        title: "Einen Vorschlag verknüpfen",
        description:
          "moto schlägt vor, welche Beschäftigungen dieselbe Person sind: zuerst gleiches Konto, dann gleiche Personalnummer, dann gleicher Name und Geburtstag.",
        steps: [
          "Wählen Sie in der Navigation `Personalakten`.",
          "Öffnen Sie den Reiter `Vorschläge`.",
          "Wählen Sie einen Vorschlag.",
          "Prüfen Sie, ob es wirklich dieselbe Person ist.",
          "Wählen Sie bei `Welcher Wert gilt?` für jedes Feld den richtigen Wert.",
          "Wählen Sie `Akte anlegen`.",
        ],
      },
      {
        title: "Eine Akte für eine Person anlegen",
        steps: [
          "Öffnen Sie den Reiter `Ohne Akte`.",
          "Wählen Sie die Person.",
          "Prüfen Sie die Werte. Wählen Sie `Akte anlegen`.",
        ],
      },
      {
        title: "Personalnummer eintragen",
        steps: [
          "Öffnen Sie die Akte im Reiter `Akten`.",
          "Wählen Sie bei `Personalnummer` den Knopf `Ändern`.",
          "Tragen Sie die Nummer aus Ihrem Lohnsystem ein. Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Bank- und Steuerdaten ändern",
        steps: [
          "Öffnen Sie die Akte.",
          "Wählen Sie bei `Bank und Steuer` den Knopf `Ändern`.",
          "Tragen Sie IBAN, Steuer-ID oder Sozialversicherungsnummer ein. Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Akte auflösen",
        description:
          "Zum Beispiel, wenn zwei verschiedene Personen verknüpft wurden.",
        steps: [
          "Öffnen Sie die Akte.",
          "Wählen Sie oben rechts das Menü mit den drei Punkten. Wählen Sie `Akte auflösen`.",
          "Bestätigen Sie zweimal.",
        ],
      },
    ],
    steps: [],
    result:
      "Die Akte gilt für alle Schulen der Person. Jede Schule sieht die Personalnummer des Trägers und kann sie nicht mehr ändern. Die Bank- und Steuerdaten ziehen von den Schulen in die Akte. Die Schulen sehen nur noch: „führt der Träger“. Planung, Zeiten und Abwesenheiten bleiben an jeder Schule wie bisher.",
    notes: [
      "Ein Wert, den Sie beim Verknüpfen nicht wählen, bleibt mit seiner Schule im `Verlauf` der Akte.",
      "Jede Schule sieht weiter nur ihr eigenes Personal. Sie sieht keine Akte und keine anderen Schulen.",
      "Jedes Anzeigen der Bank- und Steuerdaten steht im Zugriffsprotokoll jeder Schule der Person.",
      "Lehrkräfte des Landes und Kräfte anderer Träger bekommen keine Akte. Verknüpfen Sie nur Personen, die Ihr Träger beschäftigt.",
      "Die Verarbeitung stützt sich auf Art. 6 Abs. 1 lit. b und c DSGVO: Ihr Träger ist der Arbeitgeber.",
    ],
    differences: [
      "Sie sehen nur einen Hinweis, dass Personal beim Träger noch nicht freigeschaltet ist: Bitten Sie das moto-Team um die Freigabe.",
      "Sie sehen `Personalakten` nicht in der Navigation: Ihnen fehlt die Rolle `Personal`.",
    ],
    troubleshootingDetails: [
      "„Diese Personalnummer ist schon vergeben“: Eine andere Akte oder eine andere Person an einer der Schulen hat die Nummer. Ändern Sie zuerst die andere Nummer.",
      "„Eine dieser Personen hat schon eine Personalakte“: Laden Sie die Seite neu. Vielleicht hat jemand den Vorschlag gerade verknüpft.",
    ],
    related: [HELP_TOPICS.traegerAccounts, HELP_TOPICS.traegerAccessLog],
  };
}
