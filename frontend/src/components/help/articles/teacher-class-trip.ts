import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `Ausflug oder Klassenfahrt melden` (`class-day-class.tsx`, #4799). Nur mit
 * Freigabe der OGS; ohne Rückkehrzeit gilt jeder Tag als Klassenfahrt.
 */
export function teacherClassTripTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassTrip,
    title: "Einen Ausflug oder eine Klassenfahrt melden",
    question: "Meine Klasse ist unterwegs. Wie sage ich das der OGS?",
    summary: "Sie melden den Zeitraum und welche Kinder mitfahren.",
    group: "klasse",
    audience: "teacher",
    icon: "MapPin",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse.",
      "Wählen Sie `Ausflug oder Klassenfahrt melden`.",
      "Wählen Sie bei `Von` und `Bis` die Tage.",
      "Alle Kinder der Klasse sind ausgewählt. Öffnen Sie `Kinder` und nehmen Sie den Haken bei Kindern weg, die nicht mitfahren.",
      "Ist es nur ein Tag und kommt die Klasse am selben Tag zurück? Tragen Sie bei `Rückkehrzeit (optional)` die Uhrzeit ein.",
      "Wählen Sie `Für … Kinder melden`.",
    ],
    result:
      "Ohne Rückkehrzeit sieht die OGS die Kinder an jedem dieser Tage als unterwegs. Mit Rückkehrzeit kommen die Kinder um diese Uhrzeit in die OGS.",
    notes: [
      "Bei `Ziel (optional)` können Sie zum Beispiel `Zoo` eintragen. Die OGS sieht das.",
    ],
    differences: [
      "Fehlt `Ausflug oder Klassenfahrt melden`? Dann hat Ihre OGS das Melden nicht freigegeben.",
      "Kinder mit `Keine Betreuung` stehen nicht in der Auswahl. Die OGS betreut sie nicht.",
    ],
    troubleshootingDetails: [
      "moto sagt, für ein Kind steht an einem Tag schon etwas? Unter der Meldung stehen das Kind und der Tag. Das Kind ist dann zum Beispiel krank gemeldet. Es wurde nichts gespeichert. Sprechen Sie mit der OGS.",
      "moto sagt, die OGS hat schon eine andere Ankunftszeit eingetragen? Auch dann wurde nichts gespeichert. Sprechen Sie mit der OGS.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherArrivalChange,
      HELP_TOPICS.teacherMessages,
    ],
  };
}
