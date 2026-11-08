import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareMonthsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareMonths,
    title: "Monatsbeiträge abrechnen",
    question: "Wie rechne ich die Betreuung jeden Monat ab?",
    summary:
      "Ein Monatslauf rechnet einen Monat nach den gültigen Verträgen ab. Der Beitrag eines Kindes ist die Summe seiner gebuchten Wochentage, mit den Preisen aus der Preisliste. Es zählt der Vertrag, nicht die Anwesenheit. Nach der Freigabe erstellen Sie die Schreiben, senden sie an die Familien und stornieren eine falsche Rechnung.",
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
          "Die Familien sehen davon noch nichts. Sie bekommen erst das Schreiben, das Sie senden.",
        ],
      },
      {
        title: "Schreiben erstellen",
        description:
          "Jede Familie bekommt ein Schreiben im Briefkopf des Trägers. Es ist ihre Rechnung. Erstellen sendet noch nichts.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Schreiben an die Familien` den Knopf `Schreiben erstellen`. Die Seite mit den Schreiben des Laufs öffnet sich.",
          "Wählen Sie dort bei `Schreiben an die Familien` noch einmal `Schreiben erstellen`.",
          "Wählen Sie bei `Zahlen bis` das Datum, bis zu dem die Familien zahlen.",
          "Schreiben Sie bei `Hinweis für die Familien` etwas, wenn alle Familien es lesen sollen. Das ist freiwillig.",
          "Wählen Sie `Vorschau ansehen`. Sie sehen das Schreiben der ersten Familie.",
          "Wählen Sie `Schreiben erstellen`. Danach lassen sich die Schreiben nicht mehr ändern.",
        ],
      },
      {
        title: "Schreiben senden und drucken",
        description:
          "Jede Familie bekommt ihr Schreiben per E-Mail. Hat sie mit dieser Adresse ein Konto in der Eltern-App, liegt es auch dort unter `Dokumente`.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Schreiben an die Familien` den Knopf `Schreiben öffnen`. Dort steht schon, wie viele Schreiben gesendet sind.",
          "Wählen Sie bei `Schreiben an die Familien` den Knopf `An alle senden`. Prüfen Sie die Zahl und bestätigen Sie, zum Beispiel mit `3 Schreiben senden`.",
          "Die Karte zeigt, wie viele Schreiben gesendet sind, gerade gesendet werden, nicht angekommen sind, noch nicht gesendet sind und keine E-Mail-Adresse haben.",
          "In jeder Rechnung steht unter `An die Familie`, ob ihr Schreiben gesendet ist. `Ausgestellt am` sagt nur, dass die Rechnung ihre Nummer hat.",
          "Ist ein Schreiben nicht angekommen, wählen Sie bei der Rechnung das Menü mit den drei Punkten und dann `Erneut senden`. Mit `Senden` schicken Sie ein einzelnes Schreiben zum ersten Mal.",
          "Mit `Schreiben ansehen` im Menü sehen Sie genau das Schreiben, das die Familie bekommt.",
          "Für Familien ohne E-Mail wählen Sie `Drucken` und dann `Nur Familien ohne E-Mail`. Geben Sie ihnen das gedruckte Schreiben. `Alle Schreiben drucken` druckt alle Schreiben, die noch gelten.",
        ],
      },
      {
        title: "Rechnung stornieren und neu ausstellen",
        description:
          "Eine ausgestellte Rechnung ändert sich nie. Stimmt sie nicht, stornieren Sie sie. Der Storno bekommt eine eigene Nummer.",
        steps: [
          "Wählen Sie im freigegebenen Lauf `Schreiben öffnen`. Gibt es noch keine Schreiben, heißt der Knopf `Schreiben erstellen`.",
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten und dann `Stornieren`.",
          "Schreiben Sie bei `Begründung`, warum Sie stornieren. Wählen Sie `Stornieren`.",
          "Hatte die Familie die Rechnung schon bekommen, bekommt sie ein Storno-Schreiben. Es steht als eigene Zeile unter der Rechnung. Senden Sie es über das Menü dieser Zeile mit `Senden`.",
          "Haben Sie die Rechnung zum Drucken heruntergeladen, bekommt der Storno auch ein Schreiben. Laden Sie es über `Drucken` und `Alle Schreiben drucken` herunter. Geben Sie es der Familie auf Papier.",
          "Wurde die Rechnung weder gesendet noch zum Drucken heruntergeladen, bekommt die Familie kein Storno. Sie sieht nichts davon.",
          "Wählen Sie in der Meldung über den Rechnungen `Neu ausstellen`. Ein neuer Monatslauf für denselben Monat öffnet sich. Er zeigt die neue Rechnung als Vorschau. Geben Sie ihn frei und senden Sie das neue Schreiben. Danach verschwindet die Meldung zur Neuausstellung im alten Lauf.",
        ],
      },
    ],
    result:
      "Die Rechnungen des Monats sind ausgestellt, und die Familien haben ihr Schreiben per E-Mail oder auf Papier. Ein späterer Lauf desselben Monats rechnet nur Kinder ab, die noch keine gültige Rechnung für diesen Monat haben.",
    differences: [
      "Beginnt, endet oder ändert sich ein Vertrag im Monat, zählt standardmäßig der Vertrag am 1. des Monats für den ganzen Monat. Beginnt die Betreuung erst im Monat, zählt der erste Vertrag den ganzen Monat. Ein weiterer Vertrag, der später im Monat beginnt, zählt erst ab dem nächsten Monat. Anteilig nach Kalendertagen stellen Sie unter `Einstellungen`, `Allgemein`, Bereich `Monatsbeitrag` ein.",
      "Mit einem Preis, für den es schon eine Rechnung gibt, rechnet der Monat weiter ab. Ändern Sie ihn ab einem späteren Monat. Fehlende Preise ergänzen Sie auch für einen schon abgerechneten Monat. Der nächste Lauf dieses Monats rechnet die Kinder dann ab.",
    ],
    notes: [
      "Ferienpreise legen Sie im Ferienzeitraum fest. Die Preisliste gilt nur für die Monatsbeiträge.",
    ],
    troubleshootingDetails: [
      "`Schreiben erstellen` geht nicht: Es fehlt eine Bankverbindung. Tragen Sie sie unter `Einstellungen`, `Abrechnungsvorgaben` ein.",
      "Ein Schreiben geht nicht raus, weil die Person nicht mehr im Vertrag steht: Stornieren Sie die Rechnung. Wählen Sie dann `Neu ausstellen`. Die neue Rechnung geht an die Person im Vertrag.",
      "`Erneut senden` fehlt: Die Mail ist noch unterwegs, oder die Adresse ist gesperrt. Der Hinweis bei der Rechnung sagt, was zu tun ist. Drucken Sie das Schreiben, wenn die Adresse gesperrt ist.",
    ],
    related: [HELP_TOPICS.traegerCareContracts, HELP_TOPICS.traegerBillingRun],
  };
}
