import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function installAppTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.installApp,
    title: "moto als App auf dem Handy oder Tablet hinzufügen",
    question: "Wie füge ich moto auf meinem Handy oder Tablet hinzu?",
    summary:
      "Fügen Sie moto zum Startbildschirm hinzu. Sie brauchen dafür keinen App Store.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "Smartphone",
    requirements: [
      "Ein Handy oder Tablet",
      "Den Link zur moto-Seite Ihrer OGS",
      "Safari auf dem iPhone oder iPad oder Chrome auf einem Android-Gerät",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Auf dem iPhone oder iPad",
        steps: [
          "Öffnen Sie die moto-Seite Ihrer OGS in Safari.",
          "Melden Sie sich bei moto an.",
          "Tippen Sie auf das Teilen-Symbol. Wählen Sie bei Bedarf zuerst `Mehr`.",
          "Wählen Sie `Zum Home-Bildschirm`.",
          "Schalten Sie `Als Web-App öffnen` ein. Wählen Sie danach `Hinzufügen`.",
        ],
      },
      {
        title: "Auf einem Android-Handy oder Android-Tablet",
        steps: [
          "Öffnen Sie die moto-Seite Ihrer OGS in Chrome.",
          "Melden Sie sich bei moto an.",
          "Ab dem zweiten Besuch erscheint der Hinweis `moto als App nutzen`.",
          "Wählen Sie `App installieren`.",
          "Wählen Sie im nächsten Fenster `Installieren`.",
        ],
      },
    ],
    result:
      "Auf Ihrem Startbildschirm erscheint ein neues moto-Symbol. Öffnen Sie moto künftig über dieses Symbol.",
    differences: [
      "Der Hinweis erscheint auf Android nicht? Öffnen Sie in Chrome die drei Punkte. Wählen Sie `App installieren` oder `Zum Startbildschirm hinzufügen`.",
      "Nutzen Sie Samsung Internet? Wählen Sie im Hinweis `In Chrome öffnen`. Führen Sie die Schritte danach in Chrome aus.",
      "`Unsichere App blockiert` erscheint? moto selbst ist nicht unsicher. Aktualisieren Sie im Play Store Chrome und Google Play. Starten Sie das Gerät neu. Versuchen Sie es danach erneut.",
      "Arbeiten Sie für mehrere Einrichtungen? Fügen Sie jede Einrichtung einzeln zum Startbildschirm hinzu.",
    ],
    related: [
      HELP_TOPICS.login,
      HELP_TOPICS.mySchedule,
      HELP_TOPICS.leadGoLive,
    ],
  };
}
