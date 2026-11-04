import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/dienstplan`. Ansichten `Woche`, `Person` und `Halbjahr`, Schichten je Person. */
export function dutyRosterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDutyRoster,
    title: "Einen Dienstplan erstellen",
    question: "Wie plane ich die Schichten des Teams?",
    summary: "Der Dienstplan zeigt, wer wann arbeitet.",
    group: "planung",
    audience: "lead",
    icon: "Clock3",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Dienstplan`.",
      "Wählen Sie oben `Woche`, `Person` oder `Halbjahr`.",
      "Blättern Sie zur gewünschten Woche.",
      "Wählen Sie in der Zeile einer Person den passenden Tag.",
      "Tragen Sie die Schicht ein und speichern Sie sie.",
    ],
    result:
      "Die Person sieht ihre Schichten unter `Mein Kalender`. Dort tragen sie die Farbe `Schicht`.",
    notes: [
      "Jede Schicht kann eine Dienstart tragen. Die Farbe kommt von der Dienstart.",
      "Über `Dienstarten verwalten` pflegen Sie die Arten.",
      "Unter dem Plan steht `Stunden der Woche`. Dort sehen Sie je Person die Stunden je Dienstart und das Soll.",
      "Unter `Person` sehen Sie die Woche einer Person in Viertelstunden. Darunter stehen die Stunden je Tag und Dienstart.",
      "Ziehen Sie dort über die Viertelstunden, um eine Schicht anzulegen.",
      "Wählen Sie im Drei-Punkte-Menü `Drucken oder exportieren`. Wählen Sie Vorlage, Fassung und Zeitraum. `Drucken` öffnet die Druckansicht. Für eine Datei wählen Sie das Format und `Herunterladen`.",
      "Die Vorlage `Stundenübersicht` druckt diese Stunden als Liste.",
      "Eine Serie lässt Ferien, Schließtage und Feiertage aus. Arbeitet die Person auch in den Ferien, setzen Sie bei der Serie das Häkchen `Auch in den Ferien und an Schließtagen planen`.",
      "An Feiertagen plant eine Serie nie eine Schicht.",
    ],
    differences: [
      "Eine neue Schicht auf einem Schließtag löst eine Rückfrage aus.",
      "Ist ein Tag nicht vollständig durch Schichten abgedeckt, weist moto darauf hin.",
    ],
    troubleshootingDetails: [
      "Fehlt eine Person im Plan? Dann ist sie noch nicht als Personal angelegt.",
    ],
    related: [
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}
