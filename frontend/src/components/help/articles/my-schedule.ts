import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function myScheduleTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.mySchedule,
    title: "Meine Termine und Einsätze ansehen",
    question: "Wo sehe ich meine Termine und Einsätze?",
    summary:
      "Prüfen Sie persönliche Termine, geplante Schichten und Ihre Einsätze im Betreuungsplan.",
    group: "tagesplanung",
    audience: ["caregiver", "lead"],
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Termine und Einsätze ansehen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mein Kalender`.",
          "Bleiben Sie im Tab `Meine Termine`.",
          "Wählen Sie oben `Tag`, `Woche` oder `Monat`.",
          "Nutzen Sie die Pfeile oder `Heute`, um zum gewünschten Zeitraum zu wechseln.",
          "Wählen Sie einen Eintrag, um Uhrzeit, Ort und weitere Angaben zu sehen.",
        ],
      },
      {
        title: "Die Farben verstehen",
        ordered: false,
        steps: [
          "Grün zeigt Termine und Einladungen.",
          "Blau zeigt Ihre Einsätze im Betreuungsplan.",
          "Orange zeigt Ihre Schichten aus dem Dienstplan.",
          "Lila zeigt Schultermine. Eine Lehrkraft hat sie in moto schule eingetragen, zum Beispiel einen Ausflug. Die Art steht im geöffneten Termin.",
          "Türkis zeigt gesetzliche Feiertage. Sie gelten für die ganze Schule und lassen sich nicht ändern.",
        ],
      },
      {
        title: "Den Kalender abonnieren",
        description:
          "Nutzen Sie das Abo, wenn Sie hauptsächlich Ihren persönlichen Kalender verwenden. Neue und geänderte Einträge aus moto erscheinen dort automatisch.",
        steps: [
          "Gehen Sie unter dem Kalender zum Bereich `Kalender abonnieren`.",
          "Wählen Sie `Abo-Link anzeigen`.",
          "Wählen Sie `Im Kalender abonnieren`. Oder kopieren Sie den Link in Ihren persönlichen Kalender.",
          "Der Link ist persönlich und wird nur einmal angezeigt. Geben Sie ihn nicht weiter.",
        ],
      },
    ],
    result:
      "Sie haben einen Überblick über Ihre Termine und Einsätze. Das Kalender-Abo übernimmt neue, geänderte und abgesagte Einträge automatisch.",
    notes: [
      "Mit `Sa/So` blenden Sie das Wochenende ein. Eine Zahl zeigt, dass dort Einträge liegen.",
      "Bei einer Einladung wählen Sie im geöffneten Termin `Zusagen` oder `Absagen`.",
      "Legen Sie einen Termin an und wählen Sie `Alle Lehrkräfte`. Dann sehen die Lehrkräfte ihn in moto schule.",
    ],
    differences: [
      "Das Kalender-Abo ist nur zum Lesen. Änderungen in Ihrem persönlichen Kalender werden nicht an moto übertragen.",
    ],
    related: [HELP_TOPICS.carePlan, HELP_TOPICS.trackWorkTime],
  };
}
