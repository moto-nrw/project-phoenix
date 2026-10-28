import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Hinweis `Testsystem` im Träger-Portal (#4658): erfundene Beispielkinder. */
export function traegerTestSystemTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerTestSystem,
    title: "Mit dem Testsystem ausprobieren",
    question: "Was ist das Testsystem?",
    summary:
      "Im Testsystem probieren Sie moto mit erfundenen Kindern aus. Echte Daten gehören nicht hinein.",
    group: "einstieg",
    audience: "traeger",
    icon: "PuzzlePiece",
    requirements: [
      "Das moto-Team hat für Ihren Träger ein Testsystem angelegt.",
    ],
    steps: [
      "Melden Sie sich im Träger-Portal an.",
      "Gehören Sie auch zum echten Träger? Wählen Sie den Test-Träger aus.",
      "Oben steht der Hinweis `Testsystem`. So erkennen Sie das Testsystem.",
      "Die Seite `Schulen` zeigt die Test-Schulen. Probieren Sie alles aus.",
    ],
    result:
      "Wenn Sie fertig sind, löscht das moto-Team das Testsystem mit allen Test-Schulen.",
    notes: [
      "Alle Kinder im Testsystem sind erfunden. Tragen Sie keine echten Kinder ein.",
    ],
    troubleshootingDetails: [
      "Sie sehen den Hinweis `Testsystem` nicht? Dann sind Sie beim echten Träger. Melden Sie sich ab und wählen Sie den Test-Träger.",
    ],
    related: [HELP_TOPICS.traegerLogin],
  };
}
