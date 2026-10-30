import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

// Hinweise zu Pausen und Höchstarbeitszeit (#4804). Nur Hinweise, sie sperren
// nichts. Werte und Grenzen bewertet der Server.
export function workTimeHintsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.workTimeHints,
    title: "Hinweise zu Pausen und Höchstarbeitszeit",
    question: "Was bedeuten die Hinweise zu Pausen und Arbeitszeit?",
    summary:
      "moto zeigt, wenn eine Pause zu kurz war oder zu lange gearbeitet wurde.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "Hourglass",
    steps: [],
    instructionGroups: [
      {
        title: "Hinweise finden",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
          "Lesen Sie in der Tabelle `Zeiterfassung` die Spalte `Hinweis`.",
          "Lesen Sie unter jeder Woche die Zeile `Summe KW`.",
        ],
      },
      {
        title: "Diese Hinweise gibt es",
        ordered: false,
        steps: [
          "`Pause zu kurz`: Nach 6 Stunden Arbeit sind 30 Minuten Pause nötig, nach 9 Stunden 45 Minuten. Der Hinweis nennt, wie viel nötig war und wie viel Sie gemacht haben.",
          "`Mehr als 10 Std. gearbeitet`: An einem Tag sind höchstens 10 Stunden Arbeit erlaubt.",
          "`Mehr als 48 Std. in der Woche`: Das steht in der Zeile `Summe KW` der Woche.",
        ],
      },
    ],
    result:
      "Sie sehen, an welchen Tagen und in welchen Wochen die Arbeitszeit nicht zum Arbeitszeitgesetz passt.",
    notes: [
      "Die Hinweise sperren nichts. Sie können weiter ein- und ausstempeln.",
      "Es zählen nur Pausen ab 15 Minuten. Zwei Pausen von 10 Minuten zählen nicht.",
      "Eine Lücke von mindestens 15 Minuten zwischen zwei Arbeitsblöcken zählt als Pause.",
      "Während Sie arbeiten, zeigt die `Stempeluhr` den Hinweis, sobald er gilt. Während einer laufenden Pause erscheint kein Pausen-Hinweis.",
      "Ein Arbeitsblock über Mitternacht zählt zu dem Tag, an dem er begann.",
    ],
    differences: [
      "Im Monat reicht die erste oder letzte Woche über den Monatsrand. `Summe KW` zählt dann die ganze Woche.",
      "Korrigieren Sie einen Tag, prüft moto ihn nach dem Speichern neu. Der Hinweis im Dialog zeigt den bisher gespeicherten Stand.",
    ],
    related: [
      HELP_TOPICS.trackWorkTime,
      HELP_TOPICS.correctWorkTime,
      HELP_TOPICS.leadWorkTimeReview,
    ],
  };
}
