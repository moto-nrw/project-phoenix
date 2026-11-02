import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/beitraege` (#4664): Regeln des Ferienbeitrags. */
export function traegerHolidayContributionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayContribution,
    title: "Ferienbeitrag festlegen",
    question: "Wie rechnet moto den Ferienbeitrag?",
    summary:
      "Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Geschwister zahlen weniger, Familien mit Leistungsbezug nichts.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Landmark",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Beiträge`.",
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
          "Wählen Sie in der Navigation `Beiträge`.",
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
    ],
    result:
      "Eltern sehen den Preis jedes Zeitmodells schon bei der Wahl der Wochen. Den Beitrag sehen sie vor dem Absenden der Ferienanmeldung. Beim Bestätigen hält moto den Beitrag fest.",
    notes: [
      "Geschwister sind Kinder derselben Ferienanmeldung. Das Kind mit dem höchsten Preis ist das erste Kind und zahlt voll.",
      "Es gelten die Regeln vom ersten gebuchten Tag der Anmeldung.",
      "Eine Regel gilt nur für Anmeldungen, die nach der Änderung bestätigt werden. Bestätigte Anmeldungen behalten ihren Beitrag.",
      "Ob eine Familie befreit ist, entscheidet das Träger-Büro beim Bestätigen.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Beiträge`? Ihre Rolle ist nicht Abrechnung. Fragen Sie im Träger-Büro nach dieser Rolle.",
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
