import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/dokumente` (#4692): vorhandene Dokumente hochladen und für
 * Eltern oder Schulen freigeben; eine Freigabe zurücknehmen.
 */
export function traegerSharedDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSharedDocuments,
    title: "Dokumente für Eltern und Schulen freigeben",
    question: "Wie teile ich ein Dokument mit Eltern oder Schulen?",
    summary:
      "Sie laden eine Datei hoch, die Sie schon haben, zum Beispiel eine Hausordnung oder eine Liste. Dann geben Sie sie für Eltern oder für Schulen frei.",
    group: "elternarbeit",
    audience: "traeger",
    icon: "EnvelopeOpen",
    requirements: [
      "Sie haben die Rolle `Verwaltung`.",
      "Die Datei ist ein PDF, ein Word-, Excel- oder PowerPoint-Dokument oder ein Bild. Sie ist höchstens 10 MB groß.",
    ],
    instructionGroups: [
      {
        title: "Dokument hochladen",
        steps: [
          "Wählen Sie in der Navigation `Dokumente`.",
          "Wählen Sie oben `Dokument hochladen`.",
          "Ziehen Sie die Datei in das Feld. Oder wählen Sie `Datei hierher ziehen oder auswählen`.",
          "Prüfen Sie den `Titel`. Eltern und Schulen sehen diesen Titel.",
          "Wählen Sie `Hochladen`. moto öffnet danach die Seite des Dokuments.",
        ],
      },
      {
        title: "Dokument freigeben",
        steps: [
          "Öffnen Sie das Dokument in der Liste `Dokumente`.",
          "Wählen Sie oben `Freigeben`.",
          "Wählen Sie bei `Für wen?` entweder `Eltern` oder `Schulen`.",
          "Wählen Sie bei `An welchen Schulen?`: `Alle Schulen`, `Eine Schule` oder bei Eltern auch `Eine Gruppe`.",
          "Wählen Sie dann die `Schule` und gegebenenfalls die `Gruppe`.",
          "Prüfen Sie den Kasten `Sichtbar für`. Wählen Sie `Freigeben`.",
        ],
      },
      {
        title: "Freigabe zurücknehmen",
        steps: [
          "Öffnen Sie das Dokument.",
          "Wählen Sie bei der Freigabe `Zurücknehmen`.",
          "Bestätigen Sie mit `Ja, zurücknehmen` und dann `Zurücknehmen`.",
        ],
      },
    ],
    steps: [],
    result:
      "Eltern finden das Dokument in der Eltern-App unter `Dokumente`. Die OGS-Leitung findet es in moto unter `Dateien` bei `Vom Träger`. Nach dem Zurücknehmen sehen sie es sofort nicht mehr, außer eine andere Freigabe des Dokuments gilt auch für sie.",
    notes: [
      "Ein Dokument kann mehrere Freigaben haben. Zum Beispiel für die Eltern einer Gruppe und für alle Schulen.",
      "Eine Freigabe für Eltern sieht auch die OGS-Leitung der Schule. So weiß sie, was die Eltern bekommen haben.",
      "Bei `Geöffnet` sehen Sie, wie oft Eltern und Schulen das Dokument geöffnet haben.",
    ],
    differences: [
      "Sie sehen `Dokument hochladen` und `Freigeben` nicht? Ihnen fehlt die Rolle `Verwaltung`. Sie können die Dokumente nur ansehen.",
    ],
    troubleshootingDetails: [
      "moto nimmt die Datei nicht an? Prüfen Sie Art und Größe. Eine umbenannte Datei erkennt moto am Inhalt.",
      "Bei `Eine Gruppe` steht keine Gruppe? Die Schule hat in moto noch keine Gruppen angelegt.",
      "Die Meldung sagt, das Dokument ist schon freigegeben? Für diese Menschen gibt es schon eine Freigabe.",
    ],
    related: [HELP_TOPICS.traegerDocumentTemplates, HELP_TOPICS.errorMessages],
  };
}
