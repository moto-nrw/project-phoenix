import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/payroll`: keine Abrechnung, sondern deren Konfiguration. Vier Karten:
 * `Vollständigkeit`, `Lohnarten`, `DATEV-Mandant` und `Versand an das
 * Lohnbüro` (#4805, payroll/page.tsx).
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
    steps: [],
    instructionGroups: [
      {
        title: "Lohnarten und DATEV eintragen",
        steps: [
          "Klappen Sie in der Seitenleiste `Planung` auf.",
          "Öffnen Sie `Abrechnung`.",
          "Prüfen Sie oben unter `Vollständigkeit`, was noch fehlt.",
          "Tragen Sie unter `Lohnarten` die Nummern Ihres Lohnsystems ein.",
          "Tragen Sie unter `DATEV-Mandant` `Beraternummer` und `Mandantennummer` ein.",
          "Tragen Sie die `Personalnummern` bei den Personen nach. Das Feld steht beim Personal im Reiter `Stammdaten`.",
        ],
      },
      {
        title: "Die Monatsdatei automatisch an das Lohnbüro senden",
        description:
          "Nach dem Monatsabschluss schickt moto die Monatsdatei aller Mitarbeitenden an das Lohnbüro.",
        steps: [
          "Öffnen Sie `Abrechnung` und gehen Sie zur Karte `Versand an das Lohnbüro`.",
          "Schalten Sie `Automatisch nach dem Monatsabschluss senden` ein.",
          "Wählen Sie den `Versandweg`: `SFTP` oder `E-Mail`.",
          "Wählen Sie das `Dateiformat`.",
          "Bei `E-Mail`: Tragen Sie die `E-Mail-Adresse des Lohnbüros` ein.",
          "Unter `Letzter Versand` sehen Sie, ob die Datei angekommen ist.",
        ],
      },
    ],
    result:
      "Ist alles vollständig, kann die Datei für DATEV erzeugt werden. Nach jedem Monatsabschluss geht sie von selbst an das Lohnbüro, wenn der Versand eingeschaltet ist.",
    notes: [
      "Änderungen werden sofort gespeichert. Es gibt keinen Speichern-Knopf.",
      "Die Lohnartnummern kommen vom Träger, nicht von moto.",
      "`Lohn und Gehalt` braucht die Mandanten-Kennzahlen nicht.",
      "Per `E-Mail` bekommt das Lohnbüro einen Link, keinen Anhang. Der Link gilt 14 Tage und funktioniert nur einmal.",
      "Öffnen Sie einen Monat wieder und schließen ihn erneut ab, geht die Datei noch einmal raus. Ihr Name enthält dann „korrektur“.",
    ],
    differences: [
      "Eine Kategorie ohne Lohnartnummer wird nicht exportiert.",
      "Dieselbe Nummer bei mehreren Kategorien ist erlaubt, führt aber meist zu doppelten Stunden. moto warnt davor.",
    ],
    troubleshootingDetails: [
      "Der Export erzeugt keine Datei? Dann fehlt oben unter `Vollständigkeit` noch etwas.",
      "Unter `Letzter Versand` steht `Nicht gesendet` oder `Fehlgeschlagen`? Darunter steht der Grund. Wer die Zeiterfassung des Teams verwaltet, bekommt dazu eine E-Mail.",
      "Der `Versandweg` `SFTP` braucht die Gegenstelle aus den Einstellungen unter `Kalender und Export` bei `Zeitkonten übertragen`.",
    ],
    related: [
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadExports,
    ],
  };
}
