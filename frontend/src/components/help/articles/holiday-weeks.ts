import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/ferienwochen` (#4665): die bestätigten Kinder der Ferienwochen, die die
 * OGS für ihren Träger ausrichtet, nur bis zum letzten Tag der Woche. Dazu
 * die freigegebenen Antworten und die zweite sorgeberechtigte Person (#5202)
 * und die Gesundheitsangaben, wenn die Eltern zugestimmt haben (#4722).
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
    image: "/help/screens/ferienwochen-freigegebene-antworten.webp",
    imageAlt:
      "Die Liste der Kinder mit der zweiten sorgeberechtigten Person, den freigegebenen Antworten und der Spalte Gesundheit.",
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
      "Unter `Gesundheit` stehen Allergien, Medikamente und weitere Hinweise. Dort stehen auch Antworten auf Gesundheitsfragen, die der Träger freigibt.",
    ],
    result:
      "Sie wissen, wie viele Kinder in welcher Zeit kommen und woher sie kommen.",
    notes: [
      "Über das Menü mit den drei Punkten laden Sie die Liste `Als Excel herunterladen` oder `Als CSV herunterladen`.",
      "Ein `Feriengast` geht auf keine Schule Ihres Trägers. In Klammern steht die Schule, die die Eltern genannt haben.",
      "Steht in einer Spalte `–`, haben die Eltern die Frage nicht beantwortet.",
      "Gesundheitsangaben sehen Sie nur, wenn die Eltern zugestimmt haben. Sie stehen nur in der Liste, nicht in der Excel- oder CSV-Datei.",
      "Kommt ein Kind Ihrer OGS in die Ferienwoche? Dann stehen seine Gesundheitsangaben auch in seiner Kinderakte. Sie stehen dort unter `Angaben aus der Ferienanmeldung`. Die `Gesundheitsinformationen` des Kindes ändern sich nicht.",
      "E-Mail-Adressen sehen Sie nicht. Sie bleiben beim Träger.",
      "Jeder Abruf und jede Datei wird im Zugriffsprotokoll Ihrer OGS vermerkt.",
    ],
    differences: [
      "Nach dem letzten Tag der Woche ist die Woche nicht mehr zu sehen. Die Gesundheitsangaben auch nicht.",
      "Storniert der Träger eine Anmeldung, verschwindet das Kind sofort.",
    ],
    troubleshootingDetails: [
      "Fehlt ein Kind? Anmeldungen prüft und bestätigt der Träger. Fragen Sie dort nach.",
      "Steht bei einem Kind unter `Gesundheit` nichts? Dann haben die Eltern nichts angegeben oder nicht zugestimmt. Ältere Anmeldungen geben keine Gesundheitsangaben weiter.",
      "Fehlt `Ferienwochen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadHolidayGuestGroup,
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.leadExports,
    ],
  };
}
