import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/schliesstage` (#4702): Schließtage des Trägers und Feiertage je Bundesland. */
export function traegerClosingDaysTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerClosingDays,
    title: "Schließtage für Schulen planen",
    question: "Wie plane ich einen Brückentag für mehrere Schulen?",
    summary:
      "Ein Schließtag gilt für die Schulen, die Sie wählen. An diesem Tag ist die OGS zu. Die Feiertage kommen aus dem Bundesland der Schule.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarSlash",
    requirements: [
      "Sehen kann die Seite jede Rolle im Träger-Büro.",
      "Schließtage anlegen darf Ihre Rolle, wenn sie Einstellungen verwalten darf. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Schließtag anlegen",
        steps: [
          "Wählen Sie in der Navigation `Schließtage`.",
          "Wählen Sie `Schließtag anlegen`.",
          "Tragen Sie den `Grund` ein, zum Beispiel Brückentag.",
          "Tragen Sie `Von` und `Bis` ein. Für einen Tag tragen Sie zweimal dasselbe Datum ein.",
          "Wählen Sie bei `Gilt für` die Schulen. `Alle Schulen` wählt alle auf einmal.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Feiertage prüfen",
        steps: [
          "Wählen Sie in der Navigation `Schließtage`.",
          "Oben wählen Sie das Jahr.",
          "In der `Jahresübersicht` sind Feiertage blau und Schließtage orange.",
          "Unter `Feiertage nach Bundesland` stehen die Feiertage und die Schulen dazu.",
        ],
      },
    ],
    result:
      "Die gewählten Schulen haben an diesem Tag zu. Ferienwochen mit diesem Tag sind nicht mehr buchbar. Termine fallen aus, und Mitarbeitende haben kein Soll.",
    notes: [
      "Die Schulen sehen den Schließtag in ihrem Kalender mit `Vom Träger`. Ändern kann ihn nur das Träger-Büro.",
      "Der Elternbeitrag ändert sich durch einen Schließtag nicht.",
      "Das Bundesland einer Schule legt das moto-Team fest. Stimmt es nicht, schreiben Sie dem moto-Team.",
      "Bisherige Buchungen einer Ferienwoche bleiben, auch wenn Sie später einen Schließtag in die Woche legen. Neue Buchungen der Woche sind dann nicht möglich.",
    ],
    differences: [
      "Sie sehen keinen Knopf `Schließtag anlegen`? Ihre Rolle darf keine Einstellungen verwalten. Fragen Sie im Träger-Büro nach der Rolle Verwaltung.",
    ],
    related: [HELP_TOPICS.traegerHolidayPeriod, HELP_TOPICS.traegerSchoolView],
  };
}
