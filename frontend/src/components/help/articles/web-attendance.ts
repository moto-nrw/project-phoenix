import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function webAttendanceTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const result =
    presenceMode === "detailed"
      ? "Nach dem Anmelden steht das Kind zunächst unter `Unterwegs`. Ein Raum wird erst bei der nächsten Zuordnung eingetragen. Nach dem Abmelden steht das Kind unter `Zuhause`."
      : presenceMode === "binary"
        ? "Nach dem Anmelden steht das Kind auf `Anwesend`. Nach dem Abmelden steht es auf `Zuhause`."
        : "moto zeigt nach der Aktion den neuen Anwesenheitsstatus des Kindes.";

  return {
    id: HELP_TOPICS.webAttendance,
    title: "Ein Kind an- oder abmelden",
    question: "Wie melde ich ein Kind an oder ab?",
    summary: "Tragen Sie ein, ob ein Kind heute in der OGS ist.",
    group: "kinder",
    audience: "caregiver",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Über die Angaben des Kindes",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Anmelden` oder `Abmelden`.",
          "Prüfen Sie den Namen im Fenster.",
          "Bestätigen Sie mit `Anmelden` oder `Geht nach Hause`.",
        ],
      },
      {
        title: "Direkt in der Kinderliste",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Wählen Sie `An- & Abmelden`.",
          "Wählen Sie die Karte des Kindes.",
          "Prüfen Sie den neuen Status auf der Karte.",
          "Wählen Sie danach `Fertig`.",
        ],
      },
      {
        // #4762: „Alle N auswählen“ nimmt nur die sichtbaren Karten und
        // lässt Kinder aus, die heute krank, entschuldigt oder nicht
        // erwartet sind.
        title: "Eine ganze Klasse anmelden",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Wählen Sie im Filter `Klasse` die Klasse.",
          "Wählen Sie `An- & Abmelden` und danach `Mehrere`.",
          "Tippen Sie auf den Knopf mit `Alle` und der Zahl der Kinder.",
          "Die Zahl nennt die ausgewählten Kinder.",
          "Tippen Sie Kinder an, die nicht da sind. So nehmen Sie sie wieder heraus.",
          "Wählen Sie `Anmelden`.",
          "Lesen Sie den Hinweis `Noch nicht angemeldet`. Er nennt die Kinder, die noch fehlen.",
        ],
      },
    ],
    result,
    notes: [
      "Möchten Sie mehrere Kinder ändern? Wählen Sie `Mehrere`.",
      "Der Knopf mit `Alle` lässt kranke und entschuldigte Kinder aus. Das gilt auch für Kinder auf Klassenfahrt oder mit „Kommt heute nicht“. Die Leiste nennt ihre Zahl.",
      "Im Hinweis `Noch nicht angemeldet` zeigt `Nur „Schule“ zeigen` die Kinder, die noch im Unterricht sind.",
      "In der Listenansicht kreuzen Sie die Kinder links an und wählen oben `Anmelden` oder `Abmelden`.",
      // Live geprueft: in `Direkt` schreibt jeder Tipp sofort, ohne Rueckfrage.
      // Ausnahme seit #3324: frühes Gehen öffnet vorher ein Fenster.
      "In `Direkt` meldet jeder Tipp sofort an oder ab. Auf der Karte steht `Tippen zum Anmelden`.",
      "Geht ein Kind früher als seine Abholzeit, fragt moto nach dem Grund. Der Grund ist freiwillig.",
      "Den Grund sehen Sie danach beim Kind unter `Heutige Abholung`. Mit Anwesenheitsprotokoll steht er auch in der `Tagesauswertung` und im Verlauf.",
    ],
    differences: [
      "moto zeigt nur die Aktion, die zum aktuellen Status passt.",
      "Fehlt `Anmelden` oder `Abmelden`? Fragen Sie Ihre Leitung nach dem Recht zum An- und Abmelden.",
      "moto fragt nie nach einem Grund? Dann hat Ihre Schule die Frage ausgeschaltet oder das Kind hat heute keine Abholzeit.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.changeLocation,
      HELP_TOPICS.absences,
    ],
  };
}
