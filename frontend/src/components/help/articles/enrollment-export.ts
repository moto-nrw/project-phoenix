import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Export einer Phase, Export-Menü in der Kopfzeile der Phasenseite. */
export function enrollmentExportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentExport,
    title: "Anmeldungen exportieren",
    question: "Wie bekomme ich die Anmeldungen als Datei?",
    summary: "Jede Phase lässt sich als Datei herunterladen.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "Download",
    steps: [
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Wählen Sie oben `Anmeldungen exportieren`.",
      "Wählen Sie `PDF`, `Word` oder `Excel`.",
    ],
    result: "Die Datei wird erstellt und heruntergeladen.",
    notes: [
      "Oben steht, wie viele Eingänge die Phase hat und wie viele davon offen sind.",
      "Die Seite hat drei Exporte: `Anmeldungen exportieren`, `Klassenliste exportieren` und `Auswertung exportieren`.",
      "`Klassenliste exportieren` gilt für die Klasse, die Sie unter `Klasse für Klassenliste` wählen.",
      "`Auswertung exportieren` bietet PDF und Word `ausführlich` oder als `kompakte Tabelle`.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Der Export läuft am Computer.",
    ],
    troubleshootingDetails: [
      "Brauchen Sie Listen aus dem laufenden Betrieb? Die stehen unter `Exporte` in der `Datenverwaltung`.",
    ],
    related: [
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}
