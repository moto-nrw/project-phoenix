import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Anmeldung vorbereiten: Phase, Angebote und Formular. Die drei Seiten liegen
 * im Akkordeon `Anmeldungen` der Gruppe `Eltern`; die Uebersicht selbst
 * (`/admin/enrollments`) rendert ihren Inhalt erst ab `lg` (page.tsx:44).
 */
export function enrollmentSetupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentSetup,
    title: "Eine Anmeldung vorbereiten",
    question: "Wie öffne ich die Anmeldung für das nächste Schuljahr?",
    summary: "Drei Schritte: Phase anlegen, Angebote pflegen, Formular prüfen.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "CalendarPlus",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Anmeldephase anlegen",
        description:
          "Die Phase kommt zuerst. Betreuungsangebote gehören immer zu einer Phase, und ohne sie bleibt die Seite `Betreuungsangebote` leer.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Anmeldungen` und danach `Anmeldephasen`.",
          "Wählen Sie `Neue Anmeldephase`.",
          "Tragen Sie bei `Name` ein, worum es geht, zum Beispiel `Schuljahr 2026/27`.",
          "Wählen Sie den `Typ`.",
          "Tragen Sie `Beginn` und Ende der Betreuung ein.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Angebote und Formular vorbereiten",
        description:
          "Beides gehört zu der Phase, die Sie gerade angelegt haben. Wählen Sie sie oben auf der Seite aus, falls Sie mehrere haben.",
        steps: [
          "Öffnen Sie `Betreuungsangebote` und tragen Sie ein, was buchbar sein soll.",
          "Öffnen Sie `Anmeldeformulare`. Dort steht schon ein `Basisformular`. Für die meisten OGS reicht es.",
          "Sehen Sie es sich mit `Vorschau` an.",
          "Öffnen Sie `Überblick` und wählen Sie `Formular ansehen`.",
        ],
      },
      {
        title: "Die Anmeldung öffnen",
        steps: [
          "Öffnen Sie `Anmeldephasen`.",
          "Öffnen Sie bei der Phase das Menü mit den drei Punkten.",
          "Wählen Sie `Aktivieren`.",
          "Kopieren Sie den Link der Phase und geben Sie ihn an die Eltern.",
        ],
      },
    ],
    result:
      "Eltern können sich über den Link anmelden. Eingänge sehen Sie im `Überblick`.",
    notes: [
      "Über `Elternansicht öffnen` sehen Sie das Formular so wie die Eltern.",
      "Das `Basisformular` fragt Elternteil, Kind, Klassenstufe und das gewünschte Betreuungsangebot ab. Brauchen Sie mehr, legen Sie eine eigene Vorlage an.",
      "Eine Phase lässt sich mit einem Kalenderzeitraum verknüpfen. Dann passen die Daten automatisch.",
      "Bei `Betreuungsangebote` steht in der Spalte `Belegt`, wie viele Plätze gebucht sind. Bei Angeboten mit Kapazität steht auch, wie viele Plätze frei sind. Ein volles Angebot ist mit `Voll` markiert.",
      "Sie haben Varianten wie „bis 13 Uhr“ und „bis 17 Uhr“? Geben Sie ihnen bei `Pflichtauswahl` dieselbe `Gruppe (optional)` und die `Regel` `Höchstens eines pro Tag`. Dann mischen Eltern die Varianten über die Woche, aber wählen pro Tag nur eine.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Die Anmeldungen laufen am Computer.",
      "Eine Phase trägt `Aktiv` oder nichts. Nur eine aktive Phase nimmt Anmeldungen an.",
    ],
    troubleshootingDetails: [
      "Der Überblick sagt `Noch keine Anmeldephase`? Dann legen Sie zuerst eine Phase an.",
      "Auf `Betreuungsangebote` steht `Erst eine Anmeldephase anlegen`? Dann fehlt die Phase noch. Angebote gibt es nur innerhalb einer Phase.",
      "Oben steht `Anschlussphase fehlt`? Dann laufen die Buchungen aus. Wählen Sie `Anschlussphase erstellen`.",
      "Fehlt `Anmeldungen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentForm,
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadEnrollmentExport,
      HELP_TOPICS.leadCalendarPeriods,
    ],
  };
}
