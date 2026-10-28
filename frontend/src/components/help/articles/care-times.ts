import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Betreuungszeiten. Drei Wege, alle an keine der drei Einstellungen gebunden:
 * beim Anlegen (`CareWeeklyPlanModal` im Slide-over `Neues Kind`), beim
 * einzelnen Kind (`CareScheduleManager`, Knopf `Wochenplan`) und fuer mehrere
 * Kinder ueber `Auswählen` -> `Ankunftszeiten` / `Gehzeiten`
 * (students-master-detail.tsx:532).
 */
export function careTimesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCareTimes,
    title: "Betreuungszeiten eintragen",
    question: "Wie trage ich die Betreuungszeiten eines Kindes ein?",
    summary: "Legen Sie fest, an welchen Tagen ein Kind betreut wird.",
    group: "einrichten",
    audience: "lead",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Den Wochenplan eines Kindes eintragen",
        steps: [
          "Öffnen Sie das Kind und wählen Sie den Bereich `Betreuungszeiten`.",
          "Wählen Sie `Wochenplan`.",
          "Wählen Sie die Betreuungstage.",
          "Tragen Sie `Ankunft` und `Abholung` ein.",
          "Kommt das Kind an einem Betreuungstag nicht? Lassen Sie `Abholung` leer.",
          "Öffnen Sie `Notizen`. Tragen Sie den Hinweis unter `Notiz zum Tag (jede Woche)` ein.",
          "Wählen Sie `Wochenplan speichern`.",
        ],
      },
      {
        title: "Zeiten für mehrere Kinder auf einmal setzen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie oben rechts `Auswählen`.",
          "Haken Sie die Kinder an.",
          "Wählen Sie `Ankunftszeiten` oder `Gehzeiten`.",
          "Tragen Sie die Zeiten ein und wählen Sie `Speichern`.",
          "Wählen Sie `Fertig`.",
        ],
      },
      {
        title: "Einen einzelnen Tag ändern",
        steps: [
          "Öffnen Sie beim Kind den Bereich `Betreuungszeiten`.",
          "Wählen Sie beim gewünschten Tag `Ausnahme`.",
          "Ändern Sie `Ankunft`, `Abholung` oder `Abholart`.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      // #3946: student-card.tsx (dayHomeChange) zeigt unten einen orangen Streifen.
      "Die Zeiten gelten sofort. Eine Notiz ohne Abholzeit steht auf der Kinderkarte unter `Kommt heute nicht`. Eine Ausnahme ändert den Wochenplan nicht. Gilt heute eine andere Gehzeit oder Abholart, steht unten auf der Kinderkarte ein oranger Hinweis, zum Beispiel `Abholart heute geändert: Wird abgeholt`.",
    notes: [
      "Ohne eigene Zeit gilt die Klassenzeit des Kindes.",
      // #3373: Ankunft nicht vor Abholung (comesOnlyIfLessonCancelled in
      // student-time-status.ts) warnt nicht mehr als überfällig.
      "Endet der Unterricht erst zur Abholzeit? Dann steht auf der Kinderkarte `Nur bei Unterrichtsausfall`. Das Kind gilt nicht als verspätet. Kommt es doch, checken Sie es wie gewohnt ein.",
      "Den Wochenplan können Sie schon beim Anlegen des Kindes mitgeben.",
      // #3372: Die Auswahl erscheint nur mit gepflegten Schulstunden
      // (school-period-select.tsx) und kopiert die Uhrzeit ins Zeitfeld.
      "Schneller geht es mit `Nach Schulstunde` unter dem Feld `Ankunft`: Wählen Sie zum Beispiel `5. Stunde`. moto trägt die passende Uhrzeit ein.",
      "Dieselbe Auswahl gibt es für eine ganze Klasse: unter `Kinderdaten` im Menü der Klasse bei `Ankunftszeit bearbeiten`.",
      "Die Auswahl erscheint, wenn unter `Einstellungen` bei `Schulstunden` Uhrzeiten stehen. Jede Schule pflegt ihre eigenen Zeiten.",
      "Ändern Sie später eine Schulstunde, bleiben gespeicherte Zeiten unverändert.",
      // #3371: Textfeld mit Ziffernmaske statt nativem Zeitfeld, Knopf nur
      // mit gepflegter Vorgabe (care-weekly-plan-editor.tsx).
      "Tippen Sie nur Ziffern, zum Beispiel 1600 für 16:00 Uhr.",
      "Steht unter einem leeren Feld zum Beispiel `16:00 Uhr eintragen`? Ein Klick trägt die übliche Zeit Ihrer Schule ein.",
      "Die üblichen Zeiten stehen unter `Einstellungen` bei `Betreuungszeiten`. Jede Schule pflegt ihre eigenen Zeiten.",
      "Betreut Ihre OGS am Wochenende? Dann gilt dort der Freitagsplan. Am Wochenende zeigt die Wochenansicht dieser Woche auch Samstag und Sonntag.",
      // #5102: Angebotswechsel aus dem Wochenplan.
      "Wechselt das Kind mit dem Wochenplan das Angebot, zeigt moto danach `Auswirkung auf den Betreuungsplan`. Dort sehen Sie, welche Regeltermine angepasst sind und wo das Kind weiter eingeplant ist.",
    ],
    differences: [
      // Einstellung `enrollment.bookings_authoritative`, Vorgabe aus. Der
      // Satz beginnt bei dem, was zu sehen ist -- den Namen der Einstellung
      // kennt die Leserin nicht (care-weekly-plan-modal.tsx:271).
      "Lassen sich die Betreuungstage nicht anhaken? Dann bestimmen bei Ihnen die gebuchten Betreuungsangebote, wann ein Kind da ist.",
      "An einem Tag ohne Buchung sind `Ankunft` und `Abholung` grau. `Notiz zum Tag (jede Woche)` bleibt frei.",
    ],
    troubleshootingDetails: [
      "Fehlt `Wochenplan`? Dann fehlen Ihnen die Schreibrechte für dieses Kind.",
      "Die Tage kommen aus den Buchungen und das soll anders sein? Das kann nur das moto-Team umstellen.",
    ],
    related: [
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadCarePlan,
    ],
  };
}
