import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `Fehlen ohne Meldung` in der Tagesübersicht der Schulverwaltung
 * (`class-day-overview.tsx`, #4791): Meldung nachtragen und zurücknehmen.
 */
export function teacherAbsenceReportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherAbsenceReport,
    title: "Eine Meldung für ein fehlendes Kind nachtragen",
    question: "Ein Kind fehlt ohne Meldung. Die Eltern rufen an. Was tue ich?",
    summary:
      "Die Schulverwaltung trägt krank oder entschuldigt für heute nach.",
    group: "klasse",
    audience: "teacher",
    icon: "ShieldAlert",
    requirements: ["Die Rolle `Schulverwaltung`."],
    steps: [
      "Öffnen Sie die `Klassenansicht`.",
      "Wählen Sie unter `Ganze Schule` die Zahl `Fehlen ohne Meldung`.",
      "Suchen Sie das Kind in der Liste.",
      "Wählen Sie neben dem Namen `Meldung eintragen`.",
      "Wählen Sie `Krank` oder `Entschuldigt`.",
    ],
    result:
      "Das Kind steht nicht mehr unter `Fehlen ohne Meldung`. Die Lehrkraft und die OGS sehen den Status für heute.",
    notes: [
      "Die Meldung gilt nur für heute.",
      "Jede Meldung steht mit Ihrem Namen im Änderungsverlauf des Kindes.",
      "Falsch eingetragen? Öffnen Sie die Liste `Krank` oder `Entschuldigt`. Wählen Sie neben dem Kind `Zurücknehmen`. Das Kind steht dann wieder unter `Fehlen ohne Meldung`.",
    ],
    differences: [
      "Kein `Meldung eintragen` neben dem Namen? Dann haben Sie nicht die Rolle `Schulverwaltung`, oder der Tag ist nicht heute.",
      "Kinder mit `Keine Betreuung` stehen noch nicht in dieser Liste.",
    ],
    troubleshootingDetails: [
      "moto sagt, für das Kind steht heute schon etwas? Dann hat die OGS zum Beispiel eine frühere Abholung eingetragen. Es wurde nichts gespeichert. Sprechen Sie mit der OGS.",
      "`Zurücknehmen` fehlt? Sie können nur Meldungen zurücknehmen, die die Schulverwaltung eingetragen hat. Eine Krankmeldung der Eltern oder der OGS ändert die OGS.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherMorningAttendance,
      HELP_TOPICS.teacherSettings,
    ],
  };
}
