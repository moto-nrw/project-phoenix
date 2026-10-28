import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Die Checkliste „Erste Schritte“ für Betreuungskräfte (#3748). Sie öffnet
 * sich nur Personen ohne `config:update`, die neu dazukommen, sobald die
 * Schule eine Gruppe oder ein Kind hat. Die Beschriftungen stammen aus
 * `components/staff-onboarding/`.
 */
export function caregiverFirstStepsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.caregiverFirstSteps,
    title: "Erste Schritte im Arbeitsalltag",
    question: "Wie zeigt mir moto die wichtigsten Handgriffe?",
    summary:
      "Eine Checkliste zeigt Ihnen Schritt für Schritt, wo Sie was in moto finden.",
    group: "einstieg",
    audience: "caregiver",
    icon: "ClipboardList",
    requirements: ["Sie sind neu im Team Ihrer OGS."],
    steps: [
      "Melden Sie sich an. Unten rechts öffnet sich die Checkliste `Erste Schritte mit moto`.",
      "Der nächste Schritt ist aufgeklappt. Wählen Sie `Zeig es mir`.",
      "moto zeigt Ihnen den Weg in der Seitenleiste und dann die wichtigen Stellen. Klicken Sie die hervorgehobene Stelle an oder wählen Sie `Weiter`.",
      "Am Ende der Tour wählen Sie `Fertig`. Der Schritt bekommt einen Haken.",
      "Sind alle Schritte erledigt, wählen Sie `Abschließen`.",
    ],
    result:
      "Sie kennen die wichtigsten Handgriffe, und die Checkliste verschwindet. Alle Anleitungen bleiben hier in der Hilfe.",
    notes: [
      "Jeden Schritt können Sie mit `Überspringen` auslassen.",
      "Einen erledigten Schritt sehen Sie sich mit `Noch einmal zeigen` wieder an.",
      "Sie können die Checkliste einklappen. Unten rechts steht dann `Erste Schritte` mit der Zahl der offenen Schritte.",
      "Die Tour zeigt nur. Sie meldet kein Kind an und stempelt Sie nicht ein.",
    ],
    differences: [
      "Welche Schritte Sie sehen, hängt von Ihrer OGS ab. Ohne feste Gruppen fehlt zum Beispiel `Meine Gruppe ansehen`.",
      "`Meine Gruppe ansehen` erscheint erst, wenn Ihnen eine Gruppe zugeordnet ist.",
    ],
    troubleshootingDetails: [
      "Die Checkliste erscheint nicht? Sie kommt erst, wenn Ihre OGS eine Gruppe oder ein Kind in moto hat. Wer schon vor der Checkliste mit moto gearbeitet hat, bekommt sie nicht.",
      "Die Tour ist plötzlich weg? Sie endet, wenn Sie eine andere Seite öffnen. Mit `Zeig es mir` starten Sie sie neu.",
      "Sie haben `Nicht mehr anzeigen` gewählt? Die Checkliste kommt nicht wieder. Die Anleitungen stehen hier in der Hilfe.",
    ],
    related: [
      HELP_TOPICS.appOverview,
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.webAttendance,
      HELP_TOPICS.trackWorkTime,
    ],
  };
}
