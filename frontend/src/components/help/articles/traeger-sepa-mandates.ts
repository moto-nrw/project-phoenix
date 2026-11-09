import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerSepaMandatesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSepaMandates,
    title: "Lastschrift und SEPA-Mandate",
    question: "Wie buche ich Beiträge per Lastschrift ab?",
    summary:
      "Sie tragen Ihre Gläubiger-ID ein und erfassen das SEPA-Mandat jeder Familie. Die Rechnung kündigt die Abbuchung dann an: mit Betrag, Datum, Konto, Gläubiger-ID und Mandatsreferenz. moto bucht kein Geld ab.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Repeat",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Der Zahlungsweg der Leistung oder der Familie ist `Lastschrift`.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Gläubiger-ID eintragen",
        description:
          "Die Gläubiger-ID bekommen Sie von der Bundesbank. Ohne sie gibt es keine Lastschrift und kein Mandat. Ein Mandat gilt nur für die Gläubiger-ID, die beim Erfassen eingetragen war. Ändern Sie die Gläubiger-ID, brauchen alle Familien ein neues Mandat.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen` und dann `Abrechnungsvorgaben`.",
          "Wählen Sie in der Karte `Lastschrift` den Knopf `Ändern`.",
          "Tragen Sie die `Gläubiger-ID` ein.",
          "Bei `Vorabankündigung in Tagen` steht 14. So viele Tage vor der Abbuchung muss die Rechnung bei der Familie sein. Weniger Tage nur, wenn Sie das mit den Familien vereinbart haben.",
          "Wählen Sie bei `Wenn eine Familie kein gültiges Mandat hat`: `Als Überweisung ausstellen` oder `Freigabe anhalten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Mandat einer Familie erfassen",
        description:
          "Je Familie gilt ein Mandat. Es gilt auch für Geschwister mit derselben E-Mail-Adresse im Vertrag.",
        steps: [
          "Öffnen Sie den Vertrag des Kindes unter `Verträge`. Oder öffnen Sie im Monatslauf bei der Rechnung das Menü mit den drei Punkten und dann `Empfänger und Zahlung`.",
          "Wählen Sie in der Karte `SEPA-Lastschriftmandat` den Knopf `Mandat erfassen`. Es öffnet sich die Seite des Mandats.",
          "Wählen Sie dort `Mandat erfassen`.",
          "Übernehmen Sie `Kontoinhaber`, `IBAN` und `Unterschrieben am` aus dem unterschriebenen Mandat. Die `BIC` brauchen Sie nur bei Konten außerhalb der EU.",
          "Tragen Sie die `Mandatsreferenz` ein. Lassen Sie das Feld leer, vergibt moto eine.",
          "Wählen Sie die `Art`: `Dauermandat` für jeden Monat oder `Einzelmandat` für eine Abbuchung.",
          "Wählen Sie bei `Wurde mit diesem Mandat schon abgebucht?` die passende Antwort. Bei einem Mandat aus einem anderen Programm meistens `Ja, schon einmal`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Bestehende Mandate importieren",
        description:
          "Haben Sie die Mandate schon in einem anderen Programm, zum Beispiel OGS Connect? Dann übernehmen Sie alle auf einmal aus einer CSV-Datei. Referenz und Datum der Unterschrift bleiben gleich. moto bewahrt die Datei nicht auf.",
        steps: [
          "Speichern Sie die Mandate im anderen Programm als CSV-Datei. Die erste Zeile enthält die Überschriften.",
          "Öffnen Sie unter `Abrechnung` den Reiter `Lastschrift` und wählen Sie `Mandate importieren`.",
          "Ziehen Sie die Datei in das Feld oder wählen Sie `Datei wählen`.",
          "Prüfen Sie unter `Spalten zuordnen`, aus welcher Spalte jedes Feld kommt. moto schlägt passende Spalten vor.",
          "Wählen Sie bei `Die Spalte „Zahler“ enthält`, ob dort die E-Mail der Familie oder die Vertragsnummer steht.",
          "Hat Ihre Datei keine Spalte mit der Gläubiger-ID? Dann bestätigen Sie, dass alle Mandate zu Ihrer Gläubiger-ID gehören.",
          "Prüfen Sie die Zeilen unter `Prüfen und übernehmen`. Bei `Bitte klären` wählen Sie unter `Schon abgebucht?` die Antwort `Ja` oder `Nein`.",
          "Wählen Sie `Mandate übernehmen`.",
        ],
      },
      {
        title: "Mandat widerrufen",
        steps: [
          "Öffnen Sie unter `Abrechnung` im Reiter `Lastschrift` die Familie. Oder wählen Sie beim Vertrag `Mandat ansehen`.",
          "Wählen Sie beim Mandat das Menü mit den drei Punkten und dann `Widerrufen`.",
          "Wählen Sie bei `Widerruf erhalten am` den Tag, an dem der Widerruf kam.",
          "Wählen Sie `Widerrufen`.",
        ],
      },
    ],
    result:
      "Im nächsten Monatslauf steht auf jeder Rechnung mit Lastschrift die Vorabankündigung: Betrag, Tag der Abbuchung, Konto (verdeckt), Gläubiger-ID und Mandatsreferenz. Der Betrag ist das, was die Familie zahlt, also ohne den Anteil eines Kostenträgers.",
    notes: [
      "Alle Mandate sehen Sie unter `Abrechnung` im Reiter `Lastschrift`.",
      "Die IBAN sehen Sie nach dem Speichern nur verdeckt. So steht sie auch auf der Rechnung.",
      "`Verlauf ansehen` zeigt, wer das Mandat wann angesehen oder geändert hat. Bei importierten Mandaten stehen dort auch Dateiname und Zeile. Kontodaten stehen dort nicht.",
      "Beim Import übernimmt moto nur Zeilen mit dem Stand `Bereit`. Zeilen mit Fehlern oder Konflikten bleiben draußen. Korrigieren Sie die Datei und laden Sie sie noch einmal: Was schon da ist, zeigt moto als `Schon vorhanden`.",
      "Ein Mandat aus der Datei, mit dem schon abgebucht wurde, übernimmt moto als Folgelastschrift. Steht die letzte Abbuchung in der Datei, zählen die 36 Monate bis zum Verfall ab diesem Tag. Fehlt sie, zählen sie ab dem Import.",
    ],
    differences: [
      "Die Familie hat kein gültiges Mandat? Mit `Als Überweisung ausstellen` bekommt sie eine Rechnung zum Überweisen. Die Prüfung des Monatslaufs zeigt sie in der Karte `Lastschrift`.",
      "Mit `Freigabe anhalten` lässt sich der Lauf erst freigeben, wenn jede Familie mit Lastschrift ein gültiges Mandat hat.",
      "`Mandat erfassen` fehlt? Die Familie hat schon ein gültiges Mandat. Widerrufen Sie es zuerst.",
    ],
    troubleshootingDetails: [
      "Die Mandate sind übernommen, aber die Liste der Importe fehlt? Wählen Sie dort `Erneut laden`. Die Mandate müssen Sie nicht noch einmal übernehmen.",
      "Die IBAN wird nicht angenommen? Prüfen Sie jedes Zeichen. moto prüft die Prüfziffer.",
      "Beim Import steht `Konflikt`? Das Mandat passt nicht zu dem, was moto schon hat: eine andere Gläubiger-ID, eine schon vergebene Mandatsreferenz oder eine Familie mit einem anderen gültigen Mandat. moto überschreibt nichts. Klären Sie das Mandat und erfassen Sie es bei Bedarf von Hand.",
      "Beim Import steht `Zu diesem Zahler gibt es bei Ihnen keinen Vertrag`? Prüfen Sie die E-Mail-Adresse oder Vertragsnummer in der Datei. Sie muss zu einem Vertrag Ihres Trägers passen.",
      "Die Prüfung meldet `früher als Ihre Vorabankündigung erlaubt`? Die Abbuchung kommt zu früh nach dem Rechnungsdatum. Ändern Sie die Fälligkeit des Laufs oder das Rechnungsdatum.",
    ],
    related: [
      HELP_TOPICS.traegerSepaDebitFile,
      HELP_TOPICS.traegerBillingTerms,
      HELP_TOPICS.traegerCareMonths,
    ],
  };
}
