import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Reiter `Kontakte`. Live geprueft: eine Karte `Abholberechtigte und
 * Kontakte` mit dem Knopf `Kontakt oder Abholperson hinzufügen`, darunter
 * die Karte `Verbundene Konten` mit `Einladen`. Ein Zwischenschritt
 * `Person hinzufügen` erscheint nicht. Die Schalter `Darf abholen` und
 * `Notfallkontakt` stehen schon im Anlegen-Fenster; spaeter aendern darf sie
 * nur, wer `can_manage_pickup` hat -- sonst heisst das Fenster nur
 * `Hinweis zur Abholung` (guardians-panel.tsx).
 */
export function parentGuardiansTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentGuardians,
    title: "Kontakte und Abholung verwalten",
    question: "Wer darf mein Kind abholen und wer wird erreicht?",
    summary: "Sie hinterlegen Kontakte und legen Abholrechte fest.",
    group: "mein-kind",
    audience: "parent",
    icon: "Users",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Kontakt hinzufügen",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Kontakte`.",
          "Tippen Sie auf `Kontakt oder Abholperson hinzufügen`.",
          "Wählen Sie im Fenster `Kontakt hinzufügen` die `Beziehung zum Kind`.",
          "Schalten Sie bei Bedarf `Darf abholen` und `Notfallkontakt` ein.",
          "Tragen Sie `Vorname` und `Nachname` ein.",
          "Mit `Darf abholen` wählen Sie bei `An welchen Tagen?` alle Tage oder einzelne Tage.",
          "Unterschreiben Sie die Erlaubnis im Feld und tippen Sie auf `Unterschrift bestätigen`.",
          "Tippen Sie unten auf `Kontakt hinzufügen`.",
        ],
      },
      {
        title: "Abholrecht später ändern",
        description:
          "Bei jeder Person stehen rechts zwei Symbole: der Stift für die Kontaktdaten, das zweite für die Abholung.",
        steps: [
          "Tippen Sie bei der Person auf das zweite Symbol `Abholrecht verwalten`.",
          "Schalten Sie im Fenster `Abholung verwalten` `Darf abholen` ein oder aus.",
          "Wählen Sie bei `An welchen Tagen?` `An allen Tagen` oder `Nur an bestimmten Tagen` und dann die Tage.",
          "Erteilen oder ändern Sie das Abholrecht, unterschreiben Sie im Feld. Tippen Sie auf `Unterschrift bestätigen`.",
          "Schalten Sie bei Bedarf `Notfallkontakt` ein.",
          "Tragen Sie bei Bedarf einen `Hinweis zur Abholung` ein.",
          "Tippen Sie auf `Speichern`.",
        ],
      },
      {
        title: "Einer Person App-Zugang geben",
        steps: [
          "Gehen Sie zur Karte `Verbundene Konten`.",
          "Tippen Sie auf `Einladen`.",
          "Tragen Sie die E-Mail-Adresse der Person ein.",
          "Tippen Sie auf `Einladung senden`.",
        ],
      },
    ],
    result:
      "Die OGS sieht den Kontakt sofort. Ein neues oder geändertes Abholrecht gilt erst, wenn die OGS es bestätigt hat. Bis dahin steht an der Person `Wartet auf die OGS`.",
    notes: [
      "An einer Person stehen `Primär`, `Darf abholen` oder `Notfallkontakt`. Gilt das Abholrecht nur an manchen Tagen, steht dort zum Beispiel `Nur Di, Do`.",
      "Sie unterschreiben als einfache elektronische Signatur mit gezeichneter Unterschrift. Statt zu zeichnen, können Sie mit `Namen tippen` Ihren Namen eintragen.",
      "Das Abholrecht wegnehmen geht sofort und ohne Unterschrift.",
      "Holt jemand nur an einem einzigen Tag ab? Dann ändern Sie die Abholung für diesen Tag.",
      "Ein Kontakt braucht kein eigenes Konto. Zum Beispiel Oma oder Opa.",
      "Eine E-Mail beim Kontakt lädt niemanden ein. Der App-Zugang ist ein eigener Schritt.",
    ],
    differences: [
      "Unter `Verbundene Konten` sehen Sie, wer die Eltern-App für Ihr Kind nutzt.",
    ],
    troubleshootingDetails: [
      "moto sagt, die Person verwalte ihre Daten über ein eigenes Konto? Dann ändert sie ihre Angaben selbst.",
      "moto sagt, die Kontaktdaten würden von der Schule verwaltet? Dann wenden Sie sich an die OGS.",
      "Die Verwaltung ist deaktiviert? Dann nimmt Ihre OGS das nicht über die App an.",
      "`Einladen` fehlt? Dann dürfen Sie für dieses Kind niemanden einladen. Fragen Sie die OGS.",
      "`Darf abholen` fehlt bei einer Person mit eigenem Konto? Ein Elternteil mit eigenem Konto regelt die OGS. Fragen Sie die OGS.",
    ],
    related: [
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}
