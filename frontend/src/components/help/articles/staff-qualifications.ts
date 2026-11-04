import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Nachweise (#4808): Eintrag im Reiter `Stammdaten` der Personalakte, die
 * Liste `/database/qualifications`, die Arten unter
 * `/database/qualification-types` und die Erinnerung vor Ablauf. Sichtbar mit
 * `staff:stammdaten`; die eigenen Nachweise sieht jede Person im Profil.
 */
export function staffQualificationsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffQualifications,
    title: "Nachweise wie Erste Hilfe im Blick behalten",
    question: "Wie sehe ich, wessen Nachweis bald abläuft?",
    summary:
      "moto zeigt jeden Nachweis mit Ablaufdatum und erinnert vor dem Ablauf.",
    group: "personal",
    audience: "lead",
    icon: "FirstAid",
    requirements: ["Das Recht, Personaldaten zu pflegen (`Stammdaten`)."],
    steps: [],
    instructionGroups: [
      {
        title: "Einen Nachweis eintragen",
        steps: [
          "Öffnen Sie die Personalakte der Person und den Reiter `Stammdaten`.",
          "Wählen Sie oben `Bearbeiten`.",
          "Wählen Sie unter `Qualifikationen` den Knopf `Qualifikation hinzufügen`.",
          "Wählen Sie die `Art`, zum Beispiel `Erste Hilfe am Kind`. Für alles andere wählen Sie `Sonstiges` und tragen einen Namen ein.",
          "Tragen Sie `Erworben am` ein. moto schlägt `Gültig bis` aus der Gültigkeit der Art vor. Prüfen Sie das Datum.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Alle Nachweise sehen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Nachweise`.",
          "Filtern Sie oben nach `Status` oder `Art`.",
          "Wählen Sie `Für Excel herunterladen`, um die Liste als Datei zu speichern.",
        ],
      },
      {
        title: "Nachweisarten einstellen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Nachweisarten`.",
          "Wählen Sie eine Art oder legen Sie eine eigene an.",
          "Stellen Sie `Gültig für (Monate)` und `Erinnerung vor Ablauf (Wochen)` ein.",
          "Setzen Sie bei Bedarf `Pflicht für alle Mitarbeitenden`.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Läuft ein Nachweis bald ab, bekommen alle mit dem Recht `Stammdaten` und die Person selbst eine E-Mail. Am Ablauftag kommt eine zweite. In moto kommt dazu eine Nachricht, wenn sie im Profil eingeschaltet ist.",
    differences: [
      "Ein alter Eintrag ohne Art steht als `Sonstiges` da. moto ordnet ihn nicht selbst zu. Wählen Sie die Art im Reiter `Stammdaten` aus.",
      "Status `Fehlt` heißt: Die Art ist Pflicht, und die Person hat noch keinen Nachweis davon.",
    ],
    notes: [
      "moto speichert nur Art und Datum, keine Inhalte. Beim Führungszeugnis halten Sie so fest, dass Sie es eingesehen haben.",
      "Ihre eigenen Nachweise sehen alle Mitarbeitenden im `Profil` unter `Meine Nachweise`.",
    ],
    related: [HELP_TOPICS.leadStaffRecord],
  };
}
