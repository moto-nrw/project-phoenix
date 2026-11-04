import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function correctWorkTimeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.correctWorkTime,
    title: "Eigene Arbeitszeit prüfen und korrigieren",
    question: "Wie prüfe oder korrigiere ich meine Arbeitszeit?",
    summary:
      "Prüfen Sie Ihre erfassten Zeiten und berichtigen Sie einen vorhandenen Eintrag.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "Clock3",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      // Live geprueft: der Umschalter sitzt in der Kopfkarte, nicht an der
      // Tabelle. Die Tabelle heisst selbst `Zeiterfassung`.
      "Wechseln Sie oben rechts zwischen `Woche` und `Monat`.",
      "Suchen Sie in der Tabelle `Zeiterfassung` den betroffenen Tag.",
      "Wählen Sie am Ende der Zeile das Stift-Symbol.",
      "Ändern Sie Beginn, Ende oder Pause.",
      "Tragen Sie einen Grund für die Änderung ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die korrigierte Zeit und der neue Saldo werden angezeigt. Die Änderung bleibt in der Historie nachvollziehbar.",
    differences: [
      "Einen vollständig fehlenden Arbeitstag trägt nur nach, wer die Zeiten des Teams verwalten darf.",
      "Ein abgeschlossener Monat kann nachträgliche Änderungen anzeigen, sein übertragener Saldo bleibt jedoch festgeschrieben.",
    ],
    notes: [
      "Klappen Sie einen geänderten Tag auf, um die Änderungshistorie zu sehen.",
      "Öffnen Sie für Ihren Zeitnachweis oben rechts das Drei-Punkte-Menü. Wählen Sie `Exportieren`, den Zeitraum und das Format. Mit `Herunterladen` bekommen Sie die Datei.",
    ],
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.vacation],
  };
}
