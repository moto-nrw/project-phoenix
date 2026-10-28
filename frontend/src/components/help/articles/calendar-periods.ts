import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/calendar-periods` heisst in Seitenleiste und Kopfzeile `Schuljahr und
 * Kalender` (#4778).
 */
export function calendarPeriodsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCalendarPeriods,
    title: "Schuljahr, Ferien und Schließtage eintragen",
    question: "Wo trage ich Schuljahr, Ferien und Schließtage ein?",
    summary: "moto plant nur an Tagen, an denen Ihre OGS geöffnet hat.",
    group: "planung",
    audience: "lead",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Zeitraum oder Schließtag?",
        steps: [
          "Ein Zeitraum der Art `Ferien`, etwa die Herbstferien, markiert nur die Ferienzeit. Termine und Sollstunden bleiben.",
          "Hat die OGS zu, legen Sie einen Schließtag an. Dann fallen Termine aus, und niemand hat Sollstunden.",
          "Arbeitet jemand trotzdem, etwa in der Ferienbetreuung? Tragen Sie für die Person eine Sonderarbeitszeit ein.",
        ],
        ordered: false,
      },
      {
        title: "Einen Zeitraum anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Planung` auf.",
          "Öffnen Sie `Schuljahr und Kalender`.",
          "Wählen Sie `Halbjahr anlegen` oder `Zeitraum anlegen`.",
          "Tragen Sie `Bezeichnung`, `Art`, `Startdatum` und `Enddatum` ein. Für Ferien wählen Sie die Art `Ferien`.",
          "Wählen Sie `Anlegen`.",
        ],
      },
      {
        title: "Einen Schließtag eintragen",
        steps: [
          "Öffnen Sie `Schuljahr und Kalender`.",
          "Gehen Sie zum Abschnitt `Schließtage`.",
          "Wählen Sie `Schließtag anlegen`.",
          "Tragen Sie `Grund`, `Von` und `Bis` ein, zum Beispiel `Herbstferien`.",
          "Wählen Sie `Speichern`.",
          "Stehen an diesen Tagen schon Termine, fragt moto nach. Sagen Sie sie ab wie im nächsten Abschnitt.",
        ],
      },
      {
        title: "Die gesetzlichen Feiertage prüfen",
        description:
          "moto richtet die Feiertage nach dem Bundesland Ihrer Schule. Sie müssen sie nicht eintragen.",
        steps: [
          "Öffnen Sie `Schuljahr und Kalender`.",
          "Gehen Sie zum Abschnitt `Gesetzliche Feiertage`.",
          "Prüfen Sie das Bundesland im Text über der Liste.",
          "Stimmt das Bundesland nicht? Wenden Sie sich an das moto-Team. Nur das moto-Team kann es ändern.",
        ],
      },
      {
        title: "Termine an Schließtagen absagen",
        description: "Für Termine, die schon vor dem Schließtag geplant waren.",
        steps: [
          "Wählen Sie beim Schließtag die drei Punkte.",
          "Wählen Sie `Termine absagen`.",
          "Prüfen Sie den `Zeitraum`. Sie können ihn kürzen, etwa auf die erste Ferienwoche.",
          "Serien für die Ferienbetreuung bleiben im Plan. Sollen sie auch ausfallen, setzen Sie das Häkchen bei `Auch diese Serien absagen`.",
          "Wählen Sie `Termine absagen` und dann `Endgültig absagen`.",
        ],
      },
    ],
    result:
      "Im Betreuungsplan plant moto an Schließtagen und gesetzlichen Feiertagen keine Termine. Abgesagte Termine verschwinden aus dem Plan. Eltern bekommen keine Nachricht.",
    notes: [
      "Ein Zeitraum, den noch nichts benutzt, trägt `Nicht verwendet`.",
      "Ferienbetreuung an Schließtagen: Wählen Sie beim Speichern der Serie `Auch an Schließtagen planen`. Tragen Sie bei `Letzter Tag` den letzten Ferientag ein. Dann endet die Serie mit den Ferien.",
      "Schicht-Serien im Dienstplan lassen Ferien und Schließtage aus.",
      "Mit `Auch in den Ferien und an Schließtagen planen` planen Sie auch dann Schichten.",
      "An gesetzlichen Feiertagen plant eine Serie nie Schichten.",
      "Schichten, die schon geplant waren, bleiben stehen.",
      "Termine ohne Schließtag absagen: Wählen Sie im Betreuungsplan im Menü `Termine im Zeitraum absagen`.",
    ],
    differences: [
      "Legen Sie eine Schicht auf einen Schließtag, fragt moto vorher nach.",
      "Sie sehen `Termine absagen` nicht? Dann fehlt Ihnen das Recht, den Betreuungsplan zu bearbeiten.",
    ],
    troubleshootingDetails: [
      "Der Betreuungsplan sagt `Noch kein Planungszeitraum`? Dann fehlt für diese Woche ein Zeitraum.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}
