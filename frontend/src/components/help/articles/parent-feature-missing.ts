import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Problemartikel: fehlende Bereiche haengen an den Schaltern der OGS. */
export function parentFeatureMissingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentFeatureMissing,
    title: "Eine Funktion ist für mich nicht verfügbar",
    question: "Warum fehlt mir ein Bereich im Elternportal?",
    summary: "Jede OGS schaltet selbst frei, was Eltern sehen und tun dürfen.",
    group: "probleme",
    audience: "parent",
    icon: "EyeSlash",
    steps: [
      "Prüfen Sie, ob der Bereich hinter `Mehr` liegt.",
      "Fehlt er auch dort, nutzt Ihre OGS ihn nicht.",
      "Brauchen Sie ihn, schreiben Sie der OGS eine Nachricht.",
    ],
    result: "Die OGS kann die Funktion für Ihre Schule einschalten.",
    notes: [
      "moto blendet einen Bereich aus, statt ihn zu sperren. Ein fehlender Eintrag ist kein Fehler.",
      "`Elternbriefe` und `Mittagessen` gibt es nur, wenn Ihre Schule sie führt.",
    ],
    differences: [
      "Manche Dinge dürfen Sie nur ansehen, nicht ändern. Dann fehlt der Knopf, nicht der Bereich.",
    ],
    troubleshootingDetails: [
      "Das Ändern von Angaben ist deaktiviert? Schreiben Sie der OGS, was geändert werden soll.",
    ],
    related: [
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}
