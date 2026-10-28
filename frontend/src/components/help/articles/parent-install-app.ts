import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Eltern-Fassung des App-Themas. Der Samsung-Hinweis ist echt und steht in
 * den Einstellungen wie im Installationshinweis (i18n `pwaInstallHint`).
 */
export function parentInstallAppTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentInstallApp,
    title: "moto als App hinzufügen",
    question: "Wie bekomme ich moto auf meinen Startbildschirm?",
    summary: "Sie fügen moto zum Startbildschirm hinzu. Ohne App Store.",
    group: "einstieg",
    audience: "parent",
    icon: "Smartphone",
    steps: [],
    instructionGroups: [
      {
        title: "Auf dem iPhone oder iPad",
        steps: [
          "Öffnen Sie moto in Safari.",
          "Tippen Sie unten auf `Teilen`.",
          "Tippen Sie auf `Zum Home-Bildschirm`.",
        ],
      },
      {
        title: "Auf einem Android-Gerät",
        steps: [
          "Öffnen Sie moto in Chrome.",
          "Öffnen Sie das Browser-Menü.",
          "Tippen Sie auf `App installieren` oder `Zum Startbildschirm hinzufügen`.",
        ],
      },
    ],
    result: "moto öffnet sich danach im Vollbild, ohne Browserleiste.",
    notes: [
      "Manchmal bietet moto die Installation direkt an. Dann genügt ein Tippen auf `App installieren`.",
    ],
    differences: [
      "Nutzen Sie Samsung Internet? Installieren Sie moto darüber nicht. Öffnen Sie moto in Chrome und melden Sie sich dort an.",
    ],
    troubleshootingDetails: [
      "Sie finden den Punkt im Menü nicht? Prüfen Sie, ob Sie wirklich in Safari oder Chrome sind.",
      "Benachrichtigungen kommen erst an, wenn moto auf dem Startbildschirm liegt.",
    ],
    related: [
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}
