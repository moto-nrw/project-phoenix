import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/documents` (#4671, #4678, #4679): Schreiben und Verträge des
 * Trägers, den Betreuungsvertrag unterschreiben.
 */
export function parentDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDocuments,
    title: "Dokumente vom Träger finden",
    question: "Wo finde ich Schreiben vom Träger?",
    summary:
      "Unter `Dokumente` liegen die Schreiben und Verträge, die Ihnen der Träger geschickt hat.",
    group: "nachrichten",
    audience: "parent",
    icon: "FileText",
    instructionGroups: [
      {
        title: "Ein Schreiben laden",
        steps: [
          "Tippen Sie unten auf `Mehr`.",
          "Wählen Sie `Dokumente`.",
          "Wählen Sie beim Schreiben `Als PDF laden`.",
        ],
      },
      {
        title: "Den Betreuungsvertrag unterschreiben",
        description:
          "Ein Vertrag mit `Wartet auf Ihre Unterschrift` braucht Ihre Unterschrift. Das geht auch über den Link in der Mail, ohne Konto.",
        steps: [
          "Wählen Sie beim Vertrag `Lesen und unterschreiben`.",
          "Lesen Sie die gebuchte Betreuung und den Vertrag als PDF.",
          "Lesen Sie die Widerrufsbelehrung. Setzen Sie den Haken bei `Ich habe die Widerrufsbelehrung gelesen.`",
          "Soll die Betreuung schon vor dem Ende der Widerrufsfrist beginnen? Setzen Sie auch diesen Haken.",
          "Unterschreiben Sie mit dem Finger oder tippen Sie Ihren Namen. Wählen Sie `Unterschrift bestätigen`.",
          "Wählen Sie `Vertrag verbindlich unterschreiben`.",
        ],
      },
    ],
    steps: [],
    result:
      "Sie bekommen das Schreiben als PDF. Nach der Unterschrift steht beim Vertrag `Unterschrieben`, und Sie bekommen eine Bestätigung mit dem Vertrag per Mail.",
    notes: [
      "Das sind die Bestätigungsschreiben der Ferienbetreuung und die Betreuungsverträge. Das Bestätigungsschreiben ist auch Ihre Rechnung.",
      "Beim Betreuungsvertrag stehen die gebuchte Betreuung und der Beginn auf der Karte. Prüfen Sie im PDF jeden Wochentag und den Beitrag. Stimmt etwas nicht, melden Sie sich beim Träger.",
      "Jedes Schreiben bekommen Sie auch per Mail.",
      "Sie möchten eine Nachricht auf dem Handy bekommen, wenn ein Bestätigungsschreiben kommt? Schalten Sie unter `Einstellungen` den Punkt `Neues Dokument vom Träger` ein. Einen Betreuungsvertrag bekommen Sie per Mail.",
    ],
    troubleshootingDetails: [
      "Ein Schreiben fehlt? Hier liegt es nur, wenn es an die Mail-Adresse Ihres Kontos ging. Haben Sie bei der Anmeldung eine andere Adresse angegeben, bekommen Sie es nur per Mail.",
      "Statt der Liste steht, dass Ihnen die Berechtigung fehlt? Bitte fragen Sie die Schule.",
      "Der Link in der Mail gilt nicht mehr? Er gilt nur einmal und 30 Tage. Unterschreiben Sie in der Eltern-App unter `Dokumente`. Oder fragen Sie beim Träger nach einem neuen Link.",
      "Beim Vertrag fehlt `Lesen und unterschreiben`? Dann darf Ihr Konto für dieses Kind nicht unterschreiben. Bitte fragen Sie die Schule.",
    ],
    related: [
      HELP_TOPICS.parentHolidayRegistration,
      HELP_TOPICS.parentNotifications,
    ],
  };
}
