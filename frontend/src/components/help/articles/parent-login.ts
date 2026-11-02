import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/login`. Eigenes Portal mit eigener Adresse: ein Personalkonto
 * wird hier abgewiesen (i18n `parentLogin.errors.notAGuardian`).
 */
export function parentLoginTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentLogin,
    title: "Bei moto anmelden",
    question: "Wie melde ich mich im Elternportal an?",
    summary:
      "Das Elternportal hat eine eigene Adresse und eigene Zugangsdaten.",
    group: "einstieg",
    audience: "parent",
    icon: "LogIn",
    steps: [
      "Öffnen Sie die Adresse des Elternportals.",
      "Tragen Sie Ihre `E-Mail-Adresse` ein.",
      "Tragen Sie Ihr `Passwort` ein.",
      "Wählen Sie `Anmelden`.",
    ],
    result: "moto öffnet die Startseite mit dem heutigen Tag Ihres Kindes.",
    notes: [
      "Mit dem Augensymbol sehen Sie Ihr Passwort im Klartext.",
      "Legen Sie sich die Adresse als Lesezeichen an. Oder fügen Sie moto zum Startbildschirm hinzu.",
    ],
    differences: [
      "Passwort vergessen? Wählen Sie `Passwort vergessen?`. Es öffnet sich das Fenster `Passwort zurücksetzen`. Tragen Sie Ihre E-Mail-Adresse ein und wählen Sie `Link senden`. Den Link bekommen Sie per E-Mail.",
    ],
    troubleshootingDetails: [
      "Nach mehreren falschen Passwörtern warten Sie bitte 15 Minuten.",
      "Ein Passwortwechsel verkürzt diese Wartezeit nicht.",
      "moto sagt, das Konto gehöre zum Personal einer Schule? Dann melden Sie sich über die Schul-Anmeldung an.",
      "moto sagt, Ihr Konto sei deaktiviert? Wenden Sie sich an Ihre OGS.",
      "moto sagt, Ihre Schule nutze das Elternportal zurzeit nicht? Dann bietet Ihre Schule das Elternportal nicht an. Fragen Sie bei Ihrer Schule nach.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentAccountProblem,
    ],
  };
}
