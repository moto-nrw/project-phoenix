import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerDatevExportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDatevExport,
    title: "DATEV-Werte eintragen",
    question: "Wie bekommt mein Steuerbüro die Beiträge für DATEV?",
    summary:
      "Für jeden ausgestellten Lauf laden Sie eine DATEV-Datei herunter. Ihr Steuerbüro liest sie in DATEV ein. Vorher tragen Sie einmal die DATEV-Werte Ihres Steuerbüros ein.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Settings",
    requirements: [
      "Ihre Rolle im Träger-Büro hat das Recht Finanz-Export.",
      "Ihr Steuerbüro hat Ihnen die DATEV-Werte genannt.",
      "Jede Familie hat eine Debitorennummer.",
      "Der Monatslauf oder Ferienlauf ist ausgestellt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "DATEV-Werte eintragen",
        description:
          "Das machen Sie nur einmal. Fragen Sie Ihr Steuerbüro nach den Werten.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie oben rechts `DATEV-Werte`. Die Einstellungen öffnen sich im Bereich `DATEV`.",
          "Tragen Sie `Beraternummer` und `Mandantennummer` ein.",
          "Wählen Sie die `Länge der Sachkonten`, den `Kontenrahmen` und den `Beginn des Wirtschaftsjahres`.",
          "Tragen Sie das `Erlöskonto für Elternbeiträge` ein.",
        ],
      },
      {
        title: "DATEV-Datei herunterladen",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie beim Lauf `DATEV`. Die Datei wird heruntergeladen.",
          "Geben Sie die Datei an Ihr Steuerbüro.",
        ],
      },
    ],
    result:
      "In der Datei steht jede Rechnung im Soll, jedes Storno und jede Gutschrift im Haben. Die Datei steht im `Exportprotokoll` mit ihrer Nummer. In jeder Rechnung zeigt `Verlauf`, in welcher Datei sie war.",
    notes: [
      "Die Debitorennummer tragen Sie bei `Empfänger und Zahlung` der Familie ein. DATEV nimmt nur Ziffern an, eine mehr als bei den Sachkonten. Bei 4 Stellen ist das zum Beispiel `10001`.",
      "Jeder Download ist eine neue Datei mit neuer Nummer. Geben Sie Ihrem Steuerbüro einen Lauf nur einmal, sonst stehen die Buchungen doppelt in DATEV.",
      "Die DATEV-Datei und die Sollstellung zählen in derselben Reihe von Dateinummern.",
    ],
    differences: [
      "Über der Liste steht `Die DATEV-Datei fehlt noch`? Es fehlen DATEV-Werte. Der Hinweis nennt sie. Bis dahin lässt sich `DATEV` nicht wählen.",
      "Sie können die DATEV-Werte nur ansehen? Ändern darf, wer im Träger-Büro das Recht für die Einstellungen hat.",
    ],
    troubleshootingDetails: [
      "Es erscheint `Keine DATEV-Datei für …` mit Rechnungsnummern? Diese Rechnungen nimmt DATEV so nicht an. Meist fehlt die Debitorennummer oder sie hat Leerzeichen. Tragen Sie die richtige Nummer bei der Familie ein. Eine ausgestellte Rechnung behält ihre alte Nummer: Stornieren Sie sie und stellen Sie sie neu aus.",
      "Eine Rechnung liegt in einem anderen Wirtschaftsjahr? Eine DATEV-Datei gilt nur für ein Wirtschaftsjahr. Fragen Sie Ihr Steuerbüro, wie es diese Rechnung buchen möchte.",
      "Der Hinweis nennt einen Wert, der `passt nicht`? Prüfen Sie ihn mit Ihrem Steuerbüro. Das Erlöskonto darf höchstens so viele Ziffern haben wie Ihre Sachkonten.",
    ],
    related: [
      HELP_TOPICS.traegerFinanceExports,
      HELP_TOPICS.traegerBillingTerms,
      HELP_TOPICS.traegerCareMonths,
    ],
  };
}
