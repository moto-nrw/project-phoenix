import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/ferienwochen` (#4666): Die Leitung wählt die Gruppe für die Feriengäste
 * einer Woche. Die bestätigten Gäste kommen dann in die Gruppe und in den
 * Check-in.
 */
export function holidayGuestGroupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadHolidayGuestGroup,
    title: "Die Feriengruppe wählen",
    question: "Wie kommen Feriengäste in eine Gruppe?",
    summary:
      "Wählen Sie für jede Ferienwoche eine Gruppe. Dort sieht Ihr Team die Feriengäste.",
    group: "auswertung",
    audience: "lead",
    icon: "UsersThree",
    requirements: [
      "Der Träger hat Ihre OGS für eine Ferienwoche eingetragen.",
      "Es gibt eine Gruppe für die Ferienwoche, zum Beispiel `Feriengruppe`. Eine neue Gruppe legen Sie unter `Gruppen` an.",
    ],
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Öffnen Sie `Ferienwochen`.",
      "Richtet Ihre OGS mehrere Wochen aus, wählen Sie oben die Woche.",
      "Wählen Sie unter `Feriengruppe` die Gruppe.",
      "Wählen Sie `Feriengäste übernehmen`.",
    ],
    result:
      "Ab dem ersten Tag der Woche stehen die Feriengäste in der Gruppe und im Check-in. Ihr Team sieht bei jedem Feriengast Betreuung, Notfall und Abholung.",
    notes: [
      "Feriengäste sind Kinder anderer Schulen Ihres Trägers und Kinder ohne Schule beim Träger. Kinder Ihrer OGS bleiben in ihrer Gruppe.",
      "Bestätigt der Träger später weitere Kinder, kommen sie innerhalb einer Stunde in die Gruppe. Storniert er, gehen sie innerhalb einer Stunde.",
      "Jede Übernahme steht im Zugriffsprotokoll Ihrer OGS.",
    ],
    differences: [
      "Fehlen Auswahl und Knopf? Gruppen wählen darf nur, wer Kinder anlegen darf.",
    ],
    troubleshootingDetails: [
      "Nach dem letzten Tag der Woche stehen die Feriengäste nicht mehr in der Gruppe und nicht mehr im Check-in.",
    ],
    related: [HELP_TOPICS.leadHolidayWeeks, HELP_TOPICS.holidayGuests],
  };
}
