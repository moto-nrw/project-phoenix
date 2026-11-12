import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/dateien`, Karte `Vom Träger` (#4692): Dokumente, die der Träger für die
 * Schule oder die Eltern an ihr freigegeben hat.
 */
export function schoolTraegerDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.schoolTraegerDocuments,
    title: "Dokumente vom Träger finden",
    question: "Wo finde ich Dokumente, die der Träger freigegeben hat?",
    summary:
      "Ihr Träger kann Dokumente für Ihre Schule freigeben. Sie finden sie unter `Dateien`.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Download",
    requirements: ["Sie dürfen die Dateien Ihrer Schule verwalten."],
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Dateien`.",
      "Oben steht die Karte `Vom Träger`.",
      "Unter jedem Titel steht, für wen der Träger das Dokument freigegeben hat.",
      "Wählen Sie `Herunterladen`.",
    ],
    result:
      "Sie haben das Dokument auf Ihrem Gerät. Ändern kann es nur der Träger.",
    notes: [
      "Hier stehen auch Dokumente, die der Träger für die Eltern Ihrer Schule oder einer Gruppe freigegeben hat. So wissen Sie, was die Eltern bekommen haben.",
      "Soll das Team ein Dokument sehen? Laden Sie es in einen Ordner unter `Dateien` hoch.",
    ],
    differences: [
      "Die Karte `Vom Träger` fehlt? Dann hat der Träger zurzeit nichts für Ihre Schule freigegeben. Oder Ihnen fehlt das Recht, die Dateien zu verwalten.",
    ],
    troubleshootingDetails: [
      "Ein Dokument ist verschwunden? Der Träger hat die Freigabe zurückgenommen oder das Dokument gelöscht. Fragen Sie beim Träger nach.",
    ],
    related: [HELP_TOPICS.sharedFiles],
  };
}
