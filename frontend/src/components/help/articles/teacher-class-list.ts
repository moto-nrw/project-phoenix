import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Klassenseite (`class-day-class.tsx`). Fuenf benannte Abschnitte plus die
 * Statuskennzeichen aus `status-labels.ts`.
 */
export function teacherClassListTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassList,
    title: "Die Liste einer Klasse lesen",
    question: "Was bedeuten die Abschnitte in der Klassenliste?",
    summary: "Die Liste trennt, wer bleibt, wer nach Hause geht und wer fehlt.",
    group: "klasse",
    audience: "teacher",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Die Liste öffnen",
        steps: [
          "Wählen Sie in der `Klassenansicht` eine Klasse.",
          "Lesen Sie die Abschnitte von oben nach unten.",
        ],
      },
      {
        title: "Was die Abschnitte bedeuten",
        steps: [
          "`Bleiben in der Betreuung`: diese Kinder übergeben Sie an die OGS.",
          "`Gehen nach Hause`: diese Kinder gehen nach dem Unterricht.",
          "`Abgemeldet`: für heute wurde die Betreuung abgesagt.",
          "`Keine Betreuung`: diese Kinder sind gar nicht in der OGS.",
          "`Klassenverband`: alle Kinder der Klasse zusammen.",
        ],
        ordered: false,
      },
      {
        title: "Was die Kennzeichen bedeuten",
        steps: [
          "`Krank`, `Entschuldigt` oder `Klassenfahrt`: das Kind fehlt heute.",
          "`Heute abgemeldet`: die Betreuung für heute wurde abgesagt.",
          "`Andere Abholzeit`: das Kind geht heute zu einer anderen Zeit.",
        ],
        ordered: false,
      },
    ],
    result:
      "Sie sehen ohne Rückfrage, welches Kind nach dem Unterricht wohin geht.",
    notes: [
      "Bei einer anderen Abholzeit steht darunter die übliche Zeit mit `sonst`.",
      "Kam die Meldung heute herein, steht die Uhrzeit dabei.",
      "Eine neue Krankmeldung erscheint von selbst. Sie müssen die Seite nicht neu laden.",
    ],
    differences: [
      "Abschnitte und Kennzeichen ändert das OGS-Team, nicht moto schule.",
    ],
    troubleshootingDetails: [
      "Ein Kind fehlt in der Liste? Dann ist es nicht in dieser Klasse eingetragen. Schreiben Sie der OGS.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherMorningAttendance,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
