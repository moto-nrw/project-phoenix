import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/einstellungen` (#4652, ADR 0086): Träger-Werte zwischen Standard
 * und Schule. Bezeichnungen aus `settings-origin.ts` und `traeger-nav-items.ts`.
 */
export function traegerSettingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSettings,
    title: "Einstellungen für alle Schulen festlegen",
    question: "Wie lege ich Einstellungen für alle Schulen fest?",
    summary: "Was Sie hier festlegen, gilt für alle Schulen Ihres Trägers.",
    group: "konfiguration",
    audience: "traeger",
    icon: "Settings",
    requirements: ["Sie dürfen im Träger-Büro Einstellungen ändern."],
    steps: [
      "Klicken Sie links unten in der Navigation auf `Einstellungen`.",
      "Wählen Sie darunter noch einmal `Einstellungen`.",
      "Wählen Sie den Bereich, zum Beispiel `Anmeldung` oder `Betrieb`.",
      "Ändern Sie den Wert. moto speichert die Änderung von selbst.",
    ],
    result:
      "Der Wert gilt sofort für alle Schulen Ihres Trägers. Auch neue Schulen übernehmen ihn.",
    notes: [
      "Bei Ihrem Wert steht `Vom Träger festgelegt` und das Datum der letzten Änderung.",
      "`Standard` heißt: Sie haben nichts festgelegt. Es gilt der Wert von moto.",
      "`Zurücksetzen` entfernt Ihren Wert. Dann gilt wieder der Standard.",
      "Eine Schule kann einen eigenen Wert eintragen. Dann gilt dort ihr Wert.",
      "Den eigenen Wert einer Schule kann nur die Schule selbst zurücksetzen.",
    ],
    differences: [
      "Auf dem Handy steht `Einstellungen` in der Leiste unten oder unter `Mehr`.",
      "Sie können die Werte nur ansehen? Ihnen fehlt das Recht zum Ändern. Wenden Sie sich an das moto-Team.",
    ],
    related: [HELP_TOPICS.traegerLogin, HELP_TOPICS.errorMessages],
  };
}
