import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/anmeldephasen` (#4704): Anmeldephase mit Angeboten für mehrere Schulen. */
export function traegerCatalogPhasesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCatalogPhases,
    title: "Anmeldephasen für mehrere Schulen",
    question: "Wie lege ich eine Anmeldephase für mehrere Schulen an?",
    summary:
      "Sie legen eine Anmeldephase mit Betreuungsangeboten einmal an. Jede Schule bekommt davon eine eigene Kopie. Erst wenn Sie übertragen, ändert sich etwas in den Schulen.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarPlus",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Anmeldephase anlegen",
        steps: [
          "Wählen Sie in der Navigation `Anmeldephasen`.",
          "Wählen Sie `Anmeldephase anlegen`.",
          "Tragen Sie `Name`, `Art` und den `Leistungszeitraum` ein.",
          "Soll die Anmeldung nur in einer bestimmten Zeit möglich sein? Tragen Sie `Anmeldung ab` und `Anmeldung bis` ein. Sonst lassen Sie die Felder leer.",
          "Wählen Sie `Zielgruppe`, `Betreuungsauswahl` und `Verhalten bei voller Betreuung`.",
          "Wählen Sie bei `Schulen` `Alle Schulen (auch neue)` oder `Ausgewählte Schulen`. Bei ausgewählten Schulen setzen Sie bei jeder Schule einen Haken.",
          "Wählen Sie `Anmeldephase anlegen`.",
        ],
      },
      {
        title: "Betreuungsangebote anlegen",
        steps: [
          "Öffnen Sie die Anmeldephase.",
          "Wählen Sie bei `Betreuungsangebote` `Angebot anlegen`.",
          "Tragen Sie `Name` und die `Wochentage` ein. Zählt das Angebot als Betreuungstag, tragen Sie für jeden Tag eine Gehzeit ein.",
          "Tragen Sie die Preise ein. Wählen Sie `Je Wochentag` oder `Nach Tagen je Woche`. In der Spalte `Jedes weitere Kind` steht der Preis für Geschwister. Leer heißt: derselbe Preis wie beim 1. Kind.",
          "Soll die Schule auch die Vertragsbedingungen bekommen? Schalten Sie `Vertragsvorlage mitgeben` ein und füllen Sie die Felder aus.",
          "Wählen Sie `Angebot anlegen`.",
        ],
      },
      {
        title: "Vorschau ansehen und übertragen",
        steps: [
          "Öffnen Sie die Anmeldephase.",
          "Wählen Sie oben `Vorschau ansehen`.",
          "Prüfen Sie je Schule, was passiert: `Wird neu angelegt`, `Wird geändert`, `Wird zurückgezogen` oder `Lehnt ab` mit dem Grund.",
          "Wählen Sie `Freigeben und übertragen`.",
          "Unter `Ergebnis` sehen Sie, welche Schule übernommen hat.",
        ],
      },
    ],
    result:
      "Jede Schule hat eine eigene Anmeldephase mit den Angeboten. Auf der Seite der Anmeldephase steht bei `Schulen` das letzte Ergebnis jeder Schule.",
    notes: [
      "Die Schule darf ihre Kopie ändern oder ausschalten. Diese Änderungen bleiben. In der Vorschau steht dann `Die Schule hat geändert – bleibt so`, mit dem Wert der Schule und Ihrem Wert.",
      "Neue Preise gelten in der Schule ab dem nächsten Monatsbeginn. Ausgestellte Verträge und Rechnungen bleiben, wie sie sind.",
      "Eine Schule darf eine Kopie nicht löschen. Entfernen geht nur hier mit `Zurückziehen`. Danach übertragen Sie noch einmal.",
      "Hat ein Kind ein zurückgezogenes Angebot schon gebucht, schaltet moto es in der Schule nur aus.",
      "Sie ziehen das Ende einer Anmeldephase vor? Dann nennt die Vorschau je Schule, wie viele Kinder länger betreut werden. moto beendet keine Betreuung. Sagen Sie der Schule Bescheid.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Anmeldephasen`? Ihre Rolle darf keine Anmeldungen verwalten. Fragen Sie im Träger-Büro nach der Rolle Verwaltung.",
      "Eine zurückgezogene Anmeldephase lässt sich nicht mehr ändern. Das Formular fehlt dann.",
    ],
    troubleshootingDetails: [
      "Eine Schule `Lehnt ab`? Der Grund steht daneben. Passen Sie Ihre Angaben an oder bitten Sie die Schule um eine Änderung. Danach sehen Sie die Vorschau noch einmal an.",
      "Beim Übertragen steht, dass sich seit der Vorschau etwas geändert hat? moto lädt die Vorschau neu. Prüfen Sie sie und wählen Sie noch einmal `Freigeben und übertragen`.",
      "Bei einer Schule steht `Nicht übertragen`? Die Schule hat während der Vorschau etwas geändert. Übertragen Sie noch einmal.",
    ],
    related: [
      HELP_TOPICS.traegerLogin,
      HELP_TOPICS.traegerCareContracts,
      HELP_TOPICS.traegerCareMonths,
    ],
  };
}
