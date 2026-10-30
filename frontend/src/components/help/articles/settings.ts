import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/settings`. Die Reiter kommen aus dem Schema des Servers und sind je Schule
 * verschieden (settings/page.tsx:32) -- deshalb nennt der Artikel keine feste
 * Liste, sondern den Weg und die Regeln. Die wichtigsten Einstellungen stehen
 * in `settingsGuideTopic`.
 */
export function settingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.settings,
    title: "Den Betrieb der OGS einstellen",
    question: "Wo stelle ich ein, wie unsere OGS arbeitet?",
    summary: "Die Einstellungen steuern Anwesenheit, Funktionen und Abläufe.",
    group: "konfiguration",
    audience: "lead",
    icon: "Settings",
    steps: [
      "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
      "Wählen Sie oben den Bereich.",
      "Ändern Sie die gewünschte Einstellung.",
    ],
    result: "Die Änderung gilt sofort für alle in Ihrer OGS.",
    notes: [
      "Oben steht, wie viele Einstellungen von der Vorgabe abweichen.",
      "Jede Einstellung hat einen Satz darunter, der sagt, was sie bewirkt.",
      "Unter `Datenschutz` können Sie die `Nutzungsanalyse` ausschalten. Dann erhält moto keine Nutzungsstatistiken Ihrer Schule mehr.",
      "Damit enden auch Sitzungsaufzeichnungen. Die Fehlerdiagnose bleibt davon getrennt.",
      "Unter `Datenschutz` sehen Sie die Löschfristen. Sie zeigen, wie lange moto Daten aufbewahrt.",
      "moto legt die Löschfristen mit Ihrer Schule fest. Sie können sie nur lesen.",
      "Unter `Betrieb` steht das `Bundesland` Ihrer Schule. Danach richten sich die gesetzlichen Feiertage. Ändern kann es nur das moto-Team.",
      "Wer Blöcke starten oder beenden darf, legen Sie unter `Betrieb` fest: bei `Wer darf Blöcke starten?` und `Wer darf Blöcke beenden?`.",
      "`Das ganze Team` geht dort nur, wenn das Team alle Gruppen und Blöcke sieht.",
      "Manche Werte legt Ihr Träger für alle Schulen fest. Dort steht `Vom Träger festgelegt`.",
      "Sie können davon abweichen. `Zurücksetzen` übernimmt wieder den Wert des Trägers.",
    ],
    differences: [
      "Welche Bereiche Sie sehen, hängt von Ihren Rechten und den Funktionen Ihrer OGS ab.",
    ],
    troubleshootingDetails: [
      "Bei einer Einstellung steht `Legt der Träger fest`? Diesen Wert ändert nur Ihr Träger.",
      "Eine Einstellung ist gesperrt? moto legt sie mit Ihrer Schule fest. Sie können sie nur lesen.",
      "Fehlt `Alte Daten automatisch löschen`? Das ist die Notbremse. Das moto-Team nutzt sie nur bei einem Vorfall.",
      "Der Wechsel des Anwesenheitsmodus wird abgelehnt? Das geht nicht, solange Kinder eingecheckt sind.",
    ],
    related: [
      HELP_TOPICS.settingsGuide,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.leadDevices,
      HELP_TOPICS.leadMissingMenu,
    ],
  };
}
