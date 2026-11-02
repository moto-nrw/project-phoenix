import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function sharedFilesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.sharedFiles,
    title: "Eine gemeinsame Datei öffnen oder hochladen",
    question: "Wie öffne oder teile ich eine gemeinsame Datei?",
    summary: "Nutzen Sie die gemeinsame Dateiablage Ihrer OGS.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "FolderOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Datei öffnen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf und öffnen Sie `Dateien`.",
          "Wählen Sie links unter `Ordner` den passenden aus.",
          "Suchen Sie die Datei in der Liste.",
          // Live geprueft: die Aktionen liegen im Zeilenmenue und heissen
          // `Öffnen` und `Herunterladen`.
          "Öffnen Sie am Ende der Zeile das Menü mit den drei Punkten.",
          "Wählen Sie `Öffnen` oder `Herunterladen`.",
        ],
      },
      {
        title: "Eine Datei hochladen",
        steps: [
          "Öffnen Sie den passenden Ordner.",
          "Wählen Sie `Dateien auswählen`.",
          "Wählen Sie die Datei auf Ihrem Gerät aus.",
          "Warten Sie auf die Bestätigung.",
        ],
      },
    ],
    result:
      "Die Datei steht den berechtigten Personen im Ordner zur Verfügung.",
    differences: [
      "Welche Ordner Sie sehen, hängt von Ihrer Rolle ab.",
      "Fehlt `Dateien auswählen`? Dann dürfen Sie in diesem Ordner nur lesen.",
    ],
    notes: [
      "Speichern Sie persönliche Angaben nur in einem dafür vorgesehenen und geschützten Ordner.",
      "Legen Sie keine zweite Datei mit fast gleichem Namen an. Prüfen Sie zuerst, ob bereits eine aktuelle Fassung vorhanden ist.",
    ],
    related: [HELP_TOPICS.missingMenu, HELP_TOPICS.leadMissingMenu],
  };
}
