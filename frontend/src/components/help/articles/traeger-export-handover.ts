import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerExportHandoverTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerExportHandover,
    title: "Exporte an einen Empfänger übergeben",
    question: "Wie schickt moto die Dateien direkt an die Kämmerei?",
    summary:
      "moto kann die Sollstellung, die DATEV-Datei und die Lastschriftdatei selbst bei einem Empfänger ablegen, zum Beispiel bei Ihrer Kämmerei. Sie tragen den Empfänger einmal ein. Herunterladen können Sie die Dateien weiterhin.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "FolderOpen",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Der Empfänger hat Ihnen Adresse, Benutzername, Passwort, Zielordner und Fingerabdruck genannt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Empfänger eintragen",
        description: "Das machen Sie nur einmal.",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie oben rechts `Empfänger der Exporte`.",
          "Schalten Sie `Übergabe einschalten` ein.",
          "Tragen Sie `Adresse`, `Port`, `Benutzername`, `Passwort`, `Zielordner` und `Fingerabdruck` ein.",
          "Wählen Sie `Speichern`. Oben steht dann `Der Empfänger ist bereit`.",
        ],
      },
      {
        title: "Datei übergeben",
        steps: [
          "Wählen Sie `Abrechnung`, dann den Reiter `Exporte`.",
          "Wählen Sie beim Lauf `Übergeben`, dann `Sollstellung übergeben` oder `DATEV-Datei übergeben`.",
          "Kommt die Datei an, zeigt moto eine kurze Meldung mit dem Dateinamen.",
          "Die Lastschriftdatei übergeben Sie bei der Lastschrift des Laufs mit `Übergeben`.",
        ],
      },
    ],
    result:
      "Die Datei steht im `Exportprotokoll` mit dem Vermerk `An Empfänger übergeben`. Unter `Letzte Übergaben` auf der Seite des Empfängers steht jeder Versuch mit seinem Ergebnis.",
    notes: [
      "Jede Übergabe ist eine neue Datei mit neuer Nummer, wie ein Download.",
      "Ist dieselbe Datei schon beim Empfänger, fragt moto vorher nach. So bucht die Kämmerei nichts doppelt.",
      "Das Passwort speichert moto verschlüsselt. Es wird nie wieder angezeigt. Ändern Sie Adresse, Port, Benutzername oder Fingerabdruck, geben Sie es noch einmal ein.",
      "Mit `Empfänger entfernen` löschen Sie Adresse und Passwort. Der Verlauf der Übergaben bleibt.",
    ],
    differences: [
      "Beim Lauf fehlt `Übergeben`? Ihnen fehlt das Recht `Finanz-Export`, oder moto konnte den Empfänger nicht lesen. Laden Sie die Seite neu.",
      "Im Menü steht nur `Empfänger eintragen`? Es fehlen noch Angaben, oder die Übergabe ist ausgeschaltet.",
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
