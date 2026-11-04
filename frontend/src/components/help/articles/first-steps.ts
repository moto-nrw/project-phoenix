import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Assistent „Erste Schritte“ für neue Schulen (#2832). Er öffnet sich nur
 * Personen mit `config:update`, solange die Schule nicht fertig ist. Die
 * Beschriftungen stammen aus `components/school-setup/`.
 */
export function firstStepsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadFirstSteps,
    title: "Erste Schritte mit moto",
    question: "Wie führt mich moto durch den Start?",
    summary:
      "Eine neue OGS geht die ersten Schritte mit einer Checkliste durch.",
    group: "einrichten",
    audience: "lead",
    icon: "ClipboardList",
    requirements: [
      "Ihre OGS ist neu bei moto.",
      "Sie dürfen die Einstellungen Ihrer OGS ändern.",
    ],
    steps: [
      "Melden Sie sich an. Unten rechts öffnet sich die Checkliste `Erste Schritte mit moto`.",
      "Der nächste Schritt ist aufgeklappt. Wählen Sie `Zeig es mir`.",
      "moto zeigt Ihnen in der Seitenleiste den Weg zur Seite und dann jede Stelle. Klicken Sie die hervorgehobene Stelle an.",
      "Ist der Schritt erledigt, bekommt er einen Haken. Der nächste Schritt klappt auf.",
      "Sind alle Schritte erledigt, wählen Sie `Abschließen`.",
    ],
    result:
      "Die ersten Einträge stehen, und die Checkliste verschwindet. Die übrigen Daten legen Sie mit den Anleitungen hier in der Hilfe an.",
    notes: [
      "Jeden Schritt können Sie mit `Überspringen` auslassen.",
      "Ein Schritt ist erledigt, sobald es den ersten Eintrag gibt, zum Beispiel das erste Kind. Die übrigen legen Sie danach genauso an.",
      "Sie können die Checkliste einklappen. Unten rechts steht dann `Erste Schritte` mit der Zahl der offenen Schritte.",
      "Sie können jeden Schritt in der Checkliste aufklappen, auch in anderer Reihenfolge.",
      "Ob moto auch den Raum erfasst, stimmt das moto-Team mit Ihnen ab.",
      "Sie möchten die Anwesenheit mit NFC-Armbändern erfassen? Dann melden Sie sich beim moto-Team.",
      "Sie haben eine Liste aller Kinder? Wählen Sie in der Tour zu `Erstes Kind anlegen` den Knopf `Importieren`. Dann zeigt die Tour den Import.",
      "Ihre OGS nutzt die Eltern-App nicht? Dann überspringen Sie `Erste Eltern einladen`.",
      "Der Stand gilt für die ganze OGS. Andere Leitungen sehen denselben Stand.",
    ],
    troubleshootingDetails: [
      "Die Tour ist plötzlich weg? Sie endet, wenn Sie eine andere Seite öffnen. Mit `Zeig es mir` starten Sie sie neu.",
      "Sie haben `Nicht mehr anzeigen` gewählt? Die Checkliste kommt nicht wieder. Die Anleitungen zu jedem Schritt stehen hier in der Hilfe, zum Beispiel unter `Weitere Themen`.",
    ],
    related: [
      HELP_TOPICS.leadGoLive,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}
