import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/parents/calendar`: Termine der naechsten drei Monate, mit Zu- und Absage. */
export function parentCalendarTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentCalendar,
    title: "Den Kalender ansehen",
    question: "Wo sehe ich Termine und Einladungen?",
    summary: "Der Kalender zeigt die Termine Ihrer Kinder.",
    group: "nachrichten",
    audience: "parent",
    icon: "CalendarDays",
    steps: [
      "Tippen Sie unten auf `Kalender`.",
      "Lesen Sie die Termine unter `Diese Woche`, `Nächste Woche` und `Später`.",
      "Tippen Sie einen Termin an, um Einzelheiten zu sehen.",
    ],
    result: "Sie sehen Tag, Uhrzeit und für welches Kind der Termin gilt.",
    notes: [
      "Ein Termin ist eine `Betreuungszeit`, eine `Einladung` oder ein `Termin`.",
      "Bei einer Einladung antworten Sie mit `Zusagen` oder `Absagen`.",
      "Danach steht am Termin `Zugesagt` oder `Abgesagt`.",
      "Rechts steht ein Monatskalender. Mit den Pfeilen wechseln Sie den Monat.",
      "Unter `Kalender abonnieren` holen Sie sich mit `Abo-Link anzeigen` die Termine in Ihren eigenen Kalender.",
    ],
    differences: [
      "Ein Termin trägt `Antwort erforderlich`? Dann wartet die OGS auf Ihre Antwort.",
      "Hat die OGS abgesagt, steht dort `Von der OGS abgesagt`.",
      "Der Kalender zeigt die nächsten 3 Monate.",
    ],
    troubleshootingDetails: [
      "Der Kalender ist leer? Dann stehen in den nächsten 3 Monaten keine Termine an.",
    ],
    related: [
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentMealPlan,
    ],
  };
}
