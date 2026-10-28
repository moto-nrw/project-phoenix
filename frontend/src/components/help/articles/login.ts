import type { HelpTopic, HelpPresenceMode, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { loginResult } from "../help-article-shared";

export function loginTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  return {
    id: HELP_TOPICS.login,
    title: "Bei moto anmelden",
    question: "Wie melde ich mich bei moto an?",
    summary:
      "Melden Sie sich über die moto-Seite Ihrer OGS mit Ihrem persönlichen Konto an.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "UserCircle",
    requirements: [
      "Den Link zur moto-Seite Ihrer OGS",
      "Ein moto-Konto. Nehmen Sie vorher Ihre Einladung an.",
      "Ihre E-Mail-Adresse und Ihr Passwort für moto",
      "Zugriff auf Ihr E-Mail-Postfach, falls moto einen Sicherheitscode abfragt",
    ],
    steps: [
      "Öffnen Sie die moto-Seite Ihrer OGS.",
      "Geben Sie unter `E-Mail-Adresse` Ihre persönliche E-Mail-Adresse ein.",
      "Geben Sie unter `Passwort` Ihr Passwort ein.",
      "Wählen Sie `Anmelden`.",
      "Zeigt moto die `Code-Eingabe`? Öffnen Sie die E-Mail von moto.",
      "Geben Sie den sechsstelligen Code ein. moto prüft ihn automatisch.",
    ],
    result: loginResult(presenceMode, groupMode),
    differences: [
      "Für Verwaltung ist ein zweiter Nachweis immer Pflicht. Ebenso für Personalstammdaten, Personaldokumente und Personalfinanzen. Das gilt auch für Einkommensnachweise der Eltern. Ebenso für Gesundheitsdokumente und das Verwalten fremder Arbeitszeiten. Ebenso, wenn Sie Urlaub genehmigen dürfen. Diese Pflicht gilt auch bei ausgeschalteter Sicherheitsabfrage der Schule. Gesundheitshinweise der Kinder allein lösen diese Pflicht nicht aus.",
      "Richten Sie die Sicherheitsabfrage erstmals ein? Wählen Sie `Code an meine E-Mail senden`.",
      "Geben Sie danach den sechsstelligen Code ein.",
      "Wenn Sie bereits einen Passkey eingerichtet haben, können Sie `Mit Passkey anmelden` wählen.",
    ],
    notes: [
      "Nutzen mehrere Personen das Gerät? Melden Sie sich bei moto ab, wenn Sie fertig sind.",
    ],
    troubleshootingDetails: [
      "Kein Zugriff auf Ihre E-Mails? Wenden Sie sich an Ihre Schulverwaltung. Der zusätzliche Nachweis bleibt bei besonderen Zugriffsrechten Pflicht.",
      "Beim Schulwechsel fehlt ein zweiter Anmeldenachweis? Wählen Sie `Zur Schulanmeldung`. Sie werden zuerst abgemeldet. Melden Sie sich danach direkt bei der Zielschule neu an. Bestätigen Sie dort die Anmeldung.",
    ],
    troubleshooting: HELP_TOPICS.loginProblem,
    related: [
      HELP_TOPICS.acceptInvitation,
      HELP_TOPICS.installApp,
      HELP_TOPICS.loginProblem,
    ],
  };
}
