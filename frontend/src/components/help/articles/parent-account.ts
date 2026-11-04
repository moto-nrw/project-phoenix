import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Einladung annehmen unter `/parents/accept-guardian-invite/[token]`. Die
 * Passwortregeln stehen im Formular selbst (i18n `guardianInvite.passwordRules`).
 */
export function parentAccountTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentAccount,
    title: "Mein Eltern-Konto einrichten",
    question: "Wie richte ich mein Eltern-Konto ein?",
    summary: "Sie öffnen den Link aus der E-Mail und legen Ihr Passwort fest.",
    group: "einstieg",
    audience: "parent",
    icon: "KeyRound",
    requirements: ["Sie haben eine Einladungs-E-Mail Ihrer OGS bekommen."],
    steps: [
      "Öffnen Sie den Link aus der E-Mail.",
      "Prüfen Sie oben, für welche Schule die Einladung gilt.",
      "Tragen Sie bei `Passwort` Ihr neues Passwort ein.",
      "Tragen Sie es bei `Passwort bestätigen` noch einmal ein.",
      "Wählen Sie `Einladung akzeptieren`.",
      "Melden Sie sich danach mit Ihrer E-Mail-Adresse und dem neuen Passwort an.",
    ],
    result:
      "Ihr Konto ist angelegt. Sie sehen danach Ihr Kind im Elternportal.",
    notes: [
      "Das Passwort braucht mindestens 8 Zeichen, einen Großbuchstaben, einen Kleinbuchstaben, eine Zahl und ein Sonderzeichen.",
      "Unter `Passwortanforderungen` sehen Sie, was noch fehlt.",
      "Zur Einladung bekommen Sie eine zweite E-Mail: `Willkommen bei moto`. Sie führt zu dieser Anleitung und zur Elterninfo. Den Link zum Einrichten finden Sie nur in der Einladungs-E-Mail.",
    ],
    differences: [
      "Sie sehen `Konto erstellt`? Dann hat es geklappt. Melden Sie sich jetzt an.",
    ],
    troubleshootingDetails: [
      "Die Seite sagt, die Einladung sei abgelaufen oder schon benutzt? Bitten Sie Ihre OGS um eine neue Einladung.",
      "Die Seite sagt, für diese E-Mail gebe es schon ein Konto? Dann melden Sie sich direkt an.",
    ],
    related: [
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentAccountProblem,
      HELP_TOPICS.parentInstallApp,
    ],
  };
}
