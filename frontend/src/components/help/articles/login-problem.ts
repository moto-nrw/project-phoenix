import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function loginProblemTopic(nfcEnabled: boolean | null): HelpTopic {
  return {
    id: HELP_TOPICS.loginProblem,
    title: "Ich kann mich nicht anmelden",
    question: "Was kann ich tun, wenn die Anmeldung nicht klappt?",
    summary:
      "Prüfen Sie die Seite Ihrer OGS, Ihre E-Mail-Adresse und Ihr Passwort.",
    group: "probleme",
    audience: ["caregiver", "lead"],
    icon: "KeyRound",
    steps: [
      "Prüfen Sie, ob Sie die moto-Seite Ihrer OGS geöffnet haben.",
      "Geben Sie Ihre E-Mail-Adresse erneut vollständig ein.",
      "Prüfen Sie Groß- und Kleinschreibung sowie versehentliche Leerzeichen im Passwort.",
      "Wählen Sie `Passwort vergessen?`, wenn Sie Ihr Passwort nicht mehr wissen.",
      "Öffnen Sie die E-Mail von moto und folgen Sie dem Link zum neuen Passwort.",
      "Fragt moto nach einem Sicherheitscode? Prüfen Sie Ihr E-Mail-Postfach und den Spam-Ordner.",
      "Haben Sie mehrfach ein falsches Passwort eingegeben? Warten Sie 15 Minuten. Versuchen Sie es danach erneut.",
      "Ein Passwortwechsel verkürzt diese Wartezeit nicht.",
      "Versuchen Sie die Anmeldung mit dem neuen Passwort erneut.",
      "Klappt die Anmeldung weiterhin nicht? Wenden Sie sich an Ihre Leitung.",
      "Bitten Sie um eine Prüfung von Konto und Einladung.",
    ],
    result:
      "Mit einem aktiven Konto und den richtigen Zugangsdaten gelangen Sie wieder in moto.",
    differences: [
      "Eine abgelaufene oder bereits verwendete Einladung kann nicht noch einmal angenommen werden.",
      // Ohne NFC gibt es kein Tablet und keine Geraete-PIN. Der Hinweis
      // schickt dann auf die Suche nach etwas, das es nicht gibt.
      ...(nfcEnabled === false
        ? []
        : [
            "Die vierstellige Geräte-PIN des NFC-Tablets ist nicht Ihr persönliches moto-Passwort.",
          ]),
      "Ein Sicherheitscode kann nur für kurze Zeit verwendet werden. Fordern Sie bei Bedarf einen neuen Code an.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.acceptInvitation],
  };
}
