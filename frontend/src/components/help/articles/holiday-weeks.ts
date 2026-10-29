import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/ferienwochen` (#4665): die bestätigten Kinder der Ferienwochen, die die
 * OGS für ihren Träger ausrichtet, nur bis zum letzten Tag der Woche. Dazu
 * die freigegebenen Antworten und die zweite sorgeberechtigte Person (#5202).
 */
export function holidayWeeksTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadHolidayWeeks,
    title: "Die Kinder einer Ferienwoche ansehen",
    question: "Welche Kinder kommen in unsere Ferienwoche?",
    summary:
      "Richtet Ihre OGS eine Ferienwoche für den Träger aus, sehen Sie die bestätigten Kinder.",
    group: "auswertung",
    audience: "lead",
    icon: "SunHorizon",
    requirements: [
      "Der Träger hat Ihre OGS für eine Ferienwoche eingetragen.",
      "Der Träger hat Anmeldungen für diese Woche bestätigt.",
    ],
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Öffnen Sie `Ferienwochen`.",
      "Richtet Ihre OGS mehrere Wochen aus, wählen Sie oben die Woche.",
      "Lesen Sie unter `Je Zeitmodell` und `Je Heimatschule` die Zahlen.",
      "Unter `Kinder` steht jedes Kind mit Geburtsdatum, Heimatschule und Zeitmodell.",
      "Gibt der Träger Fragen frei, steht jede in einer eigenen Spalte. Zum Beispiel `Darf allein nach Hause gehen`.",
      "Unter `Zweite sorgeberechtigte Person` stehen Name und Telefon, wenn die Eltern eine zweite Person angegeben haben.",
    ],
    result:
      "Sie wissen, wie viele Kinder in welcher Zeit kommen und woher sie kommen.",
    notes: [
      "Über das Menü mit den drei Punkten laden Sie die Liste `Als Excel herunterladen` oder `Als CSV herunterladen`.",
      "Ein `Feriengast` geht auf keine Schule Ihres Trägers. In Klammern steht die Schule, die die Eltern genannt haben.",
      "Steht in einer Spalte `–`, haben die Eltern die Frage nicht beantwortet.",
      "Gesundheitsangaben und E-Mail-Adressen sehen Sie nicht. Sie bleiben beim Träger.",
      "Jeder Abruf und jede Datei wird im Zugriffsprotokoll Ihrer OGS vermerkt.",
    ],
    differences: [
      "Nach dem letzten Tag der Woche ist die Woche nicht mehr zu sehen.",
      "Storniert der Träger eine Anmeldung, verschwindet das Kind sofort.",
    ],
    troubleshootingDetails: [
      "Fehlt ein Kind? Anmeldungen prüft und bestätigt der Träger. Fragen Sie dort nach.",
      "Fehlt `Ferienwochen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadHolidayGuestGroup,
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.leadExports,
    ],
  };
}
