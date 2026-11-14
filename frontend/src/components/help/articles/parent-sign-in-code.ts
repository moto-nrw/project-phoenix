import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/settings?tab=signIn` (#7051). Die Anmeldung mit Code ist
 * freiwillig und gilt für das ganze Konto. Ein verlorenes Postfach setzt das
 * moto-Team über eine Schule des Kontos zurück; ein Konto ohne Schule kann
 * sie deshalb nicht einschalten.
 */
export function parentSignInCodeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentSignInCode,
    title: "Anmeldung mit Code",
    question: "Wie schütze ich mein Konto mit einem Code?",
    summary:
      "Nach dem Passwort geben Sie einen Code aus Ihrer E-Mail ein. Das ist freiwillig.",
    group: "einstieg",
    audience: "parent",
    icon: "ShieldCheck",
    steps: [
      "Tippen Sie unten auf `Mehr` und dann auf `Einstellungen`.",
      "Wählen Sie den Bereich `Anmeldung`.",
      "Wählen Sie bei `Anmeldung mit Code` den Knopf `Einschalten`.",
      "moto schickt Ihnen einen Code per E-Mail. Tragen Sie ihn bei `Code aus der E-Mail` ein.",
      "Wählen Sie `Einschalten`.",
    ],
    result:
      "Bei jeder Anmeldung fragt moto jetzt nach dem Passwort auch nach einem Code aus Ihrer E-Mail.",
    notes: [
      "Nach dem Code fragt moto: `Soll moto dieses Gerät merken?` Auf Ihrem eigenen Gerät wählen Sie `Ja, merken und anmelden`. Dann fragt moto dort eine Zeit lang nicht nach dem Code.",
      "Auf Geräten, die mehrere Personen nutzen, wählen Sie `Nein, nur anmelden`.",
      "Gemerkte Geräte sehen und entfernen Sie unter `Einstellungen` im Bereich `Anmeldung`.",
      "Die Anmeldung mit Code gilt für Ihr ganzes moto-Konto. Arbeiten Sie auch an einer Schule mit moto, fragt moto dort ebenfalls nach dem Code.",
    ],
    differences: [
      "Am Computer steht `Einstellungen` in der Navigation statt hinter `Mehr`.",
      "Statt `Einschalten` steht dort ein Hinweis? Dann gehört Ihr Konto noch zu keiner Schule. Sobald Ihr Kind an einer Schule angemeldet ist, können Sie die Anmeldung mit Code einschalten.",
      "Zum Ausschalten wählen Sie `Ausschalten` und geben Ihr Passwort ein.",
    ],
    troubleshootingDetails: [
      "Kein Code angekommen? Sehen Sie im Spam-Ordner nach. Oder wählen Sie `Neuen Code anfordern`.",
      "Kein Zugriff mehr auf Ihr E-Mail-Postfach? Wenden Sie sich an Ihre Schule. Das moto-Team setzt die Anmeldung mit Code dann zurück. Danach reicht wieder Ihr Passwort.",
    ],
    related: [HELP_TOPICS.parentLogin, HELP_TOPICS.parentAccount],
  };
}
