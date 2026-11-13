import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareCostSharesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareCostShares,
    title: "Kostenträger im Monatsbeitrag",
    question:
      "Wie trage ich ein, dass Bildung und Teilhabe oder das Jugendamt mitzahlt?",
    summary:
      "Sie tragen je Kind ein, wer wie viel vom Monatsbeitrag übernimmt. Die Familie bekommt nur den Rest in Rechnung. Den Teil des Kostenträgers laden Sie als eigene Liste herunter.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "HandCoins",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Der Kostenträger steht unter `Einstellungen` und `Abrechnungsvorgaben`.",
      "Sie haben den Bescheid geprüft.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Anteil eintragen",
        steps: [
          "Wählen Sie `Familien und Zahlung` und dann den Reiter `Kostenübernahmen`.",
          "Wählen Sie beim Kind das Menü und dann `Anteil eintragen`.",
          "Wählen Sie bei `Wer zahlt mit?` die Art: `Bildung und Teilhabe`, `Jugendamt` oder `Sonstiger Zuschuss`.",
          "Wählen Sie den `Kostenträger`.",
          "Wählen Sie bei `Wofür zahlt er?`: `Betreuung`, `Mensa` oder `Betreuung und Mensa`. Bei Bildung und Teilhabe ist `Mensa` schon gewählt.",
          "Wählen Sie `Prozent vom Beitrag` oder `Fester Betrag im Monat`. Tragen Sie die Zahl ein.",
          "Tragen Sie bei `Bescheid` nur das Aktenzeichen oder das Datum ein.",
          "Wählen Sie, ab wann und bis wann der Bescheid gilt.",
          "Wählen Sie `Anteil speichern`.",
        ],
      },
      {
        title: "Bevor ein Bescheid endet",
        description:
          "30 Tage vor dem Ende zeigt die Seite einen Hinweis. Der Anteil bekommt den Vermerk `Endet am`.",
        steps: [
          "Wählen Sie im Hinweis `Nur diese zeigen`.",
          "Fragen Sie bei der Familie den neuen Bescheid an.",
          "Wählen Sie beim Kind `Anteil eintragen` und tragen Sie den neuen Zeitraum ein.",
        ],
      },
      {
        title: "Liste für den Kostenträger",
        description:
          "Im Monatslauf zeigt die Karte `Kostenträger`, wie viel jede Stelle übernimmt.",
        steps: [
          "Öffnen Sie den Monatslauf unter `Monatsbeiträge`.",
          "Wählen Sie in der Karte `Kostenträger` das Menü der Zeile.",
          "Wählen Sie PDF, Excel oder CSV.",
          "Schicken Sie die Liste an den Kostenträger.",
        ],
      },
    ],
    result:
      "Der Monatslauf rechnet erst alle Ermäßigungen aus. Danach zieht er den Anteil des Kostenträgers ab. Die Rechnung zeigt den Monatsbeitrag, den Anteil und den Rest. Ein Anteil für die Mensa zieht vom Mensa-Betrag ab. Übernimmt der Kostenträger alles, steht auf der Rechnung `Beitragsfrei` und 0 €.",
    differences: [
      "Gilt ein Bescheid erst ab Mitte des Monats, zählt er für den ganzen Monat.",
      "Ein fester Betrag für `Betreuung und Mensa` geht erst an die Betreuung. Was übrig bleibt, geht an die Mensa.",
      "Kommt die Mensa auf eine eigene Rechnung, steht der Anteil für die Mensa auf der Mensa-Rechnung.",
      "Vor der Freigabe ist die Liste eine Vorschau ohne Rechnungsnummern.",
      "Eine Rechnung, die schon ausgestellt ist, bleibt, wie sie ist. Ein geänderter Anteil gilt ab dem nächsten Monatslauf.",
      "Ein stornierter Beitrag steht in der Liste, zählt aber nicht zur Summe.",
    ],
    notes: [
      "moto speichert nur Art, Anteil, Zeitraum und Aktenzeichen. Den Bescheid behalten Sie.",
      "Jedes Öffnen des Reiters `Kostenübernahmen` steht im Zugriffsprotokoll.",
    ],
    related: [
      HELP_TOPICS.traegerCareReductions,
      HELP_TOPICS.traegerCareMonths,
      HELP_TOPICS.traegerAccessLog,
    ],
  };
}
