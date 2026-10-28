import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger` (#4654): Absprung in eine Schule des Trägers. */
export function traegerSchoolSwitchTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSchoolSwitch,
    title: "In eine Schule wechseln",
    question: "Wie arbeite ich in einer Schule meines Trägers?",
    summary:
      "Sie öffnen jede Schule Ihres Trägers, ohne sich dort neu anzumelden.",
    group: "einstieg",
    audience: "traeger",
    icon: "ArrowsLeftRight",
    requirements: ["Sie haben die Rolle `Verwaltung`."],
    steps: [
      "Wählen Sie in der Navigation `Schulen`.",
      "Wählen Sie bei der Schule `Öffnen`.",
    ],
    result:
      "moto öffnet die Schule in einem neuen Tab. Dort arbeiten Sie mit der Rolle Verwaltung der Schule.",
    notes: [
      "Sie können mehrere Schulen in mehreren Tabs offen haben.",
      "Haben Sie in einem anderen Tab die Schule gewechselt? Dann stellt moto diesen Tab beim nächsten Klick wieder auf seine Schule um. Dieser Klick wird nicht ausgeführt. Wiederholen Sie ihn danach.",
      "Eine neue Schule Ihres Trägers können Sie sofort öffnen. Niemand muss Sie dafür freischalten.",
      "Jedes Öffnen steht im Protokoll der Schule.",
    ],
    differences: [
      "Sie sehen kein `Öffnen`? Dann haben Sie nicht die Rolle `Verwaltung`.",
    ],
    troubleshootingDetails: [
      "moto zeigt `Die Schule konnte nicht geöffnet werden.`? Der Link gilt nur kurz und nur einmal. Öffnen Sie die Schule noch einmal im Träger-Portal.",
      "moto sagt, die Schule gehöre nicht zu Ihrem Träger? Laden Sie die Seite neu. Fehlt die Schule dann, wenden Sie sich an das moto-Team.",
      "moto bittet Sie, sich neu anzumelden? Melden Sie sich im Träger-Portal neu an und öffnen Sie die Schule noch einmal.",
    ],
    related: [HELP_TOPICS.traegerLogin],
  };
}
