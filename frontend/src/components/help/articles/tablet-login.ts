import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { unavailableNfcTopic, unknownNfcTopic } from "../help-article-shared";

export function tabletLoginTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
      "Password",
      [HELP_TOPICS.login, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
      "Password",
      [HELP_TOPICS.login, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.tabletLogin,
    title: "Mit der PIN am Tablet anmelden",
    question: "Wie melde ich mich mit meiner PIN am Tablet an?",
    summary:
      "Öffnen Sie mit der Geräte-PIN den geschützten Bereich des Tablets.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Password",
    requirements: ["Die vierstellige Geräte-PIN Ihrer OGS"],
    steps: [
      "Wählen Sie auf dem Startbildschirm `Anmelden`.",
      "Geben Sie die vierstellige PIN über das Zahlenfeld ein.",
      "Warten Sie nach der vierten Ziffer. Das Tablet prüft die PIN automatisch.",
    ],
    result: "Bei richtiger PIN öffnet sich das `Menü`.",
    differences: [
      "Es gibt keine Schaltfläche zum Bestätigen.",
      "Bei einer falschen Eingabe wählen Sie `C` und geben die PIN erneut ein.",
      "Die Geräte-PIN ist nicht Ihr persönliches Passwort für die moto-App.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcSupervision, HELP_TOPICS.tagAssignment],
  };
}
