import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function trackWorkTimeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.trackWorkTime,
    title: "Arbeitszeit und Pausen erfassen",
    question: "Wie erfasse ich meine Arbeitszeit und Pausen?",
    summary:
      "Stempeln Sie sich zu Arbeitsbeginn ein. Erfassen Sie Pausen und das Arbeitsende.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "Timer",
    steps: [],
    instructionGroups: [
      {
        title: "Arbeitszeit starten",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
          "Wählen Sie unter `Stempeluhr` den Arbeitsort `In der OGS` oder `Unterwegs`. `Unterwegs` heißt zum Beispiel Ausflug, Fortbildung oder zu Hause.",
          "Wählen Sie `Einstempeln`.",
        ],
      },
      {
        title: "Eine Pause erfassen",
        steps: [
          "Wählen Sie `Pause starten`.",
          "Wählen Sie die geplante Pausenlänge.",
          "Wählen Sie `Starten`.",
          "Wählen Sie `Pause beenden`, wenn Sie früher weiterarbeiten.",
        ],
      },
      {
        title: "Arbeitszeit beenden",
        steps: [
          "Beenden Sie zuerst eine laufende Pause.",
          "Wählen Sie `Ausstempeln`.",
        ],
      },
    ],
    result:
      "Ihre Arbeitszeit erscheint in der Wochenübersicht und in der Tabelle `Zeiterfassung`.",
    notes: [
      // Live geprueft: `Einstempeln` ist ein runder Knopf mit dem Wort
      // darunter. Ohne gewaehlten Arbeitsort steht dort `Bitte Status wählen`.
      "`Einstempeln` steht unter dem runden Knopf in der Mitte.",
      "Oben steht Ihr Stand: `Ist`, `Soll` und `Saldo`.",
    ],
    differences: [
      "Steht dort `Bitte Status wählen`? Dann fehlt noch der Arbeitsort. Wählen Sie zuerst `In der OGS` oder `Unterwegs`.",
      "Eine geplante Schicht steht oberhalb der Stempeluhr. Sie startet die Zeiterfassung nicht automatisch.",
      "Bei einer deutlichen Abweichung von Ihrer geplanten Schicht kann moto nach einem Grund fragen.",
      "Ihre Schule kann das Einstempeln erst kurz vor Ihrer Schicht erlauben. Dann zeigt moto, ab wann es geht.",
      "Nach der gewählten Pausenlänge läuft die Arbeitszeit automatisch weiter.",
    ],
    related: [HELP_TOPICS.correctWorkTime, HELP_TOPICS.nfcWorkTime],
  };
}
