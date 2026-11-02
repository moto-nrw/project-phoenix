import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Hausaufgaben in der Lernzeit (#4796): die OGS sieht je Kind die
 * Hausaufgaben seiner Klasse und meldet „Fertig“, „Nicht fertig“ oder „War
 * schwer“ zurück. Nur in Terminen der Listenart Lernzeit.
 */
export function homeworkFeedbackTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.homeworkFeedback,
    title: "Rückmeldung zu Hausaufgaben geben",
    question:
      "Wie melde ich zurück, ob ein Kind die Hausaufgaben geschafft hat?",
    summary:
      "In der Lernzeit sehen Sie die Hausaufgaben jeder Klasse und geben je Kind eine Rückmeldung.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "ListChecks",
    requirements: [
      "Der Termin ist im Betreuungsplan als Lernzeit eingetragen.",
      "Die Lehrkraft hat für die Klasse des Kindes Hausaufgaben eingetragen.",
    ],
    steps: [
      "Öffnen Sie `Aktuelle Aufsicht` und Ihre Lernzeit.",
      "Unter dem Namen eines Kindes steht `Hausaufgaben:` mit dem, was zu tun ist.",
      "Tippen Sie beim Kind auf `Hausaufgaben`.",
      "Wählen Sie `Fertig`, `Nicht fertig` oder `War schwer`.",
      "Schreiben Sie bei Bedarf eine `Kurze Notiz (freiwillig)`.",
      "Tippen Sie auf `Rückmeldung speichern`.",
    ],
    result:
      "Beim Kind steht `Rückmeldung:` mit Ihrer Antwort. Die Lehrkraft sieht sie in ihrer Klassenansicht.",
    notes: [
      "Sie können die Rückmeldung am selben Tag ändern. Dann gilt die neue.",
      "Eltern sehen weder die Hausaufgaben noch die Rückmeldung.",
    ],
    differences: [
      "Kein Knopf `Hausaufgaben`? Dann ist der Termin keine Lernzeit, oder für die Klasse ist heute nichts eingetragen.",
    ],
    related: [HELP_TOPICS.activeSupervision],
  };
}
