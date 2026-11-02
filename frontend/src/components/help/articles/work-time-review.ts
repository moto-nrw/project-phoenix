import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Arbeitszeiten des Teams: Ansehen und Korrigieren unter `/time-tracking`
 * (mit `time_tracking:manage`), Antraege im Reiter `Mitarbeitende` des
 * Anfragen-Moduls (#2433).
 */
export function workTimeReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadWorkTimeReview,
    title: "Arbeitszeiten des Teams prüfen",
    question: "Wie prüfe ich Arbeitszeiten, Urlaub und Korrekturen?",
    summary: "Zeiten stehen bei `Mitarbeiter`, Anträge unter `Anfragen`.",
    group: "personal",
    audience: "lead",
    icon: "Clock3",
    steps: [],
    instructionGroups: [
      {
        title: "Zeiten des Teams ansehen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf.",
          "Öffnen Sie `Mitarbeiter`.",
          "Wählen Sie oben den Reiter `Zeitkonten`.",
          "Lesen Sie in der `Einrichtungs-Übersicht` die Zahlen für `Woche` oder `Monat`.",
          "Filtern Sie die Liste bei Bedarf nach `Minusstunden`, `Plusstunden` oder `über +20 Std.`.",
          "Die Spalte `Hinweise` zählt die Tage mit zu kurzer Pause oder mehr als 10 Std. Arbeit. Mit `Mit Hinweisen` sehen Sie nur diese Personen.",
        ],
      },
      {
        title: "Eine einzelne Zeit korrigieren",
        steps: [
          "Öffnen Sie `Mitarbeiter` und wählen Sie die Person.",
          "Wählen Sie den Reiter `Zeiterfassung`.",
          "Öffnen Sie den Eintrag, den Sie ändern möchten.",
          "Ändern Sie die Zeiten und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einen Antrag entscheiden",
        steps: [
          "Öffnen Sie im `Tagesbetrieb` den Bereich `Anfragen`.",
          "Wählen Sie den Reiter `Mitarbeitende`.",
          "Öffnen Sie den Antrag.",
          "Wählen Sie `Genehmigen`, `Ablehnen` oder stellen Sie eine Rückfrage.",
        ],
      },
    ],
    result:
      "Die Person sieht die Entscheidung sofort in ihrer eigenen Zeiterfassung.",
    notes: [
      "Nachträge sind der Leitung vorbehalten. Ihr Team kann Zeiten nicht selbst nachtragen.",
      "Wochensaldo und Stundenkonto rechnen gegen das hinterlegte Arbeitszeitmodell.",
      "`Team` → `Zeiterfassung` zeigt nur Ihre eigene Stempeluhr, nicht die des Teams.",
      "Mit `Monat abschließen` frieren Sie den Stand ein. Das geht ab dem 1. des Folgemonats.",
      "Ist unter `Abrechnung` der `Versand an das Lohnbüro` eingeschaltet, geht die Monatsdatei danach von selbst an das Lohnbüro.",
    ],
    differences: [
      "Fehlt der Reiter `Mitarbeitende`? Dann dürfen Sie keine Anträge entscheiden.",
    ],
    troubleshootingDetails: [
      "Der Saldo einer Person stimmt nicht? Prüfen Sie ihr `Arbeitszeitmodell` in der Personalakte.",
    ],
    related: [
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadPayroll,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}
