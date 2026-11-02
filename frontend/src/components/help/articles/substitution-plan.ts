import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/vertretung`. Heisst in Seitenleiste und Kopfzeile `Vertretungsplan` (#4778). */
export function substitutionPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadSubstitutionPlan,
    title: "Eine Vertretung planen",
    question: "Wie plane ich eine Vertretung?",
    summary: "Sie tragen ein, wer einen Einsatz übernimmt.",
    group: "planung",
    audience: "lead",
    icon: "Repeat",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Vertretungsplan`.",
      "Wählen Sie oben `Tag` oder `Woche`.",
      "Wählen Sie den Einsatz, der vertreten werden soll.",
      "Wählen Sie die Person, die übernimmt.",
      "Speichern Sie die Vertretung.",
    ],
    result: "Die vertretende Person sieht den Einsatz unter `Mein Kalender`.",
    notes: [
      "Mit `Sammel-Vertretung` tragen Sie mehrere Einsätze auf einmal ein.",
      "Auch ein `Dienst` ohne Kinder lässt sich vertreten. Sind weniger Personen da als unter `Benötigtes Personal` eingetragen, zeigt der Plan eine Lücke.",
    ],
    differences: [
      "`Übergaben` im `Tagesbetrieb` ist etwas anderes: dort übergeben Sie Gruppen für heute. Unter `Vertretungen heute` sehen Sie dort auch die Vertretungen des Tages.",
    ],
    troubleshootingDetails: [
      "Fehlt der Einsatz? Dann ist er noch nicht im Betreuungsplan angelegt.",
      "Eine externe Person steht nicht zur Wahl? Externe Personen übernehmen keine Vertretung.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadWorkTimeReview,
    ],
  };
}
