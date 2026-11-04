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
        title: "Befreiungsregel anlegen",
        description:
          "Die Regel gilt für alle Familien mit Leistungsbezug. Sie legen damit keine einzelne Familie an. Welche Familie befreit ist, wählen Sie beim Bestätigen ihrer Anmeldung.",
        steps: [
          "Wählen Sie bei `Befreiung bei Leistungsbezug` den Knopf `Befreiungsregel hinzufügen`.",
          "Tragen Sie ein, ab wann die Regel gilt. `Gilt bis` können Sie frei lassen.",
          "Schreiben Sie den `Vermerk auf dem Beitrag`. Er steht bei jeder befreiten Familie.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einstellen, welches Kind das erste Kind ist",
        description:
          "Das erste Kind zahlt den vollen Preis. Für die anderen Kinder gilt die Ermäßigung.",
        steps: [
          "Wählen Sie in der Navigation `Einstellungen` und dann den Bereich `Ferienbetreuung`.",
          "Wählen Sie bei `Welches Kind ist das erste Kind?`: `Das Kind mit dem höchsten Beitrag` oder `Das älteste Kind`.",
        ],
      },
      {
        title: "Einstellen, was ein Kostenträger bezahlt",
        steps: [
          "Wählen Sie in der Navigation `Einstellungen` und dann den Bereich `Ferienbetreuung`.",
          "Wählen Sie bei `Was bezahlt ein Kostenträger?`: `Betrag nach der Ermäßigung` oder `Vollen Preis`. Beim vollen Preis gilt die Ermäßigung nur für die anderen Kinder.",
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
      "Geschwister sind Kinder derselben Ferienanmeldung oder von Anmeldungen, die das Träger-Büro als Geschwister zusammengeführt hat. Das erste Kind zahlt voll. Am Anfang ist das das Kind mit dem höchsten Preis. Sie können auch das älteste Kind wählen.",
      "Haben zwei Kinder denselben Rang, zählt die früher eingegangene Anmeldung. Danach zählt die Reihenfolge im Formular.",
      "Eine geänderte Einstellung gilt nur für Anmeldungen, die danach bestätigt werden. Ist von Geschwistern schon eine Anmeldung bestätigt, rechnen alle diese Geschwister weiter wie bisher.",
      "Es gelten die Regeln vom ersten gebuchten Tag der Anmeldung.",
      "Eine Regel gilt nur für Anmeldungen, die nach der Änderung bestätigt werden. Bestätigte Anmeldungen behalten ihren Beitrag.",
      "Ob eine Familie befreit ist oder ein Kostenträger bezahlt, entscheidet das Träger-Büro beim Bestätigen. Für die Befreiung wählt es dort `Von der Zahlung befreien (Leistungsbezug)`. Diese Wahl gibt es nur, wenn am ersten gebuchten Tag der Anmeldung eine Befreiungsregel gilt.",
      "Befreiung heißt: Niemand bezahlt. Bei einem Kostenträger bezahlt die Familie nichts, aber der Kostenträger bezahlt den Betrag. Am Anfang ist das der Betrag nach der Geschwisterermäßigung. Sie können auch den vollen Preis wählen.",
      "Bezahlt der Kostenträger den vollen Preis, zählt sein Kind weiter als Geschwisterkind. Die anderen Kinder behalten ihre Ermäßigung.",
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
