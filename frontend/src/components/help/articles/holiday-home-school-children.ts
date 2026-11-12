import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/ferienwochen`, Reiter `Kinder bei anderen OGS` (#6679): Die Heimatschule
 * sieht die bestätigten Ferienbuchungen ihrer Kinder in Wochen, die eine
 * andere OGS des Trägers ausrichtet, bis zum letzten Tag der Woche. Ohne
 * Gesundheitsangaben und Kontakte.
 */
export function holidayHomeSchoolChildrenTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadHolidayHomeSchoolChildren,
    title: "Kinder Ihrer OGS in Ferienwochen anderer OGS",
    question: "Welche unserer Kinder betreut in den Ferien eine andere OGS?",
    summary:
      "Sie sehen, welche Kinder Ihrer OGS in den Ferien eine andere OGS des Trägers betreut.",
    group: "auswertung",
    audience: "lead",
    icon: "MapPin",
    requirements: [
      "Der Träger hat die Ferienanmeldung des Kindes für die Woche einer anderen OGS bestätigt.",
      "In der Anmeldung steht Ihre OGS als Schule des Kindes.",
    ],
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Öffnen Sie `Ferienwochen`.",
      "Richtet Ihre OGS selbst eine Ferienwoche aus, wählen Sie oben `Kinder bei anderen OGS`.",
      "Unter `Ihre Kinder in Ferienwochen anderer OGS` steht jedes Kind mit Woche, `Betreut von`, Zeitmodell und Status.",
      "Mit `Akte öffnen` öffnen Sie die Kinderakte des Kindes.",
    ],
    result:
      "Sie wissen, welche Kinder in den Ferien woanders betreut werden, wann und in welcher Zeit. So können Sie planen und die Familien informieren.",
    notes: [
      "Unter Status steht, ab wann das Kind woanders betreut wird, zum Beispiel `Ab 04.01.`. Ab diesem Tag steht dort `Läuft gerade`. Hat die Familie nur einzelne Tage gebucht, zählen diese Tage. Nach dem letzten gebuchten Tag steht dort `Vorbei`, bis die Woche endet.",
      "Gesundheitsangaben und Kontakte sehen Sie hier nicht. Die hat die OGS, die das Kind betreut.",
      "Jeder Abruf mit Kindern steht im Zugriffsprotokoll Ihrer OGS.",
    ],
    differences: [
      "Richtet Ihre OGS keine Ferienwoche aus, gibt es keine Reiter. Die Seite zeigt dann nur diese Kinder.",
      "Nach dem letzten Tag der Woche steht das Kind nicht mehr hier.",
    ],
    troubleshootingDetails: [
      "Fehlt ein Kind? Offene und abgesagte Anmeldungen und Plätze auf der Warteliste stehen hier nicht. Fragen Sie beim Träger nach.",
      "Steht bei einem Kind `Nicht zugeordnet`? Dann passen Vorname, Nachname oder Geburtsdatum zu keinem Kind in moto oder zu mehreren. Prüfen Sie die Kinderakte oder bitten Sie den Träger, die Anmeldung zu ändern.",
    ],
    related: [HELP_TOPICS.leadHolidayWeeks],
  };
}
