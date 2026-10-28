import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Anwesenheit am Vormittag (#4790): die Lehrkraft markiert heute je Kind „Da“
 * oder „Fehlt“ und speichert. Gespeichert getrennt von der OGS-Anwesenheit.
 */
export function teacherMorningAttendanceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherMorningAttendance,
    title: "Anwesenheit am Vormittag eintragen",
    question: "Wie trage ich ein, wer heute da ist?",
    summary: "Sie markieren bei jedem Kind „Da“ oder „Fehlt“ und speichern.",
    group: "klasse",
    audience: "teacher",
    icon: "CalendarCheck",
    steps: [
      "Öffnen Sie in der `Klassenansicht` Ihre Klasse für heute.",
      "Tippen Sie bei jedem Kind auf `Da` oder `Fehlt`.",
      "Sind fast alle da? Tippen Sie auf `Alle übrigen da`.",
      "Tippen Sie unten auf `Anwesenheit speichern`.",
    ],
    result: "Oben steht, wie viele Kinder da sind, fehlen und noch offen sind.",
    notes: [
      "Kinder mit Meldung (`Krank`, `Entschuldigt`, `Klassenfahrt`) stehen schon auf `Fehlt`.",
      "Fehlt ein Kind ohne Meldung, steht bei ihm `Fehlt ohne Meldung`.",
      "Sie können heute jederzeit ändern und neu speichern.",
      "Die Anwesenheit in der OGS ändert sich dadurch nicht.",
    ],
    differences: [
      "Sie sehen keine Knöpfe `Da` und `Fehlt`? Eintragen geht nur heute und an Schultagen. Oder die OGS hat es ausgeschaltet.",
    ],
    troubleshootingDetails: [
      "Speichern klappt nicht und ein Kind gehört nicht mehr zur Klasse? Laden Sie die Seite neu.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
