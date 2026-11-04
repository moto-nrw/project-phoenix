import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Hausaufgaben (#4796): die Lehrkraft trägt sie für ihre Klasse und einen
 * Tag ein und sieht je Kind die Rückmeldung der OGS aus der Lernzeit.
 */

export function teacherHomeworkTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherHomework,
    title: "Hausaufgaben eintragen",
    question: "Wie trage ich Hausaufgaben für meine Klasse ein?",
    summary:
      "Sie tragen ein, was die Klasse an einem Tag tun soll. Die OGS meldet je Kind zurück.",
    group: "klasse",
    audience: "teacher",
    icon: "NotePencil",
    steps: [],
    instructionGroups: [
      {
        title: "Hausaufgaben eintragen",
        steps: [
          "Öffnen Sie in der `Klassenansicht` Ihre Klasse und den Tag.",
          "Tippen Sie unter `Hausaufgaben` auf `Hausaufgaben eintragen`.",
          "Schreiben Sie bei Bedarf das `Fach (freiwillig)`.",
          "Schreiben Sie unter `Was ist zu tun?`, was die Kinder machen sollen.",
          "Tippen Sie auf `Speichern`.",
        ],
      },
      {
        title: "Einen Eintrag ändern oder löschen",
        steps: [
          "Tippen Sie beim Eintrag auf die drei Punkte.",
          "Wählen Sie `Ändern` oder `Löschen`.",
        ],
      },
      {
        title: "Die Rückmeldung der OGS lesen",
        steps: [
          "Öffnen Sie in der `Klassenansicht` den Tag der Hausaufgaben.",
          "Beim Kind steht `Hausaufgaben: Fertig`, `Nicht fertig` oder `War schwer`.",
          "Eine Notiz der OGS steht unter dem Namen, mit `OGS:` davor.",
        ],
      },
    ],
    result:
      "Die OGS sieht die Hausaufgaben in der Lernzeit bei jedem Kind der Klasse.",
    notes: [
      "Für einen Tag können Sie mehrere Einträge machen, zum Beispiel einen je Fach.",
      "Jede Lehrkraft der Klasse kann alle Einträge ändern oder löschen. Auch die Einträge der anderen.",
      "Führen Sie selbst eine Lernzeit unter `Meine Aufsichten`? Dann geben Sie die Rückmeldung dort mit `Hausaufgaben`.",
      "Eltern sehen weder die Hausaufgaben noch die Rückmeldung.",
    ],
    differences: [
      "Fehlen `Hausaufgaben eintragen` und die drei Punkte? Dann darf Ihre Rolle keine Hausaufgaben eintragen. Fragen Sie die OGS.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherSupervision,
    ],
  };
}
