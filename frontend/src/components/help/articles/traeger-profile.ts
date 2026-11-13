import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/konto` (#4693): der eigene Name im Träger-Büro. */
export function traegerProfileTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerProfile,
    title: "Eigenen Namen ändern",
    question: "Wie ändere ich meinen Namen im Träger-Portal?",
    summary:
      "Ihr Name steht im Exportprotokoll und im Verlauf jeder Rechnung. So sehen andere im Träger-Büro, wer etwas getan hat.",
    group: "einstieg",
    audience: "traeger",
    icon: "UserCircle",
    steps: [
      "Öffnen Sie oben rechts das Profil-Menü.",
      "Wählen Sie `Konto`.",
      "Tragen Sie bei `Ihr Name` Ihren Namen ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Oben rechts steht sofort Ihr Name. Neue Einträge im Exportprotokoll und im Verlauf der Rechnungen zeigen ihn auch. Sie selbst sehen dort „von Ihnen“.",
    notes: [
      "Im Protokoll und im Verlauf steht Ihre E-Mail-Adresse nie. Sie sehen sie im Profil-Menü unter Ihrem Namen und unter `Konto` bei `Anmeldung`.",
      "Ihren Namen geben Sie schon beim Annehmen der Einladung an.",
      "Wer die Rolle `Leitung` hat, kann Namen auch unter `Mitglieder und Rollen` eintragen.",
    ],
    troubleshootingDetails: [
      "Oben rechts steht Ihre E-Mail-Adresse statt eines Namens? Sie haben noch keinen Namen. Tragen Sie ihn unter `Konto` ein.",
      "Im Protokoll steht „von einem Konto ohne Namen“? Diese Person hat noch keinen Namen. Sie kann ihn unter `Konto` eintragen.",
      "Im Protokoll steht „von einem gelöschten Konto“? Das Konto gibt es nicht mehr. Der Name lässt sich nicht mehr zeigen.",
    ],
    related: [HELP_TOPICS.traegerInvitation, HELP_TOPICS.traegerAccounts],
  };
}
