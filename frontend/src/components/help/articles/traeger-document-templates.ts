import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/vorlagen` (#4677): Text der Briefe mit Platzhaltern. */
export function traegerDocumentTemplatesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDocumentTemplates,
    title: "Vorlagen für Briefe anpassen",
    question: "Wie ändere ich den Text der Briefe meines Trägers?",
    summary:
      "Für jede Art von Brief legen Sie Betreff, Anrede, Text und Gruß fest. Platzhalter wie {{familienname}} füllt moto für jede Familie aus.",
    group: "konfiguration",
    audience: "traeger",
    icon: "Notebook",
    requirements: ["Sie dürfen im Träger-Büro Einstellungen ändern."],
    instructionGroups: [
      {
        title: "Vorlage ändern",
        steps: [
          "Klicken Sie links unten in der Navigation auf `Einstellungen`. Wählen Sie darunter `Vorlagen`.",
          "Wählen Sie das Dokument, zum Beispiel `Bestätigung der Ferienanmeldung`.",
          "Ändern Sie bei `Text der Vorlage` die Felder `Betreff`, `Anrede`, `Brieftext` und `Grußformel`.",
          "Schreiben Sie Angaben der Familie als Platzhalter, zum Beispiel {{familienname}}. Die Liste `Platzhalter` zeigt alle, die dieses Dokument kennt.",
          "Wählen Sie `Vorschau zeigen`. Die `Vorschau` zeigt den Brief mit erfundenen Beispieldaten im Briefkopf Ihres Trägers.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Frühere Fassung zurückholen",
        steps: [
          "Suchen Sie unten bei `Versionen` die Fassung.",
          "Wählen Sie `Text übernehmen`. Der Text steht dann oben im Editor.",
          "Wählen Sie `Speichern`. Daraus wird eine neue Version.",
        ],
      },
    ],
    steps: [],
    result:
      "Jedes Speichern legt eine neue Version an. Neue Schreiben nutzen ab dann diese Version. Schreiben, die schon erstellt sind, ändern sich nicht.",
    notes: [
      "Eine leere Zeile beginnt einen neuen Absatz. Eine Zeile mit # am Anfang wird eine Überschrift. Text zwischen ** und ** wird fett.",
      "Ein Tabellen-Platzhalter wie {{positionen}} druckt eine Tabelle, wenn er allein in einer Zeile steht.",
      "Sind alle Platzhalter eines Absatzes leer, fehlt der Absatz im Brief. So entfallen zum Beispiel die Notfallnummern, wenn keine eingetragen sind.",
      "Mit `Standardtext laden` holen Sie den Text von moto zurück in den Editor.",
      "Heute nutzt moto die Vorlage für das Bestätigungsschreiben der Ferien. Die anderen Dokumente folgen.",
    ],
    differences: [
      "Sie können die Vorlage nur ansehen? Ihnen fehlt das Recht `Einstellungen`. Wenden Sie sich an eine Person mit der Rolle `Verwaltung`.",
    ],
    troubleshootingDetails: [
      "moto nimmt die Vorlage nicht an und nennt Platzhalter? Diese kennt das Dokument nicht. Prüfen Sie die Schreibweise mit der Liste `Platzhalter`.",
      "Die Meldung sagt, dass sich die Vorlage nicht geändert hat? Der Text ist gleich wie die aktuelle Version. Es entsteht keine neue Version.",
    ],
    related: [
      HELP_TOPICS.traegerBriefkopf,
      HELP_TOPICS.traegerBillingRun,
      HELP_TOPICS.errorMessages,
    ],
  };
}
