import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

// Eigener Dienstplan (#3821): eigene Seite unter `Team`, nur lesend.
export function ownShiftPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.ownShiftPlan,
    title: "Eigenen Dienstplan ansehen",
    question: "Wo sehe ich meinen Dienstplan für die Woche?",
    summary:
      "Sehen Sie Ihre Schichten der Woche, die Stunden je Dienstart und Ihr Soll.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "CalendarCheck",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mein Dienstplan`.",
      "Wechseln Sie mit den Pfeilen zur gewünschten Woche. `Diese Woche` bringt Sie zurück.",
    ],
    result:
      "Sie sehen jede Schicht im Wochenraster. Darunter stehen die Stunden je Tag und je Dienstart.",
    notes: [
      "Oben stehen die geplanten Stunden, Ihr `Soll` und die `Differenz`.",
      "In der `Zeiterfassung` führt oben der Knopf `Mein Dienstplan` ebenfalls hierher.",
      "Das Soll ist dasselbe wie in der Zeiterfassung. Feiertage und Schließtage sind schon eingerechnet.",
    ],
    differences: [
      "Auf dem Handy finden Sie `Mein Dienstplan` unten unter `Mehr`. Dort stehen die Schichten als Liste je Tag statt im Wochenraster.",
      "Sie können den Plan hier nur ansehen. Änderungen trägt die Leitung im Dienstplan ein.",
      "Auf dem Handy erscheinen Samstag und Sonntag nur mit einer Schicht. Bei einem Sonntagsdienst kann im Wochenraster auch der Samstag erscheinen.",
    ],
    troubleshootingDetails: [
      "`Mein Dienstplan` fehlt? Dann plant Ihre OGS die Schichten nicht in moto.",
    ],
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.mySchedule],
  };
}
