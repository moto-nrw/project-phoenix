import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/database/work-time-models` (#6891): die Arbeitszeitmodell-Vorlagen der
 * Schule. Vorher ließ sich eine Vorlage nur aus der Personalakte heraus
 * anlegen. Zugeordnet wird weiter in der Personalakte.
 */
export function workTimeTemplatesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadWorkTimeTemplates,
    title: "Vorlagen für Arbeitszeitmodelle verwalten",
    question: "Wie lege ich ein Arbeitszeitmodell für viele Personen an?",
    summary:
      "Eine Vorlage legt die Soll-Stunden fest, die mehrere Mitarbeitende teilen.",
    group: "personal",
    audience: "lead",
    icon: "Hourglass",
    requirements: [
      "Sie dürfen die Zeiterfassung aller Mitarbeitenden verwalten.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Eine Vorlage anlegen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Arbeitszeitmodelle`.",
          "Wählen Sie oben rechts `Vorlage`.",
          "Geben Sie einen Namen ein, zum Beispiel `Teilzeit 30 Stunden`.",
          "Wählen Sie bei `Rotation` zum Beispiel `Standard (1 Woche)` oder `A/B-Wochen (2 Wochen)`.",
          "Tragen Sie je Wochentag die Stunden ein, zum Beispiel `6`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Vorlage zuordnen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf.",
          "Öffnen Sie `Mitarbeiter` und wählen Sie die Person.",
          "Wählen Sie oben `Arbeitszeitmodell` und rechts `Bearbeiten`.",
          "Wählen Sie `Vorlage zuweisen` und die Vorlage.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Vorlage ändern",
        steps: [
          "Wählen Sie die Vorlage in der Liste.",
          "Ändern Sie die Stunden.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "In der Liste steht bei jeder Vorlage, wie viele Personen sie nutzen.",
    notes: [
      "Eine Änderung gilt ab heute für alle Personen mit dieser Vorlage. Die Seite zeigt vor dem Speichern, wie viele es sind.",
      "Über die drei Punkte wählen Sie `Nicht mehr anbieten`. Wer die Vorlage schon nutzt, behält sie. Neu zuordnen lässt sie sich dann nicht mehr.",
      "Löschen geht nur, wenn niemand die Vorlage nutzt.",
    ],
    differences: [
      "Sie sehen `Arbeitszeitmodelle` nicht? Dann hat Ihre Schule die Zeiterfassung nicht gebucht, oder Ihnen fehlt das Recht dafür.",
    ],
    related: [
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadTargetOverride,
      HELP_TOPICS.leadWorkTimeReview,
    ],
  };
}
