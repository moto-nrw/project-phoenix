import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function attendanceProblemTopic(
  presenceMode: HelpPresenceMode,
): HelpTopic {
  return {
    id: HELP_TOPICS.attendanceProblem,
    title: "Ich kann ein Kind nicht an- oder abmelden",
    question: "Warum kann ich ein Kind nicht an- oder abmelden?",
    summary: "Prüfen Sie den aktuellen Status, Ihre Auswahl und Ihre Rechte.",
    group: "probleme",
    audience: "caregiver",
    icon: "ShieldAlert",
    steps: [
      "Öffnen Sie das Kind und prüfen Sie den aktuellen Status.",
      "Laden Sie die Seite neu, damit Sie den neuesten Stand sehen.",
      "Bei einem abwesenden Kind zeigt moto `Anmelden`.",
      "Bei einem anwesenden Kind zeigt moto `Abmelden`.",
      ...(presenceMode === "detailed"
        ? [
            "Möchten Sie den Raum ändern? Prüfen Sie, ob im Zielraum eine Aufsicht läuft.",
            "Öffnen Sie bei Bedarf `Aktuelle Aufsicht`.",
            "Prüfen Sie, ob Sie den aktuellen Raum oder den Zielraum beaufsichtigen.",
          ]
        : []),
      "Versuchen Sie die Aktion erneut.",
      "Fehlt die Aktion weiterhin? Bitten Sie Ihre Leitung, Ihr Recht zum An- und Abmelden zu prüfen.",
    ],
    result:
      "Stimmen Status und Rechte, ist die Aktion verfügbar. Sonst prüft Ihre Leitung den Zugang.",
    differences: [
      "Eine Krankmeldung oder Entschuldigung ist nicht dasselbe wie An- oder Abmelden.",
      "Bei einer gleichzeitigen Änderung durch eine andere Person kann moto den neueren Status anzeigen.",
    ],
    related: [HELP_TOPICS.webAttendance, HELP_TOPICS.activeSupervision],
  };
}
