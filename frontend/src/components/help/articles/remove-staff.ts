import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/database/personal`, Knopf `Löschen`. Der Knopf heisst `Endgültig
 * löschen`, der Vorgang ist aber eine Abschaltung: der Text des Dialogs sagt
 * ausdruecklich, dass Anwesenheiten und Zeiterfassung bleiben und die Person
 * erneut eingeladen werden kann. Genau das muss der Artikel sagen. Nach der
 * Zeiterfassungsfrist löscht der nächtliche Lauf die Person (#3963).
 */
export function removeStaffTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadRemoveStaff,
    title: "Eine Person aus dem Team entfernen",
    question: "Wie entferne ich jemanden aus unserem Team?",
    summary:
      "Der Zugang geht aus. Die bisherigen Einträge bleiben vorerst erhalten.",
    group: "personal",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
      "Wählen Sie die Person aus der Liste.",
      "Wählen Sie oben rechts `Löschen`.",
      "Lesen Sie, was mit den Angaben der Person geschieht.",
      "Tippen Sie Vor- und Nachnamen der Person ein.",
      "Wählen Sie `Endgültig löschen`.",
    ],
    result:
      "Der Zugang ist aus. Unter `Mitarbeiter` finden Sie die Person nur noch mit der Auswahl `Ehemalige`.",
    notes: [
      "Anwesenheiten und Zeiterfassung bleiben bis zum Ende der Frist für die Zeiterfassung erhalten. So lange bleiben Ihre Auswertungen vollständig.",
      "Danach löscht moto die Angaben der Person. Statt des Namens steht dann „Gelöscht Benutzer“, und unter `Ehemalige` steht die Person nicht mehr. Die Frist sehen Sie unter `Einstellungen` und `Datenschutz` bei `Zeiterfassung aufbewahren`.",
      "Sie können dieselbe Person später wieder einladen.",
    ],
    differences: [
      "Der Knopf heißt `Endgültig löschen`. Die bisherigen Einträge löscht moto aber erst nach der Frist.",
    ],
    troubleshootingDetails: [
      "`Endgültig löschen` lässt sich nicht wählen? Dann stimmt der eingetippte Name noch nicht.",
    ],
    related: [
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadStaffPermissions,
    ],
  };
}
