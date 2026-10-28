import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Problemartikel: Einladung laesst sich nicht annehmen. */
export function parentAccountProblemTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentAccountProblem,
    title: "Ich kann mein Konto nicht einrichten",
    question: "Was tun, wenn die Einladung nicht funktioniert?",
    summary: "Meist ist der Link abgelaufen oder schon benutzt.",
    group: "probleme",
    audience: "parent",
    icon: "ShieldAlert",
    steps: [
      "Lesen Sie, was auf der Seite steht.",
      "Prüfen Sie, ob Sie den Link schon einmal geöffnet haben.",
      "Versuchen Sie, sich direkt anzumelden.",
      "Klappt das nicht, bitten Sie Ihre OGS um eine neue Einladung.",
    ],
    result: "Mit einer neuen Einladung können Sie Ihr Passwort festlegen.",
    notes: ["Ein Einladungslink lässt sich nur einmal benutzen."],
    differences: [
      "`Diese Einladung ist abgelaufen oder wurde bereits verwendet.` Fragen Sie nach einer neuen.",
      "`Für diese E-Mail existiert bereits ein Konto.` Melden Sie sich direkt an.",
      "`Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.` Prüfen Sie die Liste unter dem Feld.",
    ],
    troubleshootingDetails: [
      "Sie haben ein Konto, aber kein Passwort mehr? Nutzen Sie auf der Anmeldeseite `Passwort vergessen?`.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}
