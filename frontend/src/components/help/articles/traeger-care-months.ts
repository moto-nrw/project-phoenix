import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareMonthsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareMonths,
    title: "Monatsbeiträge abrechnen",
    question: "Wie rechne ich die Betreuung jeden Monat ab?",
    summary:
      "Ein Monatslauf rechnet einen Monat nach den gültigen Verträgen ab. Der Beitrag eines Kindes ist die Summe seiner gebuchten Wochentage, mit den Preisen aus der Preisliste. Es zählt der Vertrag, nicht die Anwesenheit.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarCheck",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Abrechnung`.",
      "Die Kinder haben einen gültigen Betreuungsvertrag. Module und gebuchte Wochentage kommen aus dem Vertrag.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Preise festlegen",
        description:
          "Jede Schule hat eigene Preise. Ein Preis gilt für einen gebuchten Wochentag eines Moduls, für den ganzen Monat.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Preisliste`.",
          "Wählen Sie bei der Schule `Neue Preise`.",
          "Wählen Sie bei `Gültig ab` den Monat, ab dem die Preise gelten.",
          "Tragen Sie je Modul und Wochentag den Monatspreis ein, zum Beispiel 32,00. Ein leeres Feld setzt keinen Preis. Dann gilt der Preis davor.",
          "Wählen Sie `Preise speichern`.",
        ],
      },
      {
        title: "Rechnungsnummern festlegen",
        description: "Das machen Sie einmal vor der ersten Freigabe.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen` und dann `Abrechnungsvorgaben`.",
          "Wählen Sie bei `Rechnungsnummern der Monatsbeiträge` den Knopf `Muster festlegen`.",
          "Tragen Sie ein Muster ein, zum Beispiel MB-{JJJJ}-{NNNN}. Nehmen Sie ein anderes Kürzel als bei den Ferien.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Monatslauf starten und prüfen",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Monatsbeiträge`.",
          "Wählen Sie `Monatslauf starten`, dann den Monat und `Lauf starten`.",
          "Prüfen Sie die Rechnungen. Je Schule steht eine Karte. Jede Rechnung hat je Modul eine Zeile mit den gebuchten Wochentagen.",
          "Steht oben `Ohne Preis`, fehlt für einen gebuchten Wochentag ein Preis. Wählen Sie `Preis eintragen`. Danach kommt das Kind in den Lauf.",
          "Steht oben `Doppelter Vertrag – bitte prüfen`, rechnen zwei Verträge eines Kindes am selben Tag dasselbe Modul ab. Die Freigabe rechnet dieses Kind mit allen seinen Verträgen nicht ab, alle anderen Kinder schon. Nach der Freigabe steht es unter `Nicht abgerechnet – Doppelter Vertrag`. Beenden oder ändern Sie einen der Verträge. Starten Sie dann einen neuen Monatslauf für diesen Monat. Er rechnet nur dieses Kind ab.",
        ],
      },
      {
        title: "Monatslauf freigeben",
        steps: [
          "Wählen Sie `Lauf freigeben`.",
          "Prüfen Sie Zahl und Summe der Rechnungen und wählen Sie `Freigeben`.",
          "Jede Rechnung bekommt ihre Nummer. Danach ändert sie sich nicht mehr.",
        ],
      },
    ],
    result:
      "Die Rechnungen des Monats sind ausgestellt. Ein späterer Lauf desselben Monats rechnet nur Kinder ab, die noch keine Rechnung für diesen Monat haben.",
    differences: [
      "Beginnt, endet oder ändert sich ein Vertrag im Monat, zählt standardmäßig der Vertrag am 1. des Monats für den ganzen Monat. Ein Vertrag, der später im Monat beginnt, zählt erst ab dem nächsten Monat. Anteilig nach Kalendertagen stellen Sie unter `Einstellungen`, `Allgemein`, Bereich `Monatsbeitrag` ein.",
      "Mit einem Preis, für den es schon eine Rechnung gibt, rechnet der Monat weiter ab. Ändern Sie ihn ab einem späteren Monat. Fehlende Preise ergänzen Sie auch für einen schon abgerechneten Monat. Der nächste Lauf dieses Monats rechnet die Kinder dann ab.",
    ],
    notes: [
      "Ferienpreise legen Sie im Ferienzeitraum fest. Die Preisliste gilt nur für die Monatsbeiträge.",
    ],
    related: [HELP_TOPICS.traegerCareContracts, HELP_TOPICS.traegerBillingRun],
  };
}
