import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function invitationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.acceptInvitation,
    title: "Einladung annehmen und Konto einrichten",
    question: "Wie nehme ich meine Einladung an?",
    summary:
      "Öffnen Sie Ihre Einladungs-Mail und legen Sie Ihr persönliches Passwort fest.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "EnvelopeOpen",
    requirements: [
      "Ihre Einladungs-Mail von moto",
      "Zugriff auf das E-Mail-Postfach, an das die Einladung gesendet wurde",
    ],
    steps: [
      "Öffnen Sie die Einladungs-Mail von moto.",
      "Wählen Sie in der E-Mail `Einladung annehmen`.",
      "Prüfen Sie oben die Zeile `Einladung für ... als ...`. Stimmt die Rolle?",
      "Prüfen Sie unter `Gültig bis`, wie lange die Einladung gilt.",
      "Ergänzen Sie `Vorname` und `Nachname`, falls die Felder leer sind.",
      "Geben Sie unter `Passwort` ein persönliches Passwort ein.",
      "Geben Sie dasselbe Passwort unter `Passwort bestätigen` erneut ein.",
      "Prüfen Sie, ob alle Passwortanforderungen grün markiert sind.",
      "Wählen Sie `Einladung akzeptieren`.",
    ],
    result:
      "Ihr moto-Konto ist eingerichtet und Sie können sich jetzt anmelden. Bewahren Sie Ihre E-Mail-Adresse und Ihr Passwort sicher auf und geben Sie beides nicht weiter.",
    notes: [
      "Zur Einladung bekommen Sie eine zweite E-Mail: `Willkommen bei moto`. Sie führt zu dieser Anleitung. Den Knopf `Einladung annehmen` finden Sie nur in der Einladungs-Mail.",
    ],
    differences: [
      "Steht dort `Schule hinzufügen`? Dann haben Sie schon ein moto-Konto. Melden Sie sich an und wählen Sie `Einladung annehmen`. Ihr Passwort bleibt gleich.",
    ],
    troubleshootingDetails: [
      "Sie finden keine Einladungs-Mail? Prüfen Sie auch Ihren Spam-Ordner. Bitten Sie sonst Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist abgelaufen oder wurde schon verwendet? Bitten Sie Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist nicht für Sie oder die Rolle ist falsch? Nehmen Sie sie nicht an und wenden Sie sich an Ihre Leitung.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.installApp],
  };
}
