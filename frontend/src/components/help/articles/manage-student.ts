import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Die Kindakte unter `/students/[id]`. Welche Reiter erscheinen, haengt am
 * Zugriff auf das Kind (FULL_ACCESS_BASE_TABS gegen LIMITED_ACCESS_BASE_TABS,
 * students/[id]/page.tsx:139) und daran, ob die Anmeldung ueber moto laeuft.
 * An keine der drei Arbeitsweisen gebunden.
 */
export function manageStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadManageStudent,
    title: "Angaben eines Kindes verwalten",
    question: "Wo pflege ich alle Angaben zu einem Kind?",
    summary: "Die Kindakte hat für jeden Bereich einen eigenen Reiter.",
    group: "kinderdaten",
    audience: "lead",
    icon: "UserRoundSearch",
    steps: [],
    instructionGroups: [
      {
        title: "Die Kindakte öffnen",
        steps: [
          "Öffnen Sie `Alle Kinder` und suchen Sie das Kind.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie oben den passenden Reiter.",
        ],
      },
      {
        title: "Was in welchem Reiter steht",
        steps: [
          "`Stammdaten`: Name, Klasse, Gruppe, Adresse und die dauerhaften Hinweise zum Kind.",
          "`Notizen`: Einträge zu diesem Kind, mit Datum und Verfasser.",
          "`Erziehungsberechtigte`: wer das Kind abholen darf und wer erreichbar ist.",
          "`Betreuungszeiten`: der Wochenplan und einzelne Ausnahmen.",
          "`Betreuungsplan`: die geplante Betreuung Woche für Woche.",
          "`Nachrichten`: die Unterhaltungen mit den Bezugspersonen.",
          "`Dokumente`: hochgeladene Dateien zu diesem Kind.",
          "`Änderungsprotokoll`: wer wann was geändert hat.",
          "`Historie`: die vergangenen Betreuungstage.",
        ],
        ordered: false,
      },
      {
        title: "Stammdaten ändern",
        steps: [
          "Wählen Sie den Reiter `Stammdaten`.",
          "Wählen Sie `Bearbeiten`.",
          "Ändern Sie die benötigten Angaben.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result: "Geänderte Angaben gelten sofort für alle im Team.",
    notes: ["Wer eine Angabe geändert hat, steht im `Änderungsprotokoll`."],
    differences: [
      "Sie sehen weniger Reiter? Dann haben Sie nur eingeschränkten Zugriff auf dieses Kind.",
      "Der Reiter `Anmeldungen` erscheint nur, wenn die Anmeldung über moto läuft.",
    ],
    troubleshootingDetails: [
      "Fehlt `Bearbeiten`? Dann fehlen Ihnen die Schreibrechte für dieses Kind.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.studentNotes,
      HELP_TOPICS.leadCareTimes,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}
