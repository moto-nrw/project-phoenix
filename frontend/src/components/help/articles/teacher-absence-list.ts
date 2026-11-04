import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/school/fehlzeiten` (#4793): Fehltage je Kind einer Klasse in einem
 * Zeitraum, mit Export als PDF oder Excel.
 */
export function teacherAbsenceListTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherAbsenceList,
    title: "Fehlzeiten einer Klasse ansehen",
    question: "Wie viele Tage hat ein Kind gefehlt?",
    summary:
      "Unter `Fehlzeiten` stehen die Fehltage je Kind, für Klassenbuch und Zeugnis.",
    group: "klasse",
    audience: "teacher",
    icon: "ClipboardList",
    steps: [
      "Öffnen Sie `Fehlzeiten`.",
      "Wählen Sie die Klasse.",
      "Prüfen Sie `Von` und `Bis`. Vorgewählt ist das laufende Halbjahr.",
      "Lesen Sie je Kind die Fehltage, davon entschuldigt und unentschuldigt.",
      "Wählen Sie bei Bedarf `Exportieren`. Wählen Sie `PDF` oder `Excel`.",
    ],
    result: "Sie sehen die Fehltage jedes Kindes im gewählten Zeitraum.",
    notes: [
      "Gezählt werden die Anwesenheit am Vormittag und die Meldungen.",
      "Krank oder entschuldigt gemeldet zählt als entschuldigt. Auch eine Meldung der Schulverwaltung zählt.",
      "Ein Kind war am Vormittag da, ist aber krank gemeldet? Dann zählt der Tag nicht als Fehltag.",
      "Klassenfahrt ist kein Fehltag.",
      "`Nicht erfasst` heißt: An diesem Schultag gibt es keinen Eintrag. Das ist kein Fehltag.",
      "Es gibt nur ganze Tage. Ein halber Tag zählt nicht extra.",
      "Kinder, deren Betreuung im Zeitraum endet, bleiben in der Liste. Gezählt werden nur ihre Betreuungstage.",
      "Jeder Abruf und jeder Export wird protokolliert.",
    ],
    differences: [
      "Die Schulverwaltung sieht alle Klassen. Eine Lehrkraft sieht nur ihre Klassen.",
      "Kinder ohne eigenen Eintrag in moto stehen noch nicht in der Liste.",
    ],
    troubleshootingDetails: [
      "Viele Tage `Nicht erfasst`? Dann fehlt die Anwesenheit am Vormittag. Tragen Sie sie jeden Morgen ein.",
      "`Unvollständiger Zeitraum`? Ältere Anwesenheit oder Meldungen können bereits gelöscht sein. Die Liste und der Export zählen erst ab dem angegebenen Datum.",
    ],
    related: [
      HELP_TOPICS.teacherMorningAttendance,
      HELP_TOPICS.teacherAbsenceReport,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}
