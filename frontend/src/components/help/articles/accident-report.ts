import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Unfallanzeige (#4771): Unfall am Kind erfassen, unterschreiben, als PDF
 * weitergeben. moto schickt nichts an die Unfallkasse.
 */
export function accidentReportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadAccidentReport,
    title: "Einen Unfall erfassen",
    question: "Wie erfasse ich einen Unfall und erstelle die Unfallanzeige?",
    summary:
      "Sie tragen den Unfall am Kind ein. moto erstellt daraus die Unfallanzeige als PDF.",
    group: "kinderdaten",
    audience: "lead",
    icon: "FirstAid",
    requirements: [
      "Das Recht `Unfälle von Kindern erfassen, Unfallanzeigen ändern, unterschreiben und korrigieren`. Die Leitung hat es immer.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Den Unfall erfassen",
        steps: [
          "Öffnen Sie das Kind und wählen Sie den Reiter `Dokumente`.",
          "Wählen Sie `Unfall erfassen`.",
          "Tragen Sie ein, wann und wo es passiert ist und was passiert ist.",
          "Tragen Sie die Verletzung, die Erste Hilfe und die Aufsichtsperson ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Die Unfallanzeige unterschreiben",
        steps: [
          "Die Unfallanzeige öffnet sich nach dem Speichern.",
          "Unterschreiben Sie im Feld `Unterschrift` und wählen Sie `Unterschrift bestätigen`.",
          "Wählen Sie `Unterschreiben`.",
        ],
      },
      {
        title: "Auf Papier unterschreiben",
        steps: [
          "Wählen Sie `PDF herunterladen` und drucken Sie das PDF aus.",
          "Lassen Sie es unterschreiben und scannen oder fotografieren Sie es.",
          "Wählen Sie `Unterschriebenes Papierexemplar hochladen`.",
        ],
      },
    ],
    result:
      "Die Unfallanzeige liegt unterschrieben beim Kind unter `Dokumente`. Das PDF geben Sie selbst an die Unfallkasse weiter.",
    notes: [
      "Felder, die moto nicht kennt, etwa die Unternehmensnummer, ergänzen Sie auf dem Ausdruck.",
      "Jedes Erfassen, Ändern und Unterschreiben steht im Änderungsprotokoll des Kindes. Jedes Herunterladen hält moto im Zugriffsprotokoll fest.",
    ],
    differences: [
      "Ist die Unfallanzeige schon unterschrieben, fehlt `Ändern`. Wählen Sie `Korrektur anlegen`. Die alte Fassung bleibt erhalten, die Korrektur wird neu unterschrieben.",
      "Fehlt `Unfall erfassen`? Dann fehlt Ihnen das Recht dafür. Fragen Sie die Leitung.",
    ],
    related: [HELP_TOPICS.leadManageStudent],
  };
}
