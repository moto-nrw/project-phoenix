import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Bereich `Angaben zum Kind` (i18n `parentMasterData`). Drei Arten von
 * Feldern: direkt aenderbar, nur auf Anfrage, und Gesundheitshinweise, die
 * sofort gespeichert werden.
 */
export function parentChildDataTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildData,
    title: "Angaben zu meinem Kind ansehen und ändern",
    question: "Wie ändere ich die Angaben zu meinem Kind?",
    summary: "Manches ändern Sie selbst. Anderes prüft die OGS zuerst.",
    group: "mein-kind",
    audience: "parent",
    icon: "FileText",
    steps: [],
    instructionGroups: [
      {
        title: "Die Angaben öffnen",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Angaben`.",
        ],
      },
      {
        title: "Name, Geburtsdatum oder Klasse ändern",
        steps: [
          "Gehen Sie zum Abschnitt `Persönliche Angaben`.",
          "Tippen Sie auf `Änderungen anfragen`.",
          "Ändern Sie nur die Angaben, die nicht mehr stimmen.",
          "Tippen Sie auf `Anfrage an OGS senden`.",
        ],
      },
      {
        title: "Gesundheitshinweise eintragen",
        steps: [
          "Gehen Sie zum Abschnitt `Gesundheit`.",
          "Tragen Sie bei `Gesundheitshinweise und Allergien` ein, was wichtig ist.",
          "Die Eingabe wird automatisch gespeichert.",
        ],
      },
    ],
    result:
      "Unter dem Feld steht dann `Beantragte Änderung` mit dem neuen Wert und `In Prüfung`. Bis zur Bestätigung gilt der Wert darüber.",
    notes: [
      "Gesundheitshinweise gehen sofort an das Team, ohne Prüfung.",
      "Sie möchten eine offene Anfrage ändern? Tippen Sie unter `Beantragte Änderung` auf `Anfrage bearbeiten`.",
    ],
    differences: [
      "Unter einem Feld steht schon `Beantragte Änderung`? Dann wartet diese Änderung noch auf die OGS.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Bearbeiten sei deaktiviert? Dann nimmt Ihre OGS keine Änderungen über die App an. Schreiben Sie ihr eine Nachricht.",
    ],
    related: [
      HELP_TOPICS.parentGuardians,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentFeatureMissing,
    ],
  };
}
