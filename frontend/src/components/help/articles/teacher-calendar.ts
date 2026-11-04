import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function teacherCalendarTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherCalendar,
    title: "Den Kalender nutzen",
    question: "Wie trage ich einen Ausflug oder Schultermin ein?",
    summary:
      "Der Kalender zeigt Termine der OGS, Schließtage, Ferien und Ihre Schultermine.",
    group: "nachrichten",
    audience: "teacher",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Schultermin eintragen",
        steps: [
          "Öffnen Sie `Kalender`.",
          "Wählen Sie `Neuer Termin`.",
          "Geben Sie einen Titel ein und wählen Sie die Art, zum Beispiel `Ausflug`.",
          "Wählen Sie Tag und Uhrzeit. Oder setzen Sie den Haken bei `Ganztägig`.",
          "Wählen Sie unter `Wer sieht den Termin?`, ob auch Eltern ihn sehen.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Auf eine Einladung der OGS antworten",
        steps: [
          "Wählen Sie den Termin im Kalender.",
          "Wählen Sie `Zusagen` oder `Absagen`.",
        ],
      },
    ],
    result:
      "Die OGS und alle Lehrkräfte sehen Ihren Schultermin. Die Eltern sehen ihn nur, wenn Sie das gewählt haben.",
    notes: [
      "Ihre eigenen Schultermine öffnen Sie im Kalender. Dort finden Sie `Bearbeiten` und `Löschen`.",
      "Schließtage und Ferien trägt die OGS ein. Sie stehen ganztägig im Kalender.",
      "Unter `Kalender abonnieren` holen Sie die Termine in Ihren Handy-Kalender.",
    ],
    differences: [
      "Wer den Termin sieht, wählen Sie nur beim Anlegen. Beim Ändern bleibt es gleich.",
      "`Eltern meiner Klassen` ist grau? Dann ist Ihnen keine Klasse zugeordnet.",
    ],
    related: [
      HELP_TOPICS.teacherNotices,
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
