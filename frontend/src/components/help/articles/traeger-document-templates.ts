import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/einstellungen/briefkopf/vorlagen` (#4677): Text der Briefe mit
 * Platzhaltern; bei Bestätigung und Rechnung auch die Rechnungstabelle und
 * die Vorschau des ganzen Briefs (#5691).
 */
export function traegerDocumentTemplatesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDocumentTemplates,
    title: "Vorlagen für Briefe anpassen",
    question: "Wie ändere ich den Text der Briefe meines Trägers?",
    summary:
      "Für jede Art von Brief legen Sie Betreff, Anrede, Text und Gruß fest. Bei Schreiben mit Rechnung auch, was die Rechnungstabelle zeigt. Platzhalter wie {{familienname}} füllt moto für jede Familie aus.",
    group: "konfiguration",
    audience: "traeger",
    icon: "Notebook",
    requirements: ["Sie dürfen im Träger-Büro Einstellungen ändern."],
    instructionGroups: [
      {
        title: "Vorlage ändern",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Briefkopf und Vorlagen`.",
          "Für das Schreiben nach der Ferienanmeldung wählen Sie `Schreiben anpassen`. Es steht bei `Bestätigung und Rechnung`.",
          "Für andere Schreiben wählen Sie bei `Weitere Vorlagen` den Knopf `Vorlagen öffnen`. Wählen Sie dann das Schreiben.",
          "Ändern Sie bei `Text der Vorlage` die Felder `Betreff`, `Anrede`, `Brieftext` und `Grußformel`.",
          "Für Angaben der Familie tippen Sie bei `Platzhalter` auf einen Eintrag, zum Beispiel {{familienname}}. moto setzt ihn im `Brieftext` ein. Er steht dort, wo Sie zuletzt geschrieben haben.",
          "Bei Schreiben mit Rechnung wählen Sie unter `Rechnungstabelle`, was die Tabelle zeigt. Es gibt `Nach Kind ordnen` und `Zeitmodell zeigen`.",
          "Wählen Sie `Vorschau zeigen`. Die `Vorschau` zeigt den ganzen Brief mit erfundenen Beispieldaten.",
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
      "Der Brief hat die Reihenfolge des Brieftexts. Steht {{positionen}} weiter oben, steht auch die Tabelle weiter oben. So ordnen Sie Text, Tabelle, Hinweise und Notfallnummern selbst.",
      "Einleitung, Kontaktsatz und Unterschrift schreiben Sie frei. Sie stehen im `Brieftext` und in der `Grußformel`. Statt {{rueckfragen}} geht auch ein eigener Satz.",
      "Woche, Betrag, Ermäßigungen und Gesamtbetrag stehen immer in der Rechnungstabelle. Das sind Pflichtangaben einer Rechnung.",
      "Die `Vorschau` zeigt auch Anschrift, Rechnungsnummer und Debitorennummer. So sehen Sie Text, Nummern und Tabelle zusammen.",
      "Sind alle Platzhalter eines Absatzes leer, fehlt der Absatz im Brief. So entfallen zum Beispiel die Notfallnummern, wenn keine eingetragen sind.",
      "Mit `Standardtext laden` holen Sie den Text von moto zurück in den Editor.",
      "Heute nutzt moto die Vorlagen für das Bestätigungsschreiben der Ferien und für den Betreuungsvertrag. Die anderen Dokumente folgen.",
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
