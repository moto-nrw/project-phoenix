import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/einstellungen/mitglieder` (#4650, #5474, #5685, #4693): Personen einladen, Namen und Rollen ändern, entfernen, Verlauf. */
export function traegerAccountsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerAccounts,
    title: "Personen ins Träger-Büro einladen",
    question:
      "Wie lade ich Personen ins Träger-Büro ein und gebe ihnen Rollen?",
    summary:
      "Unter `Mitglieder und Rollen` laden Sie Kolleginnen und Kollegen ein. Die Rolle bestimmt, was eine Person darf.",
    group: "einstieg",
    audience: "traeger",
    icon: "UsersThree",
    requirements: ["Sie haben die Rolle `Leitung`."],
    instructionGroups: [
      {
        title: "Person einladen",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Mitglieder und Rollen`.",
          "Wählen Sie `Person einladen`.",
          "Tragen Sie die `E-Mail-Adresse` ein.",
          "Wählen Sie mindestens eine Rolle.",
          "Wählen Sie `Einladung senden`.",
        ],
      },
      {
        title: "Namen eintragen oder ändern",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Mitglieder und Rollen`.",
          "Öffnen Sie bei der Person das Menü mit den drei Punkten.",
          "Wählen Sie `Namen eintragen`. Hat die Person schon einen Namen, heißt der Eintrag `Namen ändern`.",
          "Tragen Sie den `Name` ein und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Rollen ändern",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Mitglieder und Rollen`.",
          "Öffnen Sie bei der Person das Menü mit den drei Punkten.",
          "Wählen Sie `Rollen ändern`.",
          "Wählen Sie die Rollen und dann `Speichern`.",
        ],
      },
      {
        title: "Person entfernen",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Mitglieder und Rollen`.",
          "Öffnen Sie bei der Person das Menü mit den drei Punkten.",
          "Wählen Sie `Aus dem Träger-Büro entfernen`.",
          "Wählen Sie `Ja, entfernen` und dann `Entfernen`.",
        ],
      },
      {
        title: "Verlauf ansehen",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Mitglieder und Rollen`.",
          "Scrollen Sie nach unten zu `Verlauf`.",
          "Ältere Änderungen sehen Sie mit `Ältere Einträge laden`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die eingeladene Person steht unter `Offene Einladungen`, bis sie die Einladung annimmt.",
    notes: [
      "`Leitung` darf Personen einladen, ihre Rollen ändern und sie entfernen. `Verwaltung` darf Anmeldungen, Verträge, Einstellungen und das Protokoll verwalten. `Dokumente` darf Dokumente für Eltern und Schulen hochladen, freigeben und löschen. `Schulzugang` darf jede Schule des Trägers öffnen. `Abrechnung` darf abrechnen und Finanzdaten exportieren. `Personal` darf das Personal verwalten. `Nur lesen` darf die Übersicht und die Berichte ansehen.",
      "Nur eine Person mit `Leitung` kann `Leitung` vergeben oder wegnehmen. Ihre eigene `Leitung` geben Sie erst ab, wenn eine andere Person sie hat.",
      "Eine Person kann mehrere Rollen haben. Wählen Sie alle Rollen auf einmal aus.",
      "Wer bisher `Verwaltung` hatte, hat `Dokumente` dazubekommen. Soll sich eine andere Person um die Dokumente kümmern? Geben Sie ihr `Dokumente` und nehmen Sie die Rolle bei der Verwaltung weg.",
      "Unter `Offene Einladungen` stehen auch Einladungen, die das moto-Team verschickt hat.",
      "Der Name steht im Exportprotokoll und im Verlauf jeder Rechnung, die E-Mail-Adresse nie. Steht bei einer Person `Kein Name`, zeigen diese Stellen „von einem Konto ohne Namen“.",
      "Jede Person kann ihren eigenen Namen auch selbst ändern: im Profil-Menü unter `Konto`.",
      "Unter `Verlauf` steht, wer wann eingeladen, Rollen oder Namen geändert oder entfernt hat. Bei Rollen sehen Sie, welche vorher galten und welche jetzt gelten.",
      "Ändern Sie die Rollen einer Person oder entfernen Sie sie, bekommt die Person eine E-Mail. Darin steht, wer es war und wann. Das gilt auch, wenn Sie Ihre eigenen Rollen ändern.",
    ],
    differences: [
      "Sie sehen in den Einstellungen kein `Mitglieder und Rollen`? Dann haben Sie nicht die Rolle `Leitung`.",
      "Neue Rollen gelten nach wenigen Minuten. Nehmen Sie eine Rolle weg, wird die Person sofort abgemeldet. Sie meldet sich dann neu an. Das gilt auch für Ihre eigenen Rollen.",
      "Eine entfernte Person wird sofort abgemeldet. Sie kann sich nicht mehr anmelden.",
    ],
    troubleshootingDetails: [
      "Sie können bei `Leitung` keinen Haken entfernen? Dann hat nur diese Person die Rolle. Geben Sie die Rolle zuerst einer anderen Person.",
      "moto sagt, das Träger-Büro brauche mindestens eine Person mit der Rolle Leitung? Geben Sie die Rolle zuerst einer anderen Person.",
      "Die Einladung ist abgelaufen? Laden Sie die Person noch einmal ein. Die alte Einladung gilt dann nicht mehr.",
      "Eine Einladung soll nicht mehr gelten? Wählen Sie bei der Einladung `Einladung zurücknehmen`.",
    ],
    related: [HELP_TOPICS.traegerInvitation, HELP_TOPICS.traegerProfile],
  };
}
