import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerFinanceExportsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerFinanceExports,
    title: "Sollstellung für die Kämmerei",
    question: "Wie gebe ich die Rechnungen an die Kämmerei?",
    summary:
      "Für jeden ausgestellten Lauf laden Sie eine Sollstellung herunter. Das ist eine CSV-Datei mit einer Zeile je Rechnung. Welche Spalten sie hat, legen Sie selbst fest. Jede Datei bekommt eine laufende Nummer.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Table",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Der Monatslauf oder Ferienlauf ist ausgestellt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Spalten festlegen",
        description:
          "Fragen Sie Ihre Kämmerei, welche Spalten ihr Programm liest. Das machen Sie nur einmal.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie oben rechts `Feldzuordnung`.",
          "Wählen Sie bei jeder Spalte den `Inhalt` und tragen Sie den `Spaltennamen` ein.",
          "Mit `Spalte` fügen Sie eine Spalte hinzu. Für einen Wert, der in jeder Zeile gleich ist, wählen Sie `Fester Wert`, etwa ein Konto.",
          "Legen Sie unter `Datei` Trennzeichen, Beträge, Datum, Zeichensatz und Dateinamen fest. `{NR}` im Dateinamen wird zur Dateinummer.",
          "Prüfen Sie die `Vorschau` und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Erste Dateinummer festlegen",
        description:
          "Nur nötig, wenn die Nummern Ihres bisherigen Programms weiterlaufen sollen.",
        steps: [
          "Tragen Sie in der Karte `Dateinummern` bei `Erste Nummer` die Nummer nach Ihrer letzten Datei ein.",
          "Wählen Sie `Festlegen` und dann `Ja, festlegen`.",
        ],
      },
      {
        title: "Sollstellung herunterladen",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie beim Lauf `Sollstellung`. Die Datei wird heruntergeladen.",
          "Geben Sie die Datei an Ihre Kämmerei. Oder lassen Sie moto sie übergeben, siehe „Exporte an Empfänger übergeben“.",
        ],
      },
    ],
    result:
      "Die Datei steht im `Exportprotokoll` mit Nummer, Datum und wer sie geholt hat. Beim Lauf steht die letzte Sollstellung. In jeder Monatsrechnung zeigt `Verlauf`, in welchen Dateien sie war.",
    notes: [
      "Jeder Download ist eine neue Datei mit neuer Nummer. Steht im Protokoll `Gleicher Inhalt wie Nr. …`, enthält die Datei nichts Neues.",
      "Auch jede Lastschriftdatei und jede DATEV-Datei bekommt eine Nummer aus derselben Reihe. So nennt jede Nummer genau eine Datei.",
      "Für Ihr Steuerbüro gibt es beim Lauf auch `DATEV`. Wie das geht, steht unter „DATEV-Werte eintragen“.",
      "Die Debitorennummer der Familie tragen Sie bei `Empfänger und Zahlung` der Familie ein. Neue Monats- und Ferienrechnungen übernehmen sie. Hat eine Ferienanmeldung eine eigene Nummer, gilt bei der Ferienrechnung diese. Beim Import der Mandate übernimmt moto die Spalte `Debitorennummer`.",
      "Eine neue Feldzuordnung gilt für jede neue Datei. Frühere Dateien bleiben, wie sie waren.",
      "Wählen Sie als Inhalt `Erlöskonto`, `Kostenstelle (KOST1)` oder `Zweite Kostenstelle (KOST2)`, hat die Datei eine Zeile je Konto einer Rechnung. Der `Betrag` ist dann der Teil der Rechnung auf diesem Konto. Die Konten legen Sie unter `Konten und Kostenstellen` fest.",
    ],
    differences: [
      "Der Reiter `Exporte` fehlt? Ihnen fehlt das Recht `Finanz-Export`.",
      "Ein älterer Lauf oder eine ältere Datei fehlt? Die Listen zeigen zuerst die neuesten 50. Wählen Sie unter der Liste `Ältere anzeigen`.",
      "Die Karte `Dateinummern` zeigt kein Feld? Die Nummern laufen schon. Die erste Nummer lässt sich nicht mehr ändern.",
    ],
    troubleshootingDetails: [
      "Die Kämmerei sieht falsche Umlaute? Wählen Sie bei `Zeichensatz` den Wert `Windows-1252`.",
      "In der Datei stehen alle Werte in einer Spalte? Das Trennzeichen passt nicht zum Programm der Kämmerei. Wählen Sie ein anderes.",
      "Beim Import einer Datei zeigt eine Zeile `Geschwister haben verschiedene Nummern`? moto führt eine Nummer je Familie. Tragen Sie die richtige Nummer bei der Familie ein.",
    ],
    related: [
      HELP_TOPICS.traegerExportHandover,
      HELP_TOPICS.traegerDatevExport,
      HELP_TOPICS.traegerCareMonths,
      HELP_TOPICS.traegerSepaDebitFile,
      HELP_TOPICS.traegerBillingTerms,
    ],
  };
}
