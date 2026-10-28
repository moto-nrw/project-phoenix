import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Aktivitaetenkatalog unter `/database/activities`. Nur mit NFC: die Seite
 * gehört im Modul-Register (lib/module-register.ts) zu NFC-Anwesenheit.
 * Deshalb auch in NFC_ONLY_TOPIC_IDS.
 */
export function activitiesCatalogTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadActivities,
    title: "Aktivitäten anlegen",
    question: "Wie lege ich Aktivitäten an?",
    summary:
      "Aktivitäten sind Angebote neben der Gruppe, zum Beispiel eine AG.",
    group: "einrichten",
    audience: "lead",
    icon: "Sparkles",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aktivität anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Aktivitäten`.",
          "Wählen Sie oben rechts `+ Aktivität`.",
          "Tragen Sie bei `Name` die Bezeichnung des Angebots ein.",
          "Wählen Sie eine `Kategorie`.",
          "Tragen Sie bei Bedarf `Maximale Teilnehmer` ein.",
          "Wählen Sie bei Bedarf einen `Hauptbetreuer`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Eine Aktivität ändern",
        steps: [
          "Öffnen Sie `Aktivitäten` und wählen Sie die Aktivität aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Aktivität löschen",
        steps: [
          "Öffnen Sie `Aktivitäten` und wählen Sie die Aktivität aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Die Aktivität steht sofort zur Auswahl. Ihr Team kann sie am Tablet starten.",
    notes: [
      "`Maximale Teilnehmer` steht zunächst auf 20. Leeren Sie das Feld für keine Begrenzung.",
      "Über `Kategorie` filtern Sie die Liste auf eine Art von Angebot.",
    ],
    differences: [
      "Auf dem Handy steht statt `+ Aktivität` ein rundes Plus unten rechts.",
      "`Schulhof Freispiel` und `WC` legt moto selbst an. Ihre Namen sind fest, und `Löschen` fehlt.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Fehlt die passende `Kategorie`? Legen Sie sie in der `Datenverwaltung` unter `Terminkategorien` an.",
      "Meldet moto oder das Tablet, die Aktivität ist voll? Dann ist `Maximale Teilnehmer` erreicht. Steht bei ihr `Überbucht`, sind sogar mehr Kinder da als erlaubt. Es kommen erst wieder Kinder dazu, wenn es weniger sind. Oder Sie erhöhen die Zahl. Die Grenze des Raums hilft hier nicht.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.dataManagement,
    ],
  };
}
