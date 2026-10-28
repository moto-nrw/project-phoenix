import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/news`: Elternbriefe, Mitteilungen und Umfragen. Manche verlangen
 * eine Lesebestaetigung, Umfragen eine Antwort.
 */
export function parentNewsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentNews,
    title: "Elternbriefe lesen und beantworten",
    question: "Wo finde ich Post von der OGS?",
    summary:
      "Unter `Elternbriefe` stehen Mitteilungen, Umfragen und Anfragen zum Einverständnis.",
    group: "nachrichten",
    audience: "parent",
    icon: "Megaphone",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Elternbrief bestätigen",
        steps: [
          "Tippen Sie unten auf `Mehr`.",
          "Wählen Sie `Elternbriefe`.",
          "Tippen Sie auf den Eintrag unter `Offen`.",
          "Lesen Sie den Text.",
          "Tippen Sie auf `Gelesen bestätigen`.",
        ],
      },
      {
        title: "Eine Umfrage beantworten",
        steps: [
          "Öffnen Sie die Umfrage unter `Offen`.",
          "Wählen Sie Ihre Antwort.",
          "Tippen Sie auf `Antwort speichern`.",
        ],
      },
      {
        title: "Auf ein Einverständnis antworten",
        steps: [
          "Öffnen Sie den Eintrag `Einverständnis` unter `Offen`.",
          "Lesen Sie den Text und die Dateien.",
          "Tippen Sie beim Kind auf `Zustimmen` oder `Ablehnen`.",
          "Prüfen Sie im Fenster das Kind und Ihre Antwort.",
          "Verlangt die Schule eine Unterschrift? Unterschreiben Sie im Feld mit dem Finger oder der Maus. Tippen Sie dann auf `Unterschrift bestätigen`.",
          "Bestätigen Sie im Fenster mit demselben Knopf.",
        ],
      },
    ],
    result: "Erledigte Einträge wandern von `Offen` zu `Erledigt`.",
    notes: [
      "Die Bestätigung heißt `Lesebestätigung`. Damit bestätigen Sie nur, dass Sie den Brief gelesen haben.",
      "Bei mehreren Kindern zeigt moto, wie viele Antworten noch fehlen.",
      "Angehängte Dateien öffnen Sie, indem Sie auf den Namen tippen.",
      "Nach Ihrer Antwort öffnen Sie mit `Nachweis ansehen` Ihren Nachweis. Mit `Als PDF herunterladen` speichern Sie ihn.",
    ],
    differences: [
      "Umfragen tragen über dem Titel `Umfrage`, Einverständnisse `Einverständnis`, abgesagte Betreuung trägt `Betreuung fällt aus`.",
      "Unten am Eintrag steht, ob er `Wichtig` ist, ob Sie eine `Erinnerung` bekommen haben und ob eine `Bestätigung erforderlich` ist.",
      "Fragt moto nach Ihrem Passwort? Dann möchte die Schule das vor jeder Antwort.",
      "Bei einer Umfrage steht `Antwort bis` mit dem letzten Tag.",
      "Ist die Frist vorbei, steht dort `Umfrage geschlossen`.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Elternbrief sei nicht mehr aktuell? Laden Sie die Seite neu.",
      "moto sagt, die Schule habe den Text geändert? Lesen Sie ihn noch einmal und antworten Sie neu.",
      "Fehlt `Elternbriefe` ganz? Dann nutzt Ihre OGS diese Funktion nicht.",
    ],
    related: [
      HELP_TOPICS.parentSurveys,
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentFeatureMissing,
    ],
  };
}
