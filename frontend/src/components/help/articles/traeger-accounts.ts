import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/accounts` (#4650): Personen einladen, Rollen ändern, entfernen. */
export function traegerAccountsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerAccounts,
    title: "Personen ins Träger-Büro einladen",
    question:
      "Wie lade ich Personen ins Träger-Büro ein und gebe ihnen Rollen?",
    summary:
      "Unter `Personen` laden Sie Kolleginnen und Kollegen ein. Die Rolle bestimmt, was eine Person darf.",
    group: "einstieg",
    audience: "traeger",
    icon: "UsersThree",
    requirements: ["Sie haben die Rolle `Verwaltung`."],
    instructionGroups: [
      {
        title: "Person einladen",
        steps: [
          "Wählen Sie in der Navigation `Personen`.",
          "Wählen Sie `Person einladen`.",
          "Tragen Sie die `E-Mail-Adresse` ein.",
          "Wählen Sie mindestens eine Rolle.",
          "Wählen Sie `Einladung senden`.",
        ],
      },
      {
        title: "Rollen ändern",
        steps: [
          "Wählen Sie in der Navigation `Personen`.",
          "Öffnen Sie bei der Person das Menü mit den drei Punkten.",
          "Wählen Sie `Rollen ändern`.",
          "Wählen Sie die Rollen und dann `Speichern`.",
        ],
      },
      {
        title: "Person entfernen",
        steps: [
          "Wählen Sie in der Navigation `Personen`.",
          "Öffnen Sie bei der Person das Menü mit den drei Punkten.",
          "Wählen Sie `Aus dem Träger-Büro entfernen`.",
          "Wählen Sie `Ja, entfernen` und dann `Entfernen`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die eingeladene Person steht unter `Offene Einladungen`, bis sie die Einladung annimmt.",
    notes: [
      "`Verwaltung` darf alles verwalten, auch Personen. `Abrechnung` darf abrechnen und Finanz-Daten exportieren. `Personal` darf das Personal verwalten. `Nur lesen` darf die Übersicht und die Berichte ansehen.",
      "Eine Person kann mehrere Rollen haben. Wählen Sie alle Rollen auf einmal aus.",
      "Unter `Offene Einladungen` stehen auch Einladungen, die das moto-Team verschickt hat.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Personen`? Dann haben Sie nicht die Rolle `Verwaltung`.",
      "Geänderte Rollen gelten nach wenigen Minuten. Eine entfernte Person kann sich nicht mehr anmelden. Eine offene Sitzung endet nach wenigen Minuten.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Träger-Büro brauche mindestens eine Person mit der Rolle Verwaltung? Geben Sie die Rolle zuerst einer anderen Person.",
      "Die Einladung ist abgelaufen? Laden Sie die Person noch einmal ein. Die alte Einladung gilt dann nicht mehr.",
      "Eine Einladung soll nicht mehr gelten? Wählen Sie bei der Einladung `Einladung zurücknehmen`.",
    ],
    related: [HELP_TOPICS.traegerInvitation],
  };
}
