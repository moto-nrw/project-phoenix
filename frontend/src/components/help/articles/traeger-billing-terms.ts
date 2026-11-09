import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerBillingTermsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerBillingTerms,
    title: "Zahlung und Empfänger der Rechnungen",
    question: "Wie lege ich fest, wie und bis wann Familien zahlen?",
    summary:
      "Je Leistung legen Sie den Zahlungsweg und das Zahlungsziel fest. Das Mittagessen steht auf der Rechnung der Betreuung oder auf einer eigenen Rechnung. Für eine einzelne Familie legen Sie einen anderen Empfänger, einen eigenen Zahlungsweg oder einen eigenen Zahltag fest.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Users",
    requirements: ["Ihre Rolle im Träger-Büro ist Abrechnung."],
    steps: [],
    instructionGroups: [
      {
        title: "Zahlung je Leistung festlegen",
        description:
          "Ohne Ihre Wahl zahlen alle Familien per Überweisung, innerhalb von 14 Tagen.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen` und dann `Abrechnungsvorgaben`.",
          "In der Karte `Zahlung und Rechnungen` steht je Leistung eine Zeile: `Monatsbeitrag Betreuung`, `Mittagessen` und `Ferienbetreuung`.",
          "Wählen Sie die Zeile der Leistung. Oder wählen Sie das Menü mit den drei Punkten und dann `Ändern`.",
          "Wählen Sie den `Zahlungsweg`: `Überweisung` oder `Lastschrift`.",
          "Tragen Sie das `Zahlungsziel in Tagen` ein. Das sind die Tage nach dem Rechnungsdatum.",
          "Beim Mittagessen wählen Sie bei `Rechnung`: `Auf der Rechnung der Betreuung` oder `Eigene Rechnung`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Fälligkeit eines Laufs festlegen",
        description:
          "Ein Lauf kann ein festes Datum haben, bis zu dem alle Familien zahlen. Das geht nur vor der Freigabe.",
        steps: [
          "Öffnen Sie den Lauf unter `Abrechnungsläufe`, im Reiter `Monatsbeiträge` oder `Ferien`.",
          "Beim Monatslauf wählen Sie in der Karte `Monatslauf` beim Feld `Fällig` den Knopf `Ändern`. Beim Ferienlauf wählen Sie `Fälligkeiten ändern`. Es öffnet sich eine eigene Seite.",
          "Wählen Sie je Zahlungsweg ein Datum. Ohne Datum gilt das Zahlungsziel der Leistung.",
          "Wählen Sie `Speichern`. Sie kommen zurück zum Lauf. Bei `Fällig` steht jetzt das Datum.",
        ],
      },
      {
        title: "Empfänger und Zahlung einer Familie ändern",
        description:
          "Das gilt für alle Rechnungen dieser Familie, auch für Geschwister. Die Familie ist die E-Mail-Adresse der Person im Vertrag.",
        steps: [
          "Öffnen Sie einen Monatslauf unter `Abrechnungsläufe` und `Monatsbeiträge`.",
          "Wählen Sie bei der Rechnung des Kindes das Menü mit den drei Punkten und dann `Empfänger und Zahlung`. Es öffnet sich eine eigene Seite.",
          "Wählen Sie bei `Rechnung an`: `Person im Vertrag`, `Beide Eltern` oder `Andere Person`.",
          "Bei `Beide Eltern` tragen Sie den Namen des zweiten Elternteils ein. Beide Namen stehen dann auf der Rechnung.",
          "Bei `Andere Person` tragen Sie Name, E-Mail-Adresse und Anschrift ein. Die Rechnung geht dann an diese Person.",
          "Wählen Sie bei Bedarf einen eigenen `Zahlungsweg`. `Wie in den Abrechnungsvorgaben` nimmt den dort eingestellten Zahlungsweg.",
          "Wählen Sie bei Bedarf einen `Eigenen Zahltag`, zum Beispiel den 15. des Monats.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Jede Rechnung zeigt den Zahlungsweg und das Datum, an dem sie fällig ist. Steht dahinter `Zahltag der Familie`, kommt das Datum von der Familie. Rechnungen an beide Eltern oder an eine andere Person tragen das Zeichen `Beide Eltern` oder `Abweichender Zahler`.",
    notes: [
      "Die Ferienbetreuung hat immer eine eigene Rechnung. Sie steht nie auf der Rechnung der Betreuung.",
      "Bei `Lastschrift` buchen Sie den Betrag selbst ab. Dafür braucht die Familie ein SEPA-Mandat bei Ihnen, und Sie brauchen eine Gläubiger-ID. Ohne Mandat geht die Rechnung als Überweisung. moto bucht kein Geld ab.",
      "Unter `Familien und Zahlung` sehen Sie bei jeder Familie den Zahlungsweg ihrer Rechnungen.",
      "Beim Erstellen der Schreiben stehen die Fälligkeiten bereits fest. Das Feld `Zahlen bis` gilt nur für ältere Rechnungen ohne eigenes Datum.",
      "Welches Datum gilt: zuerst der eigene Zahltag der Familie, dann das Datum des Laufs, sonst das Zahlungsziel der Leistung.",
    ],
    differences: [
      "Eine Rechnung zeigt keinen Zahlungsweg? Sie wurde ausgestellt, bevor es diese Angaben gab.",
      "Bei einem freigegebenen Lauf fehlt `Ändern` beim Feld `Fällig`? Ausgestellte Rechnungen ändern sich nicht mehr.",
      "Ändern Sie eine Fälligkeit, nachdem Sie die Prüfliste geprüft haben, lehnt moto die Freigabe ab. Laden Sie die Seite neu und prüfen Sie den Lauf noch einmal.",
    ],
    troubleshootingDetails: [
      "Änderungen erscheinen nicht auf einer Rechnung? Ausgestellte Rechnungen bleiben, wie sie sind. Die Änderung gilt ab dem nächsten Lauf. Eine falsche Rechnung stornieren Sie und stellen sie neu aus.",
      "`Empfänger und Zahlung` lässt sich nicht speichern, weil eine E-Mail-Adresse fehlt? Im Vertrag steht keine E-Mail-Adresse. Tragen Sie sie im Vertrag ein.",
    ],
    related: [
      HELP_TOPICS.traegerSepaMandates,
      HELP_TOPICS.traegerCareMonths,
      HELP_TOPICS.traegerBillingRun,
    ],
  };
}
