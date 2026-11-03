import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/beitraege` (#4664, #5627): Regeln des Ferienbeitrags und Kostenträger. */
export function traegerHolidayContributionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayContribution,
    title: "Ferienbeitrag festlegen",
    question: "Wie rechnet moto den Ferienbeitrag?",
    summary:
      "Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Geschwister zahlen weniger. Bei Leistungsbezug zahlt die Familie nichts.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Landmark",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Ermäßigungen`.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Preis festlegen",
        steps: [
          "Den Preis jeder Woche legen Sie im Ferienzeitraum fest. Jedes Zeitmodell hat einen `Preis in Euro`.",
        ],
      },
      {
        title: "Geschwisterregel anlegen",
        steps: [
          "Wählen Sie in der Navigation `Ermäßigungen`.",
          "Wählen Sie bei `Geschwister` den Knopf `Geschwisterregel hinzufügen`.",
          "Tragen Sie ein, ab wann die Regel gilt. `Gilt bis` können Sie frei lassen.",
          "Wählen Sie `ab dem 2. Kind` oder `ab dem 3. Kind`.",
          "Wählen Sie `Anteil in Prozent` oder `Betrag in Euro` und tragen Sie den Wert ein. Zum Beispiel 50 für die Hälfte.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Befreiung bei Leistungsbezug anlegen",
        steps: [
          "Wählen Sie bei `Befreiung bei Leistungsbezug` den Knopf `Befreiung hinzufügen`.",
          "Tragen Sie ein, ab wann die Befreiung gilt.",
          "Schreiben Sie den `Vermerk auf dem Beitrag`. Er steht bei jeder befreiten Familie.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Kostenträger anlegen",
        description:
          "Ein Kostenträger bezahlt den Beitrag für eine Familie, zum Beispiel das Jugendamt bei Leistungsbezug. Sie fordern den Betrag dann beim Kostenträger an.",
        steps: [
          "Wählen Sie bei `Kostenträger` den Knopf `Kostenträger hinzufügen`.",
          "Tragen Sie den `Name` ein. `Anschrift` und `Ihr Aktenzeichen beim Kostenträger` sind freiwillig.",
          "Wählen Sie `Speichern`.",
          "Gibt es den Kostenträger nicht mehr, wählen Sie im Menü der Zeile `Ausschalten`. Beim Bestätigen steht er dann nicht mehr zur Wahl.",
        ],
      },
    ],
    result:
      "Eltern sehen den Preis jedes Zeitmodells schon bei der Wahl der Wochen. Den Beitrag sehen sie vor dem Absenden der Ferienanmeldung. Beim Bestätigen hält moto den Beitrag fest.",
    notes: [
      "Geschwister sind Kinder derselben Ferienanmeldung oder von Anmeldungen, die das Träger-Büro als Geschwister zusammengeführt hat. Das Kind mit dem höchsten Preis ist das erste Kind und zahlt voll.",
      "Es gelten die Regeln vom ersten gebuchten Tag der Anmeldung.",
      "Eine Regel gilt nur für Anmeldungen, die nach der Änderung bestätigt werden. Bestätigte Anmeldungen behalten ihren Beitrag.",
      "Ob eine Familie befreit ist oder ein Kostenträger bezahlt, entscheidet das Träger-Büro beim Bestätigen.",
      "Befreiung heißt: Niemand bezahlt. Bei einem Kostenträger bezahlt die Familie nichts, aber der Kostenträger bezahlt den Betrag nach der Geschwisterermäßigung.",
      "Einen Kostenträger, der schon gewählt wurde, können Sie nur ausschalten, nicht löschen. Ändern Sie seinen Namen, behalten bestätigte Beiträge und Rechnungen den alten Namen.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Ermäßigungen`? Ihre Rolle ist nicht Abrechnung. Fragen Sie im Träger-Büro nach dieser Rolle.",
    ],
    troubleshootingDetails: [
      "Die Regel lässt sich nicht speichern, weil schon eine andere gilt? Von jeder Art gilt an einem Tag nur eine Regel. Setzen Sie bei der alten Regel `Gilt bis` auf den Tag davor.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayRegistrationReview,
      HELP_TOPICS.traegerBillingRun,
    ],
  };
}
