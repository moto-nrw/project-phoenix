import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/statistics`: Quoten je Bereich ueber einen frei gewaehlten Zeitraum. */
export function statisticsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStatistics,
    title: "Eine Statistik öffnen",
    question: "Wo sehe ich Quoten und Auslastung?",
    summary: "Die Statistik zeigt die Anwesenheit über einen Zeitraum.",
    group: "auswertung",
    audience: "lead",
    icon: "BarChart3",
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Öffnen Sie `Statistik`.",
      "Wählen Sie oben den Bereich: `Gruppen`, `Kinder`, `Räume` oder `Kurse`.",
      "Wählen Sie den Zeitraum.",
      "Lesen Sie die Zahlen der Tabelle.",
    ],
    result: "Die Zahlen gelten für den gewählten Bereich und Zeitraum.",
    notes: [
      "Bei `Kurse` wählen Sie zusätzlich zwischen `Je Kurs` und `Je Kind`.",
      "Über das Menü mit den drei Punkten geht `Exportieren` als `PDF`, `Excel` oder `Word`.",
      "Die Pfeile schieben den Zeitraum um seine eigene Länge weiter.",
    ],
    differences: [
      "`Räume` zeigt nur mit eingeschaltetem `Anwesenheitsprotokoll` Zahlen.",
      "Ohne `Anwesenheitsprotokoll` fehlt dort auch das Menü zum Exportieren.",
      "Ist die Raumhistorie ausgeschaltet? Ihre Administration kann das `Anwesenheitsprotokoll` einschalten. Die Einstellung steht unter `Einstellungen` und `Datenschutz`.",
      "`Kurse` zeigt nur dann Zahlen, wenn Ihre OGS damit arbeitet.",
    ],
    troubleshootingDetails: [
      "Fehlt `Statistik` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadAbsenceReport,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.dayLog,
    ],
  };
}
