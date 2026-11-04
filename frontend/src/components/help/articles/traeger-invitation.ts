import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/invite` (#4650): Einladung aus der Mail annehmen. */
export function traegerInvitationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerInvitation,
    title: "Einladung ins Träger-Büro annehmen",
    question: "Wie nehme ich eine Einladung ins Träger-Büro an?",
    summary:
      "Sie haben eine E-Mail mit einer Einladung bekommen. So kommen Sie ins Träger-Portal.",
    group: "einstieg",
    audience: "traeger",
    icon: "UserRoundPlus",
    requirements: [
      "Das moto-Team oder jemand aus dem Träger-Büro hat Sie per E-Mail eingeladen.",
    ],
    steps: [
      "Öffnen Sie die E-Mail `Einladung ins Träger-Portal von moto`.",
      "Wählen Sie `Einladung annehmen`.",
      "Haben Sie noch kein moto-Konto? Tragen Sie ein `Neues Passwort` ein und wiederholen Sie es.",
      "Haben Sie schon ein moto-Konto? Tragen Sie das Passwort dieses Kontos ein.",
      "Wählen Sie `Einladung annehmen`.",
      "moto öffnet die Anmeldung des Träger-Portals. Melden Sie sich mit Ihrer E-Mail-Adresse und dem Passwort an.",
      "Beim ersten Anmelden richten Sie den Code ein. Wählen Sie `Code an meine E-Mail senden`.",
      "Öffnen Sie die E-Mail von moto. Geben Sie den sechsstelligen Code ein.",
      "Wählen Sie `Weiter zum Dashboard`.",
    ],
    result:
      "moto öffnet die Seite `Schulen`. Was Sie sehen, hängt von Ihrer Rolle ab.",
    notes: [
      "Mit derselben E-Mail-Adresse und demselben Passwort melden Sie sich später an.",
      "Ein Konto in einer Schule brauchen Sie dafür nicht.",
    ],
    troubleshootingDetails: [
      "moto sagt, die Einladung sei abgelaufen? Bitten Sie das Träger-Büro oder das moto-Team um eine neue Einladung.",
      "Sie kennen das Passwort Ihres vorhandenen moto-Kontos nicht mehr? Wählen Sie in der Anmeldung des Träger-Portals `Passwort vergessen?`.",
    ],
    related: [HELP_TOPICS.traegerLogin],
  };
}
