import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Ferienwunsch aus der OGS-Anmeldung (#4703): Vorrang oder nur mit
 * Ferienwunsch, die Prüfung durch das Büro.
 */
export function traegerHolidayWishTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayWish,
    title: "Ferienwunsch aus der OGS-Anmeldung",
    question: "Wie bekommen Kinder mit Ferienwunsch Vorrang?",
    summary:
      "Eltern geben in der OGS-Anmeldung an, ob ihr Kind in den Ferien Betreuung braucht. Sie schalten je Ferienzeitraum ein, dass dieser Ferienwunsch zählt.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Sun",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
      "Die OGS fragt den Ferienwunsch in ihrer Formularvorlage ab. Das schaltet die OGS bei `Ferienwunsch (Kind)` mit `Ferienwunsch abfragen` ein.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Festlegen, was ohne Ferienwunsch gilt",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Allgemein` und dann den Reiter `Ferienbetreuung`.",
          "Wählen Sie bei `Was gilt ohne Ferienwunsch?`: `Kinder mit Ferienwunsch haben Vorrang` oder `Nur Kinder mit Ferienwunsch können sich anmelden`.",
        ],
      },
      {
        title: "Für einen Ferienzeitraum einschalten",
        steps: [
          "Wählen Sie in der Navigation `Ferienbetreuung`.",
          "Öffnen Sie den Ferienzeitraum und wählen Sie den Reiter `Planung`.",
          "Schalten Sie `Ferienwunsch aus der OGS-Anmeldung berücksichtigen` ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Ferienwunsch prüfen",
        description:
          "Ist der Ferienwunsch nicht bekannt, steht beim Kind `Ferienwunsch prüfen`.",
        steps: [
          "Wählen Sie im Reiter `Anmeldungen` beim Filter `Ferienwunsch` den Eintrag `Ferienwunsch prüfen`.",
          "Öffnen Sie eine Anmeldung.",
          "Fragen Sie bei der OGS nach, was in der OGS-Anmeldung steht.",
          "Wählen Sie beim Kind unter `Ferienwunsch` den Knopf `Mit Ferienwunsch` oder `Ohne Ferienwunsch`.",
        ],
      },
    ],
    result:
      "Ist eine Woche voll, warten Kinder ohne Ferienwunsch auf der Warteliste hinter den anderen. Im Reiter `Plätze` stehen sie mit `Ohne Ferienwunsch`.",
    notes: [
      "Freie Plätze gehen weiter in der Reihenfolge der Anmeldung. Ein Platz, den ein Kind schon hat, bleibt.",
      "Eltern mit Elternkonto sehen den Ferienwunsch ihres Kindes vor dem Absenden. moto prüft ihn dann selbst. Bei `Nur Kinder mit Ferienwunsch können sich anmelden` geht das Absenden ohne Ferienwunsch nicht.",
      "Es zählt die zugesagte OGS-Anmeldung des Schuljahres, in dem der Ferienzeitraum beginnt.",
      "Feriengäste ohne OGS-Platz sind nicht betroffen.",
      "Nur das Träger-Büro sieht den Ferienwunsch und `Ferienwunsch prüfen`. Eltern, die Bestätigung und die ausrichtende OGS sehen ihn nicht.",
    ],
    differences: [
      "Beim Kind steht nichts zum Ferienwunsch? Der Ferienzeitraum berücksichtigt ihn nicht, oder das Kind ist ein Feriengast.",
    ],
    troubleshootingDetails: [
      "Auch mit Elternkonto kann der Ferienwunsch fehlen. Dann steht beim Kind `Ferienwunsch prüfen`. Bitte fragen Sie bei der OGS nach.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayPlaces,
      HELP_TOPICS.traegerHolidayRegistrationReview,
      HELP_TOPICS.traegerHolidayPeriod,
    ],
  };
}
