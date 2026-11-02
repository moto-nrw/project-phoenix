import type { HelpTopic, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function dayLogTopic(groupMode: HelpGroupMode): HelpTopic {
  // Ohne feste Gruppen zeigt die Tagesauswertung eine gemeinsame Liste.
  const openCare = groupMode === "open_care";
  return {
    id: HELP_TOPICS.dayLog,
    title: "Den heutigen Betreuungstag prüfen",
    question: "Wie prüfe ich den heutigen Betreuungstag?",
    summary: "Prüfen Sie für heute, welche Kinder anwesend oder abwesend sind.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "CalendarCheck",
    requirements: ["Ihre OGS hat das `Anwesenheitsprotokoll` eingeschaltet."],
    steps: [],
    instructionGroups: [
      {
        title: "Den Betreuungstag prüfen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf und öffnen Sie `Tagesauswertung`.",
          openCare
            ? "Prüfen Sie oben die Zahlen für alle Kinder."
            : "Prüfen Sie oben die Zahlen für alle Gruppen.",
          ...(openCare ? [] : ["Wählen Sie bei einer Gruppe `Details`."]),
          "Prüfen Sie die Kinder in den einzelnen Bereichen.",
          "Wählen Sie ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      {
        title: "Die Auswertung ausgeben",
        steps: [
          ...(openCare
            ? ["Bleiben Sie für alle Kinder in der Übersicht."]
            : [
                "Bleiben Sie für alle Gruppen in der Übersicht.",
                "Für eine einzelne Gruppe öffnen Sie deren `Details`.",
              ]),
          "Wählen Sie `Drucken`, `PDF`, `Word` oder `Excel`.",
        ],
      },
    ],
    result:
      "Sie sehen für heute, welches Kind da ist. Die Liste ist danach geordnet. Kinder ohne Meldung stehen unter `Unentschuldigt abwesend`. Vergangene Tage können Sie hier nicht auswählen.",
    differences: [
      "Fehlt `Tagesauswertung`? Fragen Sie Ihre Leitung nach dem `Anwesenheitsprotokoll`. Sie finden es unter `Einstellungen` und `Datenschutz`.",
      "Steht dort `Anwesenheitsprotokoll ist ausgeschaltet`? Dann ist es für Ihre Schule nicht eingeschaltet.",
    ],
    notes: [
      "Ist ein Kind früher gegangen, steht der Grund hinter `gegangen`. Er steht auch im `PDF`, in `Word` und in `Excel`.",
    ],
    related: [
      HELP_TOPICS.absences,
      HELP_TOPICS.emergency,
      HELP_TOPICS.ownGroups,
      HELP_TOPICS.leadAbsenceReport,
      HELP_TOPICS.leadStatistics,
    ],
  };
}
