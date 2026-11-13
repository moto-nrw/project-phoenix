import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerExportHandoverTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerExportHandover,
    title: "Exporte an Empfänger übergeben",
    question:
      "Wie schickt moto die Dateien direkt an Kämmerei oder Steuerbüro?",
    summary:
      "moto kann die Sollstellung, die DATEV-Datei und die Lastschriftdatei selbst beim Empfänger ablegen. Jede Datei geht an genau einen Empfänger, zum Beispiel die Sollstellung an die Kämmerei und die DATEV-Datei an das Steuerbüro. Herunterladen können Sie die Dateien weiterhin.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "FolderOpen",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Jeder Empfänger hat Ihnen Adresse, Benutzername, Passwort, Zielordner und Fingerabdruck genannt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Empfänger eintragen",
        description: "Das machen Sie einmal je Empfänger.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie oben rechts `Empfänger der Exporte`, dann `Empfänger hinzufügen`.",
          "Geben Sie einen `Name` ein, zum Beispiel „Kämmerei“.",
          "Tragen Sie `Adresse`, `Port`, `Benutzername`, `Passwort`, `Zielordner` und `Fingerabdruck` ein.",
          "Wählen Sie `Empfänger anlegen`. In der Liste `Empfänger` steht er dann mit `Bereit`.",
        ],
      },
      {
        title: "Dateien zuordnen",
        description: "So legen Sie fest, welche Datei an wen geht.",
        steps: [
          "Öffnen Sie `Empfänger der Exporte`.",
          "Wählen Sie unter `Welche Datei geht wohin` bei jeder Datei einen Empfänger. `Nur herunterladen` heißt: moto übergibt diese Datei nicht.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Datei übergeben",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie beim Lauf `Übergeben`, dann `Sollstellung übergeben` oder `DATEV-Datei übergeben`. Im Menü steht, an welchen Empfänger die Datei geht.",
          "Kommt die Datei an, zeigt moto eine kurze Meldung mit dem Dateinamen und dem Empfänger.",
          "Die Lastschriftdatei übergeben Sie bei der Lastschrift des Laufs mit `Übergeben`.",
        ],
      },
    ],
    result:
      "Die Datei steht im `Exportprotokoll` mit dem Vermerk `An Kämmerei übergeben` (mit dem Namen Ihres Empfängers). Unter `Letzte Übergaben` steht jeder Versuch mit Empfänger und Ergebnis. Ging die letzte Übergabe schief, zeigt die Liste `Empfänger` das beim Empfänger an.",
    notes: [
      "Jede Übergabe ist eine neue Datei mit neuer Nummer, wie ein Download.",
      "Ist dieselbe Datei schon bei diesem Empfänger, fragt moto vorher nach. So bucht die Kämmerei nichts doppelt. Ein anderer Empfänger hatte die Datei noch nicht, dann fragt moto nicht.",
      "Hat der Empfänger eine DATEV-Datei verloren, wählen Sie beim Lauf `Übergeben`, dann `Alle DATEV-Buchungen erneut übergeben`. Kam eine DATEV-Datei nicht an, enthält die nächste ihre Buchungen wieder.",
      "Das Passwort speichert moto verschlüsselt. Es wird nie wieder angezeigt. Ändern Sie Adresse, Port, Benutzername oder Fingerabdruck, geben Sie es noch einmal ein.",
      "Mit `Empfänger entfernen` auf der Seite des Empfängers löschen Sie Adresse und Passwort. Seine Dateien können Sie danach nur herunterladen, bis Sie sie einem anderen Empfänger zuordnen. Der Verlauf der Übergaben bleibt.",
    ],
    differences: [
      "Beim Lauf fehlt `Übergeben`? Ihnen fehlt das Recht `Finanz-Export`, oder moto konnte den Empfänger nicht lesen. Laden Sie die Seite neu. Die Lastschriftdatei übergeben Sie auch ohne dieses Recht.",
      "Im Menü steht `Empfänger einrichten` oder `Empfänger zuordnen`? Diese Datei hat noch keinen Empfänger, oder beim Empfänger fehlen Angaben oder die Übergabe ist ausgeschaltet. Das Menü nennt den Grund.",
    ],
    troubleshootingDetails: [
      "`Nicht angekommen` mit dem Hinweis auf den Fingerabdruck? Fragen Sie beim Empfänger nach dem Fingerabdruck für den RSA-Schlüssel.",
      "`Die Anmeldung wurde abgelehnt`? Prüfen Sie Benutzername und Passwort.",
      "`Ergebnis unklar`? Fragen Sie beim Empfänger nach, ob die Datei da ist. Erst dann übergeben Sie sie noch einmal.",
      "`Diese Adresse ist nicht erlaubt`? moto übergibt nur an Adressen im Internet, nicht ins eigene Netz.",
    ],
    related: [
      HELP_TOPICS.traegerFinanceExports,
      HELP_TOPICS.traegerDatevExport,
      HELP_TOPICS.traegerSepaDebitFile,
    ],
  };
}
