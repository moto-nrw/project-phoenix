import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Elternportal-Schalter, alle im Einstellungsbereich `Betrieb`. */
export function parentVisibilityTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentVisibility,
    title: "Festlegen, was Eltern in moto sehen",
    question: "Was sehen Eltern von unserer OGS?",
    summary: "Sie schalten einzeln ein, was Eltern sehen und tun dürfen.",
    group: "konfiguration",
    audience: "lead",
    icon: "Eye",
    steps: [
      "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
      "Bleiben Sie oben im Reiter `Betrieb`.",
      "Tragen Sie in `Einstellung suchen` `Elternportal` ein.",
      "Gehen Sie zur Gruppe `Elternportal`.",
      "Schalten Sie ein oder aus, was Eltern dürfen sollen.",
    ],
    result: "Eltern sehen die Änderung, wenn sie moto das nächste Mal öffnen.",
    notes: [
      "Einzeln schaltbar sind zum Beispiel `Nachrichten von Eltern`, `Abholung für einen Tag ändern (Eltern)` und `Stammdaten bearbeiten (Eltern)`.",
      "Wählen Sie bei `Krankmeldungen durch Eltern` die Möglichkeit `Erst bestätigen`, wenn Ihr Team jede Meldung freigeben soll.",
      "Bei `Wer gibt Änderungswünsche von Eltern frei?` wählen Sie, wer Anfragen zu Stammdaten, Abholung und Angeboten freigibt.",
      "`Berechtigtes Team` heißt: alle, die Kinderdaten bearbeiten dürfen. Das passt auch für ein gemeinsames Tablet.",
      "Speiseplan und Anmeldung zum Mittagessen stehen im Reiter `Mittagessen`.",
    ],
    differences: [
      "Ist ein Schalter aus, sehen Eltern den Bereich gar nicht. Er wird nicht ausgegraut.",
      "`Weitere Bezugspersonen einladen (Eltern)` ist kein Schalter, sondern eine Auswahl: `Nicht erlaubt`, `Ohne Freigabe` oder `Mit Freigabe durch das Team`.",
      "Bei `Mit Freigabe durch das Team` landen die Einladungen unter `Elternzugänge`.",
    ],
    troubleshootingDetails: [
      "Eltern kommen nicht hinein? Dann fehlt ihr Zugang. Das steht unter `Elternzugänge`.",
    ],
    related: [
      HELP_TOPICS.settings,
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadMealPlan,
    ],
  };
}
