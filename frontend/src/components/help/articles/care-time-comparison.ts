import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Soll und Ist (#4774): beim Kind im Bereich `Betreuungszeiten`
 * (`StudentCareTimesSection`), fuer die Leitung als Liste in der `Statistik`
 * (`CareTimeOverrunsSection`). Die Definitionen stehen auch auf dem Schirm.
 */
export function careTimeComparisonTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.careTimeComparison,
    title: "Soll und Ist vergleichen",
    question: "War ein Kind länger da als gebucht?",
    summary:
      "Soll und Ist vergleicht die gebuchte Betreuungszeit mit der Zeit, in der das Kind da war.",
    group: "auswertung",
    audience: ["caregiver", "lead"],
    icon: "Hourglass",
    steps: [],
    instructionGroups: [
      {
        title: "Bei einem Kind nachsehen",
        steps: [
          "Öffnen Sie das Kind und wählen Sie den Bereich `Betreuungszeiten`.",
          "Scrollen Sie nach unten zu `Soll und Ist`.",
          "Wählen Sie den Zeitraum. Am Anfang ist es der laufende Monat.",
          "Lesen Sie je Tag `Gebucht`, `Tatsächlich` und `Abweichung`.",
        ],
      },
      {
        title: "Alle Kinder mit Überschreitungen sehen",
        description: "Nur für die Leitung.",
        steps: [
          "Öffnen Sie in der Seitenleiste `Verwaltung` und danach `Statistik`.",
          "Wählen Sie oben den Bereich `Soll und Ist`.",
          "Wählen Sie den Zeitraum und, wenn nötig, Klassen oder Gruppen.",
          "Tippen Sie auf eine Spalte, um nach ihr zu sortieren.",
          "Über das Menü mit den drei Punkten geht `Exportieren` als `PDF` oder `Excel`.",
        ],
      },
    ],
    result:
      "Überschreitungen tragen den Text `Außerhalb der Buchung` oder `Da ohne Buchung`. Ihre `Abweichung` steht in Rot.",
    notes: [
      "`Gebucht` ist die Betreuungszeit, die in moto hinterlegt ist: Ankunft bis Abholung laut Wochenplan, mit Ausnahmen für einzelne Tage.",
      "Hat das Kind einen Vertrag vom Träger, gilt der Vertrag. Dann steht unter `Gebucht` der Hinweis `laut Vertrag`. Das Ende ist die Endzeit des letzten Moduls an diesem Wochentag. Die Ankunft kommt weiter aus dem Wochenplan.",
      "`Tatsächlich` reicht von der ersten Anmeldung bis zur letzten Abmeldung des Tages.",
      "Zeit im Unterricht wird nicht abgezogen. Unter `Hinweis` steht sie als `Zum Unterricht` mit der Uhrzeit.",
      "Kommt ein Kind nach dem Unterricht nicht zurück, endet `Tatsächlich` mit dieser Abmeldung. Die `Abweichung` ist dann groß und negativ. `Zum Unterricht: ab` mit der Uhrzeit zeigt, dass es nicht nach Hause gegangen ist.",
      "Eine Überschreitung ist mehr als 15 Minuten früher oder später als gebucht, oder ein Tag ohne Buchung.",
      "Krank, entschuldigt, abgemeldet, Schließtage und Feiertage zählen nicht als zu kurz.",
      "War das Kind an einem abgemeldeten Tag doch da, steht dort `Da ohne Buchung`.",
      "Der heutige Tag zählt erst, wenn das Kind angemeldet ist.",
    ],
    differences: [
      "Steht `Keine Zeiten hinterlegt`? Für das Kind ist kein Wochenplan eingetragen. Dann gibt es nichts zu vergleichen.",
      "Steht `Noch nicht abgemeldet`? Das Kind ist noch da, oder die Abmeldung fehlt.",
    ],
    troubleshootingDetails: [
      "Fehlt `Soll und Ist` beim Kind? Ihr Konto darf die Kinderdaten nicht lesen.",
    ],
    related: [
      HELP_TOPICS.leadCareTimes,
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.leadExports,
    ],
  };
}
