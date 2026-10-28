import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/parents/meal-plan`: Wochenplan des Mittagessens, nur mit Freischaltung. */
export function parentDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDocuments,
    title: "Dokumente vom Träger finden",
    question: "Wo finde ich Schreiben vom Träger?",
    summary:
      "Unter `Dokumente` liegen die Schreiben, die Ihnen der Träger geschickt hat.",
    group: "nachrichten",
    audience: "parent",
    icon: "FileText",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Dokumente`.",
      "Wählen Sie beim Schreiben `Als PDF laden`.",
    ],
    result:
      "Sie bekommen das Schreiben als PDF. Sie können es speichern oder drucken.",
    notes: [
      "Heute sind das die Bestätigungsschreiben der Ferienbetreuung. Das Schreiben ist auch Ihre Rechnung.",
      "Jedes Schreiben bekommen Sie auch per Mail.",
      "Sie möchten eine Nachricht auf dem Handy bekommen? Schalten Sie unter `Einstellungen` den Punkt `Neues Dokument vom Träger` ein.",
    ],
    troubleshootingDetails: [
      "Ein Schreiben fehlt? Hier liegt es nur, wenn es an die Mail-Adresse Ihres Kontos ging. Haben Sie bei der Anmeldung eine andere Adresse angegeben, bekommen Sie es nur per Mail.",
      "Statt der Liste steht, dass Ihnen die Berechtigung fehlt? Bitte fragen Sie die Schule.",
    ],
    related: [
      HELP_TOPICS.parentHolidayRegistration,
      HELP_TOPICS.parentNotifications,
    ],
  };
}
