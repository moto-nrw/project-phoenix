import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function absencesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.absences,
    title: "Ein Kind krank oder entschuldigt melden",
    question: "Wie melde ich ein Kind krank oder entschuldigt?",
    summary:
      "Tragen Sie eine Krankmeldung oder Entschuldigung für die passenden Tage ein.",
    group: "kinder",
    audience: "caregiver",
    icon: "BellRing",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind krankmelden",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Krank melden`.",
          "Wählen Sie `Einzelne Tage` oder `Zeitraum`.",
          "Wählen Sie die Tage der Krankmeldung aus.",
          "Tragen Sie bei Bedarf einen Grund ein.",
          "Wählen Sie `Krankmelden`.",
        ],
      },
      {
        title: "Ein Kind entschuldigen",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Entschuldigen`.",
          "Wählen Sie `Ganzer Tag` oder `Ab Uhrzeit`.",
          "Bei `Ganzer Tag`: Wählen Sie `Einzelne Tage` oder `Zeitraum`. Wählen Sie danach die Tage aus.",
          "Bei `Ab Uhrzeit`: Wählen Sie einen Tag. Tragen Sie danach die passende Uhrzeit ein.",
          "Tragen Sie bei Bedarf einen Grund ein.",
          "Wählen Sie `Entschuldigen`.",
        ],
      },
      {
        title: "Bei `Ab Uhrzeit`",
        steps: [
          "`Ab Uhrzeit` gilt für genau einen Tag.",
          "Spätere Betreuungszeiten sind an diesem Tag entschuldigt.",
          "Ist keine Abholzeit eingetragen, übernimmt moto die gewählte Uhrzeit.",
        ],
        ordered: false,
      },
      {
        title: "Die Meldung für heute aufheben",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Gesund melden` oder `Entschuldigung aufheben`.",
          "Bestätigen Sie im Fenster mit `Gesundmelden` oder `Entschuldigung aufheben`.",
        ],
        description:
          "Das gilt auch, wenn die Eltern das Kind für heute abgemeldet haben.",
      },
      {
        title: "Einen geplanten Tag entfernen",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Krank melden` oder `Entschuldigen`.",
          "Steht dort `Gesund melden` oder `Entschuldigung aufheben`? Wählen Sie das. Wählen Sie danach im Fenster `Alle Kranktage ansehen` oder `Alle entschuldigten Tage ansehen`.",
          "Suchen Sie den Tag unter `Bereits krank` oder `Bereits entschuldigt`.",
          "Öffnen Sie beim Tag das Menü mit den drei Punkten.",
          "Wählen Sie `Entfernen`.",
          "Wählen Sie `Entfernen bestätigen`. Wählen Sie danach `Eintrag entfernen`.",
        ],
      },
      {
        // Live geprueft: die Liste `Bereits ab Uhrzeit entschuldigt` erscheint,
        // sobald `Ab Uhrzeit` gewaehlt ist. Ein Tag muss dafuer nicht gewaehlt sein.
        title: "Eine Entschuldigung `Ab Uhrzeit` entfernen",
        steps: [
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Entschuldigen`.",
          "Wählen Sie `Ab Uhrzeit`.",
          "Öffnen Sie unter `Bereits ab Uhrzeit entschuldigt` beim Tag das Menü mit den drei Punkten.",
          "Wählen Sie `Entfernen`. Für eine andere Uhrzeit wählen Sie `Bearbeiten`.",
          "Wählen Sie `Entfernen bestätigen`. Wählen Sie danach `Teilentschuldigung entfernen`.",
        ],
      },
    ],
    result:
      "Die Krankmeldung oder Entschuldigung ist gespeichert. Das Kind erscheint am gewählten Tag mit dem passenden Status.",
    notes: [
      "Die heutige Meldung aufheben ändert geplante Tage in der Zukunft nicht.",
    ],
    differences: [
      "Fehlen `Krank melden` und `Entschuldigen`? Fragen Sie Ihre Leitung, ob Ihre Rolle Abwesenheiten bearbeiten darf.",
      "Steht dort `Gesund melden` oder `Entschuldigung aufheben`? Dann ist das Kind heute krank gemeldet oder entschuldigt.",
      "Fehlt `Ab Uhrzeit`? Dann dürfen Sie nur ganze Tage entschuldigen.",
      "Steht beim Tag `Automatisch (Abholzeit)`? Dann kommt die Entschuldigung aus der früheren Abholzeit. Ändern Sie dafür die Abholzeit des Tages.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.webAttendance,
    ],
  };
}
