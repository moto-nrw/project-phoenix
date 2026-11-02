import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Problemartikel: falsches Portal ist die haeufigste Ursache. */
export function teacherLoginProblemTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherLoginProblem,
    title: "Die Anmeldung klappt nicht",
    question: "Was tun, wenn die Anmeldung nicht klappt?",
    summary: "Meist wurde die Anmeldung der OGS statt moto schule benutzt.",
    group: "probleme",
    audience: "teacher",
    icon: "ShieldAlert",
    steps: [
      "Prüfen Sie, ob oben `Willkommen im Schulportal` steht.",
      "Steht dort etwas anderes, nutzen Sie die Adresse von moto schule.",
      "Prüfen Sie E-Mail-Adresse und Passwort.",
      "Nutzen Sie sonst `Passwort vergessen?`.",
    ],
    result: "Nach der Anmeldung landen Sie in der `Klassenansicht`.",
    notes: [
      "Der Link aus der Einladungs-E-Mail führt immer zur richtigen Adresse.",
    ],
    differences: [
      "`Dieses Konto hat keinen Zugang zum Schul-Portal.` Dann fehlt Ihrem Konto die Rolle. Wenden Sie sich an die OGS-Verwaltung.",
      "`Die E-Mail-Adresse oder das Passwort stimmt nicht.` Prüfen Sie beides.",
    ],
    troubleshootingDetails: [
      "Nach mehreren falschen Passwörtern warten Sie bitte 15 Minuten.",
      "Ein Passwortwechsel verkürzt diese Wartezeit nicht.",
      "Sie werden immer wieder abgemeldet? Melden Sie sich einmal neu an.",
    ],
    related: [
      HELP_TOPICS.teacherLogin,
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}
