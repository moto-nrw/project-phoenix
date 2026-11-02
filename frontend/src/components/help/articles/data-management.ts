import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Uebersichtsseite `/database`. Die Kacheln blenden sich nach Rechten,
 * NFC und Planungsbereich aus (page.tsx:312-336) -- deshalb steht in der
 * Anleitung keine feste Liste, sondern nur die Grenze zum Tagesbetrieb.
 */
export function dataManagementTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.dataManagement,
    title: "Die Datenverwaltung im Überblick",
    question: "Wo pflege ich die Stammdaten der OGS?",
    summary:
      "In der `Datenverwaltung` legen Sie an, was die OGS dauerhaft braucht.",
    group: "einrichten",
    audience: "lead",
    icon: "Database",
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Wählen Sie `Datenverwaltung`.",
      "Wählen Sie die Kachel des Bereichs, den Sie pflegen möchten.",
    ],
    result:
      "Jede Kachel führt in ihren Bereich. Auf manchen Kacheln steht, wie viele Einträge es gibt.",
    notes: [
      "In der Seitenleiste erreichen Sie jeden Bereich auch direkt unter `Datenverwaltung`.",
      "Der Info-Knopf neben dem Seitennamen erklärt kurz, was hierher gehört.",
      "Die Kacheln `Terminkategorien`, `Planungsspuren`, `Dienstarten` und `Abwesenheitsarten` sind kurze Stammdaten-Listen.",
    ],
    differences: [
      "Sie sehen weniger Kacheln als eine Kollegin? Jede Kachel hängt an einem eigenen Recht.",
      "Ohne NFC fehlen `Aktivitäten` und `Geräte`.",
      "Ohne Planungsbereich fehlen `Planungsspuren` und `Dienstarten`.",
    ],
    troubleshootingDetails: [
      "Suchen Sie ein Kind für heute? Das steht nicht hier, sondern unter `Alle Kinder` im `Tagesbetrieb`.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadExports,
    ],
  };
}
