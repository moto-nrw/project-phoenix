import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerDatevExportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDatevExport,
    title: "DATEV-Werte eintragen",
    question: "Wie bekommt mein Steuerbüro die Beiträge für DATEV?",
    summary:
      "Für jeden ausgestellten Lauf laden Sie eine DATEV-Datei herunter. Sie enthält nur Buchungen, die noch in keiner DATEV-Datei waren. Ihr Steuerbüro liest sie in DATEV ein. Vorher tragen Sie einmal die DATEV-Werte Ihres Steuerbüros ein.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Settings",
    requirements: [
      "Ihre Rolle im Träger-Büro hat das Recht Finanz-Export.",
      "Ihr Steuerbüro hat Ihnen die DATEV-Werte genannt.",
      "Bei Monatsbeiträgen hat jede Familie eine Debitorennummer. Bei Ferienbeiträgen steht sie in der Ferienanmeldung.",
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
          "Tragen Sie das `Standard-Erlöskonto` ein. Darauf bucht moto alles ohne eigenes Konto.",
        ],
      },
      {
        title: "Erlöskonten und Kostenstellen festlegen",
        description:
          "Nur nötig, wenn Ihr Steuerbüro Leistungen, Schulen oder Angebote getrennt bucht.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie oben rechts `Konten und Kostenstellen`.",
          "Unter `Erlöskonten` wählen Sie bei `Monatsbeiträge`, `Mensa` oder `Ferienbetreuung` `Ändern`. Tragen Sie das Konto ein und wählen Sie `Speichern`. Ein Angebot kann ein eigenes Konto haben.",
          "Unter `Kostenstellen (KOST1)` wählen Sie bei der Schule `Neue Kostenstelle`. Wählen Sie den Tag bei `Gilt ab` und tragen Sie die `Kostenstelle` ein.",
          "Eine Ferienwoche bucht auf die Kostenstelle der Schule, an der sie stattfindet. Die Kostenstelle bei `Ferienbetreuung` ist der Ersatz, wenn diese Schule keine hat.",
          "Eine `Zweite Kostenstelle (KOST2)` je Angebot tragen Sie nur ein, wenn Ihr Steuerbüro sie braucht.",
        ],
      },
      {
        title: "Neue Buchungen herunterladen",
        description:
          "Die Spalte `DATEV-Datei` zeigt, wie viele neue Buchungen ein Lauf hat.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie beim Lauf `DATEV`. Die Datei wird heruntergeladen. Sie enthält nur die neuen Buchungen.",
          "Geben Sie die Datei an Ihr Steuerbüro.",
        ],
      },
      {
        title: "Alle Buchungen erneut herunterladen",
        description:
          "Nur wenn Ihr Steuerbüro eine frühere Datei nicht hat oder nicht einlesen konnte.",
        steps: [
          "Wählen Sie beim Lauf das Menü mit den drei Punkten neben `DATEV`.",
          "Wählen Sie `Alle Buchungen erneut`.",
          "Lesen Sie den Hinweis. Hat der Lauf Buchungen in mehreren Wirtschaftsjahren, wählen Sie das `Wirtschaftsjahr`.",
          "Wählen Sie `Erneut herunterladen`.",
        ],
      },
    ],
    result:
      "In der Datei steht jede Rechnung im Soll, jedes Storno und jede Gutschrift im Haben. Die Datei steht im `Exportprotokoll` mit ihrer Nummer. Eine Datei mit allen Buchungen steht dort als `Alle erneut: Wiederholung`. In jeder Rechnung zeigt `Verlauf`, in welcher Datei sie war.",
    notes: [
      "Bei Monatsbeiträgen tragen Sie die Debitorennummer bei `Empfänger und Zahlung` der Familie ein. Bei Ferienbeiträgen tragen Sie sie in der Ferienanmeldung ein. DATEV nimmt nur Ziffern an, eine mehr als bei den Sachkonten. Bei 4 Stellen ist das zum Beispiel `10001`.",
      "Jede Buchung kommt nur einmal in eine DATEV-Datei. Stornieren Sie später eine Rechnung, steht das Storno in der nächsten Datei des Laufs. Die Rechnung selbst steht dort nicht noch einmal.",
      "Eine DATEV-Datei gilt für ein Wirtschaftsjahr. Liegt ein Storno im neuen Jahr, kommt es in eine eigene Datei. Laden Sie dafür noch einmal `DATEV` herunter.",
      "Hat ein Lauf keine neuen Buchungen, lässt sich `DATEV` nicht wählen. Dann entsteht keine Datei und keine Nummer.",
      "Die DATEV-Datei und die Sollstellung zählen in derselben Reihe von Dateinummern.",
      "Eine Rechnung über 0 € steht nicht in der DATEV-Datei. Sie bewegt kein Geld.",
      "Eine Rechnung hält Erlöskonto und Kostenstellen beim Ausstellen fest. Ändern Sie später ein Konto, gilt das nur für neue Rechnungen. Ein Storno bucht auf dieselben Konten wie seine Rechnung.",
      "Hat eine Rechnung Positionen auf verschiedenen Konten, steht sie mit einer Zeile je Konto in der DATEV-Datei. Ermäßigungen verteilen sich anteilig auf diese Zeilen.",
      "Eine Kostenstelle gilt ab ihrem Tag bis zur nächsten. Maßgeblich ist der erste Tag der Leistung, also der Monat oder die Ferienwoche.",
    ],
    differences: [
      "Über der Liste steht `Die DATEV-Datei fehlt noch`? Es fehlen DATEV-Werte. Der Hinweis nennt sie. Bis dahin lässt sich `DATEV` nicht wählen.",
      "Beim Lauf steht `Alles exportiert`? Alle Buchungen sind schon in einer DATEV-Datei. Darunter steht ihre Nummer.",
      "Sie können die DATEV-Werte nur ansehen? Ändern darf, wer im Träger-Büro das Recht für die Einstellungen hat.",
      "Im `Exportprotokoll` steht `… mit Ersatz-Kostenstelle`? Diese Ferienwochen fanden an einer Schule ohne Kostenstelle statt. Die Datei bucht sie auf die Kostenstelle der `Ferienbetreuung`. Darunter stehen Rechnung, Woche und Grund. Tragen Sie für die Schule eine Kostenstelle ein. Das gilt für neue Rechnungen.",
      "Im `Exportprotokoll` steht `… mit Konten vom Exporttag`? Diese Rechnungen wurden ausgestellt, bevor moto die Konten festgehalten hat. Die Datei nutzt dann die Werte vom Tag des Exports.",
    ],
    troubleshootingDetails: [
      "Es erscheint `Keine DATEV-Datei für …` mit ungültigen Debitorennummern? Eine ausgestellte Rechnung behält ihre Debitorennummer. Bei Monatsbeiträgen tragen Sie bei der Familie die richtige Nummer ein. Stornieren Sie dann die genannte Rechnung und stellen Sie sie neu aus. Bei Ferienbeiträgen ohne Gutschrift stornieren Sie zuerst die Rechnung. Korrigieren Sie die Debitorennummer in der Ferienanmeldung. Stellen Sie dann die Rechnung neu aus. Waren Rechnung und Storno in keiner DATEV-Datei, heben sie sich auf. Danach lässt sich `DATEV` für den Lauf wieder wählen. Die neue Rechnung kommt mit ihrem Lauf in DATEV.",
      "Hat eine Ferienrechnung mit ungültiger Debitorennummer schon eine Gutschrift? Dann ist kein Storno möglich. Laden Sie die Sollstellung des Laufs für Ihr Steuerbüro herunter. Es kann die Buchungen daraus manuell in DATEV erfassen. Der DATEV-Download dieses Laufs bleibt gesperrt.",
      "Es erscheint `Keine neuen Buchungen für …`? Jemand hat die Buchungen gerade heruntergeladen. Die Datei steht im `Exportprotokoll`.",
      "Der Hinweis nennt einen Wert, der `passt nicht`? Prüfen Sie ihn mit Ihrem Steuerbüro. Das Erlöskonto darf höchstens so viele Ziffern haben wie Ihre Sachkonten.",
      "Es erscheint `Erlöskonto ist länger als Ihre Sachkonten` oder `Kostenstelle fehlt`? Schon festgehaltene Konten und Kostenstellen bleiben unverändert. Auch ein Storno übernimmt diese Werte. Laden Sie dann die Sollstellung dieses Laufs herunter. Geben Sie die Datei und den Hinweis an Ihr Steuerbüro. Es kann die genannten Werte berichtigen und manuell in DATEV buchen. Der DATEV-Download dieses Laufs bleibt dann gesperrt. Korrigieren Sie unter `Konten und Kostenstellen` die Werte für neue Rechnungen.",
      "Ist in den DATEV-Werten `Kostenstelle ist Pflicht` an? Dann braucht jede Schule eine Kostenstelle. Sonst gibt es keine DATEV-Datei.",
    ],
    related: [
      HELP_TOPICS.traegerFinanceExports,
      HELP_TOPICS.traegerBillingTerms,
      HELP_TOPICS.traegerCareMonths,
    ],
  };
}
