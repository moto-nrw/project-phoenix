import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/documents` (#4671, #4678, #4692): Schreiben und Verträge des
 * Trägers und Dokumente, die der Träger für Eltern freigegeben hat.
 */
export function parentDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDocuments,
    title: "Dokumente vom Träger finden",
    question: "Wo finde ich Schreiben vom Träger?",
    summary:
      "Unter `Dokumente` liegen die Schreiben und Verträge, die Ihnen der Träger geschickt hat. Dort finden Sie auch Dokumente, die der Träger für Eltern freigegeben hat, etwa eine Hausordnung.",
    group: "nachrichten",
    audience: "parent",
    icon: "FileText",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Dokumente`.",
      "Wählen Sie beim Schreiben `Als PDF laden`. Bei einem freigegebenen Dokument wählen Sie `Herunterladen`.",
    ],
    result:
      "Sie bekommen das Dokument auf Ihr Gerät. Sie können es speichern oder drucken.",
    notes: [
      "Das sind die Bestätigungsschreiben der Ferienbetreuung und die Betreuungsverträge. Das Bestätigungsschreiben ist auch Ihre Rechnung.",
      "Beim Betreuungsvertrag stehen die gebuchte Betreuung und der Beginn auf der Karte. Prüfen Sie im PDF jeden Wochentag und den Beitrag. Stimmt etwas nicht, melden Sie sich beim Träger.",
      "Steht im Vertrag `Vertragsschluss`? Dann gilt er seit der Annahme Ihrer Anmeldung. Sie müssen nichts unterschreiben. Heben Sie ihn gut auf.",
      "Jedes Schreiben bekommen Sie auch per Mail.",
      "Bei einem freigegebenen Dokument steht `Vom Träger für Eltern freigegeben`. Es gilt für alle Eltern, für die der Träger es freigegeben hat, zum Beispiel für eine Gruppe. Diese Dokumente kommen nicht per Mail.",
      "Sie möchten eine Nachricht auf dem Handy bekommen, wenn ein Bestätigungsschreiben kommt? Schalten Sie unter `Einstellungen` den Punkt `Neues Dokument vom Träger` ein. Einen Betreuungsvertrag bekommen Sie per Mail.",
    ],
    troubleshootingDetails: [
      "Ein Schreiben fehlt? Hier liegt es nur, wenn es an die Mail-Adresse Ihres Kontos ging. Haben Sie bei der Anmeldung eine andere Adresse angegeben, bekommen Sie es nur per Mail.",
      "Statt der Liste steht, dass Ihnen die Berechtigung fehlt? Bitte fragen Sie die Schule.",
      "Ein freigegebenes Dokument ist verschwunden? Der Träger hat die Freigabe zurückgenommen.",
    ],
    related: [
      HELP_TOPICS.parentHolidayRegistration,
      HELP_TOPICS.parentNotifications,
    ],
  };
}
