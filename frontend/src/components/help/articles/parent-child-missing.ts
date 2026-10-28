import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Problemartikel: kein Kind sichtbar. Der Zugang haengt an der Freigabe. */
export function parentChildMissingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildMissing,
    title: "Mein Kind wird nicht angezeigt",
    question: "Warum sehe ich mein Kind nicht?",
    summary: "Die OGS muss Ihren Zugang zum Kind erst freigeben.",
    group: "probleme",
    audience: "parent",
    icon: "Search",
    steps: [
      "Prüfen Sie, ob Sie mit der richtigen E-Mail-Adresse angemeldet sind.",
      "Haben Sie mehrere Kinder, prüfen Sie oben die Auswahl.",
      "Schreiben Sie sonst der OGS und nennen Sie Name und Klasse des Kindes.",
    ],
    result: "Sobald die OGS Ihren Zugang freigibt, erscheint das Kind.",
    notes: [
      "Ein Kind kommt nicht automatisch zu Ihrem Konto. Die OGS verbindet beides.",
      "Eine abgegebene Anmeldung allein genügt nicht. Das Kind erscheint erst nach der Übernahme.",
    ],
    differences: [
      "moto sagt, mit Ihrem Konto sei noch kein Kind verknüpft? Dann fehlt die Freigabe.",
    ],
    troubleshootingDetails: [
      "Sie haben zwei E-Mail-Adressen? Melden Sie sich mit der an, an die die Einladung ging.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentMessages,
    ],
  };
}
