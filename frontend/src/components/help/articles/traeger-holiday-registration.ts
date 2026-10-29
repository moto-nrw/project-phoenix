import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Der Link eines veröffentlichten Ferienzeitraums (#4661). */
export function traegerHolidayRegistrationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayRegistration,
    title: "Link zur Ferienanmeldung teilen",
    question: "Wie melden Eltern ihre Kinder für die Ferien an?",
    summary:
      "Jeder veröffentlichte Ferienzeitraum hat einen Link für Eltern. Er gilt für alle Wochen und Schulen.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Megaphone",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
      "Der Ferienzeitraum ist veröffentlicht.",
    ],
    steps: [
      "Wählen Sie in der Navigation `Ferien`.",
      "Öffnen Sie den Ferienzeitraum.",
      "Unter `Link für Eltern` steht der Link. Wählen Sie daneben `Link kopieren`.",
      "Geben Sie den Link an die Eltern weiter, zum Beispiel in einem Elternbrief der Schulen.",
    ],
    result:
      "Eltern melden ihre Kinder über den Link an. Unter dem Link steht, wie viele Anmeldungen eingegangen sind.",
    notes: [
      "Unter `Hinweise und Einwilligungen` öffnen Sie die PDFs. Setzen Sie die nötigen Häkchen. Die Zusammenfassung zeigt Ihre Bestätigungen.",
      "Eltern brauchen kein Elternkonto.",
      "Eltern können eine `Zweite sorgeberechtigte Person` angeben. Die Bestätigung geht nur an die anmeldende Person.",
      "Ein Kind ohne Schule Ihres Trägers melden Eltern als Feriengast an. Es wird dadurch kein Schulkind einer OGS.",
      "Eltern bekommen eine Bestätigung per E-Mail. Sie ist noch keine Zusage.",
      "Gesundheitsangaben stehen nur in der Anmeldung. Die Bestätigung nennt sie nicht.",
      "Drei Monate nach der letzten Woche löscht moto die Anmeldungen. Ausgestellte Rechnungen und Bestätigungsschreiben bleiben.",
    ],
    differences: [
      "Sie sehen keinen Link? Dann ist der Ferienzeitraum noch ein Entwurf. Veröffentlichen Sie ihn zuerst.",
    ],
    troubleshootingDetails: [
      "Eine Woche oder ein Zeitmodell lässt sich nicht entfernen? Dafür gibt es schon Anmeldungen.",
      "Eltern sagen, die Anmeldung sei nicht offen? Prüfen Sie `Anmeldebeginn` und `Anmeldeschluss`.",
    ],
    related: [HELP_TOPICS.traegerHolidayPeriod],
  };
}
