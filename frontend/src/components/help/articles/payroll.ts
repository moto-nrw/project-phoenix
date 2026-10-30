import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/payroll`: keine Abrechnung, sondern deren Konfiguration. Drei Karten:
 * `Vollständigkeit`, `Lohnarten`, `DATEV-Mandant` (payroll/page.tsx:109).
 */
export function payrollTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadPayroll,
    title: "Eine Abrechnung für DATEV vorbereiten",
    question: "Wie bereite ich die Lohnabrechnung vor?",
    summary: "Sie hinterlegen einmal die Lohnarten Ihres Trägers.",
    group: "personal",
    audience: "lead",
    icon: "Landmark",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Abrechnung`.",
      "Prüfen Sie oben unter `Vollständigkeit`, was noch fehlt.",
      "Tragen Sie unter `Lohnarten` die Nummern Ihres Lohnsystems ein.",
      "Tragen Sie unter `DATEV-Mandant` `Beraternummer` und `Mandantennummer` ein.",
      "Tragen Sie die `Personalnummern` bei den Personen nach. Das Feld steht beim Personal im Reiter `Stammdaten`.",
    ],
    result: "Ist alles vollständig, kann die Datei für DATEV erzeugt werden.",
    notes: [
      "Änderungen werden sofort gespeichert. Es gibt keinen Speichern-Knopf.",
      "Die Lohnartnummern kommen vom Träger, nicht von moto.",
      "`Lohn und Gehalt` braucht die Mandanten-Kennzahlen nicht.",
    ],
    differences: [
      "Eine Kategorie ohne Lohnartnummer wird nicht exportiert.",
      "Dieselbe Nummer bei mehreren Kategorien ist erlaubt, führt aber meist zu doppelten Stunden. moto warnt davor.",
    ],
    troubleshootingDetails: [
      "Der Export erzeugt keine Datei? Dann fehlt oben unter `Vollständigkeit` noch etwas.",
    ],
    related: [
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadExports,
    ],
  };
}
