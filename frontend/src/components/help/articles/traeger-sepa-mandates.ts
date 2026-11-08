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
          "Die Gläubiger-ID bekommen Sie von der Bundesbank. Ohne sie gibt es keine Lastschrift.",
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
      "`Verlauf ansehen` zeigt, wer das Mandat wann angesehen oder geändert hat. Kontodaten stehen dort nicht.",
    ],
    differences: [
      "Die Familie hat kein gültiges Mandat? Mit `Als Überweisung ausstellen` bekommt sie eine Rechnung zum Überweisen. Die Prüfung des Monatslaufs zeigt sie in der Karte `Lastschrift`.",
      "Mit `Freigabe anhalten` lässt sich der Lauf erst freigeben, wenn jede Familie mit Lastschrift ein gültiges Mandat hat.",
      "`Mandat erfassen` fehlt? Die Familie hat schon ein gültiges Mandat. Widerrufen Sie es zuerst.",
    ],
    troubleshootingDetails: [
      "Die IBAN wird nicht angenommen? Prüfen Sie jedes Zeichen. moto prüft die Prüfziffer.",
      "Die Prüfung meldet `früher als Ihre Vorabankündigung erlaubt`? Die Abbuchung kommt zu früh nach dem Rechnungsdatum. Ändern Sie die Fälligkeit des Laufs oder das Rechnungsdatum.",
    ],
    related: [HELP_TOPICS.traegerBillingTerms, HELP_TOPICS.traegerCareMonths],
  };
}
