import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { unavailableNfcTopic, unknownNfcTopic } from "../help-article-shared";

export function tagAssignmentTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
      "Watch",
      [HELP_TOPICS.studentSearch, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
      "Watch",
      [HELP_TOPICS.studentSearch, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.tagAssignment,
    title: "Ein Armband zuweisen oder ändern",
    question: "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
    summary:
      "Verbinden Sie ein NFC-Armband mit dem richtigen Kind oder Teammitglied.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Watch",
    requirements: [
      "Die Person ist in moto angelegt und steht dort nur einmal.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Ein freies Armband zuweisen",
        steps: [
          "Melden Sie sich am Tablet an.",
          "Wählen Sie im `Menü` `Armband identifizieren`.",
          "Wählen Sie `Scan starten` und halten Sie das Armband an den NFC-Sensor.",
          "Wählen Sie bei einem freien Armband `Person auswählen`.",
          "Suchen Sie die richtige Person und wählen Sie sie aus.",
          "Wählen Sie `Armband zuweisen`.",
        ],
      },
      {
        title: "Eine Zuweisung ändern",
        steps: [
          "Scannen Sie das Armband erneut unter `Armband identifizieren`.",
          "Wählen Sie `Anderer Person zuweisen`, um die Person zu wechseln.",
          "Oder wählen Sie `Armband freigeben`, damit das Armband wieder frei ist.",
          "Bestätigen Sie die Rückfrage.",
        ],
      },
    ],
    result:
      "Nach der Bestätigung ist das Armband der ausgewählten Person zugeordnet oder wieder frei.",
    differences: [
      "Auf dem Armband selbst stehen keine Namen oder anderen persönlichen Angaben.",
      "Finden Sie eine Person nicht? Prüfen Sie ihren Status und ihre OGS-Zuordnung in moto.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.nfcWorkTime],
  };
}
