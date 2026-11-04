import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school` (`class-day-overview.tsx`): Tagesuebersicht ueber alle Klassen. */
export function teacherClassDayTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassDay,
    title: "Meinen Klassentag ansehen",
    question: "Wer aus meinen Klassen bleibt heute in der Betreuung?",
    summary:
      "Die `Klassenansicht` zeigt Ihre Klassen und die Zahlen des Tages.",
    group: "klasse",
    audience: "teacher",
    icon: "ChalkboardTeacher",
    steps: [
      "Melden Sie sich bei moto schule an.",
      "Sie landen in der `Klassenansicht`.",
      "Lesen Sie bei jeder Klasse die Zahlen des Tages.",
      "Wählen Sie eine Klasse, um ihre Liste zu öffnen.",
    ],
    result: "Sie sehen, wer heute bleibt und wer nach Hause geht.",
    notes: [
      "Steht bei einer Klasse `4 Kinder anders als sonst`, weicht dort heute etwas vom üblichen Plan ab.",
      "Mit den Pfeilen sehen Sie den vorherigen oder nächsten Tag.",
      "Betreut Ihre OGS am Wochenende? Dann sehen Sie auch Samstag und Sonntag. Dort gilt der Freitagsplan.",
    ],
    differences: [
      "An einem Wochenende ohne Betreuung steht `Kein Schultag`.",
      "Sie haben die Rolle `Schulverwaltung`? Dann steht oben `Ganze Schule` mit den Zahlen aller Klassen. Wählen Sie eine Zahl. Darunter stehen dann die Kinder, nach Klassen sortiert. `Liste schließen` blendet sie wieder aus.",
      "Unter `Fehlen ohne Meldung` stehen die Kinder, die eine Lehrkraft als fehlend eingetragen hat und für die es keine Meldung gibt. Ist die Benachrichtigung `Fehlt ohne Meldung` eingeschaltet, ändert sich die Liste von selbst.",
    ],
    troubleshootingDetails: [
      "Es steht `Keine Klassen zugewiesen`? Dann hat die OGS Ihnen noch keine Klasse zugeordnet.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherArrivalChange,
      HELP_TOPICS.teacherClassTrip,
      HELP_TOPICS.teacherAbsenceReport,
    ],
  };
}
